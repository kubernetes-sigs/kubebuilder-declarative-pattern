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
package mockkubeapiserver

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

type MemoryStorage struct {
	mutex   sync.Mutex
	schema  mockSchema
	objects map[schema.GroupResource]*objectList
}

func NewMemoryStorage() *MemoryStorage {
	s := &MemoryStorage{
		objects: make(map[schema.GroupResource]*objectList),
	}
	return s
}

type mockSchema struct {
	resources []mockSchemaResource
}

type mockSchemaResource struct {
	metav1.APIResource
}

type objectList struct {
	GroupResource schema.GroupResource
	Objects       map[types.NamespacedName]*unstructured.Unstructured
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

	for _, resource := range s.schema.resources {
		if resource.Group != gv.Group || resource.Version != gv.Version {
			continue
		}
		if resource.Kind != kind {
			continue
		}

		gr := schema.GroupResource{Group: resource.Group, Resource: resource.Name}

		return s.PutObject(ctx, gr, id, obj)
	}
	gvk := gv.WithKind(kind)
	return fmt.Errorf("object group/version/kind %v not known", gvk)
}

func (s *MemoryStorage) GetObject(ctx context.Context, gr schema.GroupResource, id types.NamespacedName) (*unstructured.Unstructured, bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	objects := s.objects[gr]
	if objects == nil {
		return nil, false, nil
	}

	object := objects.Objects[id]
	if object == nil {
		return nil, false, nil
	}

	return object, true, nil
}

func (s *MemoryStorage) PutObject(ctx context.Context, gr schema.GroupResource, id types.NamespacedName, u *unstructured.Unstructured) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	objects := s.objects[gr]
	if objects == nil {
		objects = &objectList{
			GroupResource: gr,
			Objects:       make(map[types.NamespacedName]*unstructured.Unstructured),
		}
		s.objects[gr] = objects
	}

	objects.Objects[id] = u
	s.objectChanged(u)
	return nil
}

// RegisterType registers a type with the schema for the mock kubeapiserver
func (s *MemoryStorage) RegisterType(gvk schema.GroupVersionKind, resource string, scope meta.RESTScope) {
	r := mockSchemaResource{
		APIResource: metav1.APIResource{
			Name:    resource,
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind,
		},
	}
	if scope.Name() == meta.RESTScopeNameNamespace {
		r.Namespaced = true
	}

	s.schema.resources = append(s.schema.resources, r)
}

func (s *MemoryStorage) AllResources() []metav1.APIResource {
	var ret []metav1.APIResource
	for _, resource := range s.schema.resources {
		ret = append(ret, resource.APIResource)
	}
	return ret
}
