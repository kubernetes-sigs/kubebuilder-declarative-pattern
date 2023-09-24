/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package memorystorage

import (
	"context"
	"fmt"
	"strconv"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
)

type MemoryStorage struct {
	schemaMutex      sync.Mutex
	schema           mockSchema
	resourceStorages map[schema.GroupResource]*resourceStorage

	clock        storage.Clock
	uidGenerator storage.UIDGenerator

	hooksMutex sync.Mutex
	hooks      []storage.Hook

	resourceVersionClock resourceVersionClock
}

func NewMemoryStorage(clock storage.Clock, uidGenerator storage.UIDGenerator) (*MemoryStorage, error) {
	s := &MemoryStorage{
		resourceStorages: make(map[schema.GroupResource]*resourceStorage),
		clock:            clock,
		uidGenerator:     uidGenerator,
	}

	if err := s.schema.Init(); err != nil {
		return nil, err
	}

	for _, builtinType := range s.schema.builtin.Meta.Resources {
		klog.V(4).Infof("registering builtin type %v", builtinType.Key)
		gvk := schema.GroupVersionKind{Group: builtinType.Group, Version: builtinType.Version, Kind: builtinType.Kind}
		gvr := gvk.GroupVersion().WithResource(builtinType.Resource)
		gr := gvr.GroupResource()

		rs := &resourceStorage{
			GroupResource:        gr,
			resourceVersionClock: &s.resourceVersionClock,
			objects:              make(map[types.NamespacedName]*unstructured.Unstructured),
			parent:               s,
		}

		// TODO: share storage across different versions
		s.resourceStorages[gr] = rs

		r := &memoryResourceInfo{
			api: metav1.APIResource{
				Name:    builtinType.Resource,
				Group:   gvk.Group,
				Version: gvk.Version,
				Kind:    gvk.Kind,
			},
			gvk: gvk,
			gvr: gvr,

			parent:  s,
			storage: rs,
		}
		r.listGVK = gvk.GroupVersion().WithKind(gvk.Kind + "List")

		parserType := s.schema.builtin.Parser.Type(builtinType.Key)
		r.parseableType = &parserType
		if r.parseableType == nil || !r.parseableType.IsValid() {
			klog.Warningf("type info not known for %v", gvk)
		}

		if meta.RESTScopeName(builtinType.Scope) == meta.RESTScopeNameNamespace {
			r.api.Namespaced = true
		}

		s.schema.resources = append(s.schema.resources, r)
	}

	return s, nil
}

// AddObject pre-creates an object
func (s *MemoryStorage) AddObject(obj *unstructured.Unstructured) error {
	ctx := context.Background()

	gv, err := schema.ParseGroupVersion(obj.GetAPIVersion())
	if err != nil {
		return fmt.Errorf("cannot parse apiVersion %q: %w", obj.GetAPIVersion(), err)
	}
	kind := obj.GetKind()

	id := types.NamespacedName{
		Namespace: obj.GetNamespace(),
		Name:      obj.GetName(),
	}

	gvk := gv.WithKind(kind)

	resource := s.findResourceByGVK(gvk)
	if resource == nil {
		return fmt.Errorf("object group/version/kind %v not known", gvk)
	}

	return resource.CreateObject(ctx, id, obj)
}

func (s *MemoryStorage) AddStorageHook(hook storage.Hook) {
	s.hooksMutex.Lock()
	defer s.hooksMutex.Unlock()

	s.hooks = append(s.hooks, hook)
}

func (resource *memoryResourceInfo) GetObject(ctx context.Context, id types.NamespacedName) (*unstructured.Unstructured, bool, error) {
	resource.storage.mutex.Lock()
	defer resource.storage.mutex.Unlock()

	object := resource.storage.objects[id]
	if object == nil {
		return nil, false, nil
	}

	return object, true, nil
}

func (resource *memoryResourceInfo) ListObjects(ctx context.Context, filter storage.ListFilter) (*unstructured.UnstructuredList, error) {
	resource.storage.mutex.Lock()
	defer resource.storage.mutex.Unlock()

	ret := &unstructured.UnstructuredList{}

	for _, obj := range resource.storage.objects {
		if filter.Namespace != "" {
			if obj.GetNamespace() != filter.Namespace {
				continue
			}
		}
		ret.Items = append(ret.Items, *obj)
	}

	rv := strconv.FormatInt(resource.storage.resourceVersionClock.Now(), 10)
	ret.SetResourceVersion(rv)

	return ret, nil
}

func (resource *memoryResourceInfo) CreateObject(ctx context.Context, id types.NamespacedName, u *unstructured.Unstructured) error {
	resource.storage.mutex.Lock()
	defer resource.storage.mutex.Unlock()

	_, found := resource.storage.objects[id]
	if found {
		return apierrors.NewAlreadyExists(resource.gvr.GroupResource(), id.Name)
	}

	u.SetCreationTimestamp(resource.parent.clock.Now())

	uid := resource.parent.uidGenerator.NewUID()
	u.SetUID(uid)

	rv := strconv.FormatInt(resource.storage.resourceVersionClock.GetNext(), 10)
	u.SetResourceVersion(rv)

	resource.storage.objects[id] = u

	resource.storage.broadcastEventHoldingLock(ctx, resource.gvk, "ADDED", u)

	return nil
}

func (resource *memoryResourceInfo) UpdateObject(ctx context.Context, id types.NamespacedName, u *unstructured.Unstructured) error {
	resource.storage.mutex.Lock()
	defer resource.storage.mutex.Unlock()

	_, found := resource.storage.objects[id]
	if !found {
		return apierrors.NewAlreadyExists(resource.gvr.GroupResource(), id.Name)
	}

	rv := strconv.FormatInt(resource.storage.resourceVersionClock.GetNext(), 10)
	u.SetResourceVersion(rv)

	resource.storage.objects[id] = u

	resource.storage.broadcastEventHoldingLock(ctx, resource.gvk, "MODIFIED", u)

	return nil
}

func (resource *memoryResourceInfo) DeleteObject(ctx context.Context, id types.NamespacedName) (*unstructured.Unstructured, error) {
	resource.storage.mutex.Lock()
	defer resource.storage.mutex.Unlock()

	deletedObj, found := resource.storage.objects[id]
	if !found {
		// TODO: return apierrors something?
		return nil, apierrors.NewNotFound(resource.gvr.GroupResource(), id.Name)
	}
	delete(resource.storage.objects, id)

	resource.storage.broadcastEventHoldingLock(ctx, resource.gvk, "DELETED", deletedObj)

	return deletedObj, nil
}

// RegisterType registers a type with the schema for the mock kubeapiserver
func (s *MemoryStorage) RegisterType(gvk schema.GroupVersionKind, resource string, scope meta.RESTScope) {
	s.schemaMutex.Lock()
	defer s.schemaMutex.Unlock()

	gvr := gvk.GroupVersion().WithResource(resource)
	gr := gvr.GroupResource()

	storage := &resourceStorage{
		GroupResource:        gr,
		resourceVersionClock: &s.resourceVersionClock,
		objects:              make(map[types.NamespacedName]*unstructured.Unstructured),
		parent:               s,
	}

	// TODO: share storage across different versions
	s.resourceStorages[gr] = storage

	r := &memoryResourceInfo{
		api: metav1.APIResource{
			Name:    resource,
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
		gvk:     gvk,
		gvr:     gvr,
		storage: storage,
		parent:  s,
	}
	r.listGVK = gvk.GroupVersion().WithKind(gvk.Kind + "List")

	if gvk.Group == "" {
		parserType := s.schema.builtin.Parser.Type("io.k8s.api.core." + gvk.Version + "." + gvk.Kind)
		r.parseableType = &parserType
	}
	if r.parseableType == nil || !r.parseableType.IsValid() {
		klog.Warningf("type info not known for %v", gvk)
	}

	if scope.Name() == meta.RESTScopeNameNamespace {
		r.api.Namespaced = true
	}

	s.schema.resources = append(s.schema.resources, r)
}

func (s *MemoryStorage) AllResources() []metav1.APIResource {
	s.schemaMutex.Lock()
	defer s.schemaMutex.Unlock()

	var ret []metav1.APIResource
	for _, resource := range s.schema.resources {
		ret = append(ret, resource.api)
	}
	return ret
}

func (s *MemoryStorage) FindResource(gr schema.GroupResource) storage.ResourceInfo {
	s.schemaMutex.Lock()
	defer s.schemaMutex.Unlock()

	for _, resource := range s.schema.resources {
		if resource.gvr.GroupResource() == gr {
			return resource
		}
	}
	return nil
}

func (s *MemoryStorage) findResourceByGVK(gvk schema.GroupVersionKind) storage.ResourceInfo {
	s.schemaMutex.Lock()
	defer s.schemaMutex.Unlock()

	for _, resource := range s.schema.resources {
		if resource.gvk == gvk {
			return resource
		}
	}
	return nil
}

func (s *MemoryStorage) fireOnWatchEvent(ev *storage.WatchEvent) {
	s.hooksMutex.Lock()
	defer s.hooksMutex.Unlock()

	for _, hook := range s.hooks {
		hook.OnWatchEvent(ev)
	}
}
