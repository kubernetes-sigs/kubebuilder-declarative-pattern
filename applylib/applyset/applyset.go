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

package applyset

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/kubebuilder-declarative-pattern/applylib/applyset/forked/kubectlapply"
)

// ApplySet is a set of objects that we want to apply to the cluster.
//
// An ApplySet has a few cases which it tries to optimize for:
// * We can change the objects we're applying
// * We want to watch the objects we're applying / be notified of changes
// * We want to know when the objects we apply are "healthy"
// * We expose a "try once" method to better support running from a controller.
//
// TODO: Pluggable health functions.
type ApplySet struct {
	// client is the dynamic kubernetes client used to apply objects to the k8s cluster.
	client dynamic.Interface
	// restMapper is used to map object kind to resources, and to know if objects are cluster-scoped.
	restMapper meta.RESTMapper
	// patchOptions holds the options used when applying, in particular the fieldManager
	patchOptions metav1.PatchOptions

	// deleteOptions holds the options used when pruning
	deleteOptions metav1.DeleteOptions

	// mutex guards trackers
	mutex sync.Mutex
	// trackers is a (mutable) pointer to the (immutable) objectTrackerList, containing a list of objects we are applying.
	trackers *objectTrackerList

	// whether to prune the previously objects that are no longer in the current deployment manifest list.
	// Finding the objects to prune is done via "apply-set" labels and annotations. See KEP
	// https://github.com/KnVerey/enhancements/blob/b347756461679f62cf985e7a6b0fd0bc28ea9fd2/keps/sig-cli/3659-kubectl-apply-prune/README.md#optional-hint-annotations
	prune bool
	// Parent provides the necessary methods to determine a ApplySet parent object, which can be used to find out all the on-track
	// deployment manifests.
	parent Parent
	// Leveraging the applyset from kubectl. Eventually, we expect to use a tool-neutral library that can provide the key ApplySet
	// methods. Nowadays, k8s.io/kubectl/pkg/cmd/apply provides the most complete ApplySet library. However, it is bundled
	// with kubectl and can hardly be used directly by kubebuilder-declarative-pattern.
	innerApplySet *kubectlapply.ApplySet
	// If not given, the tooling value will be the `Parent` Kind.
	tooling string
}

// Options holds the parameters for building an ApplySet.
type Options struct {
	// Client is the dynamic kubernetes client used to apply objects to the k8s cluster.
	Client dynamic.Interface
	// RESTMapper is used to map object kind to resources, and to know if objects are cluster-scoped.
	RESTMapper meta.RESTMapper
	// PatchOptions holds the options used when applying, in particular the fieldManager
	PatchOptions  metav1.PatchOptions
	DeleteOptions metav1.DeleteOptions
	Prune         bool
	Parent        Parent
	Tooling       string
}

// New constructs a new ApplySet
func New(options Options) (*ApplySet, error) {
	parent := options.Parent
	parentRef := &kubectlapply.ApplySetParentRef{Name: parent.Name(), RESTMapping: parent.RESTMapping()}
	kapplyset := kubectlapply.NewApplySet(parentRef, kubectlapply.ApplySetTooling{Name: options.Tooling}, options.RESTMapper)
	options.PatchOptions.FieldManager = kapplyset.FieldManager()
	a := &ApplySet{
		client:        options.Client,
		restMapper:    options.RESTMapper,
		patchOptions:  options.PatchOptions,
		deleteOptions: options.DeleteOptions,
		prune:         options.Prune,
		innerApplySet: kapplyset,
		parent:        parent,
		tooling:       options.Tooling,
	}
	a.trackers = &objectTrackerList{}
	return a, nil
}

// SetDesiredObjects is used to replace the desired state of all the objects.
// Any objects not specified are removed from the "desired" set.
func (a *ApplySet) SetDesiredObjects(objects []ApplyableObject) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	newTrackers := a.trackers.setDesiredObjects(objects)
	a.trackers = newTrackers

	return nil
}

// ApplyOnce will make one attempt to apply all objects and observe their health.
// It does not wait for the objects to become healthy, but will report their health.
//
// TODO: Limit the amount of time this takes, particularly if we have thousands of objects.
//
//	We don't _have_ to try to apply all objects if it is taking too long.
//
// TODO: We re-apply every object every iteration; we should be able to do better.
func (a *ApplySet) ApplyOnce(ctx context.Context) (*ApplyResults, error) {
	// snapshot the state
	a.mutex.Lock()
	trackers := a.trackers
	a.mutex.Unlock()

	results := &ApplyResults{total: len(trackers.items)}
	if err := a.BeforeApply(ctx); err != nil {
		return nil, err
	}
	for i := range trackers.items {
		tracker := &trackers.items[i]
		obj := tracker.desired

		name := obj.GetName()
		ns := obj.GetNamespace()
		gvk := obj.GroupVersionKind()
		nn := types.NamespacedName{Namespace: ns, Name: name}

		restMapping, err := a.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			results.applyError(gvk, nn, fmt.Errorf("error getting rest mapping for %v: %w", gvk, err))
			continue
		}
		gvr := restMapping.Resource

		var dynamicResource dynamic.ResourceInterface

		switch restMapping.Scope.Name() {
		case meta.RESTScopeNameNamespace:
			if ns == "" {
				// TODO: Differentiate between server-fixable vs client-fixable errors?
				results.applyError(gvk, nn, fmt.Errorf("namespace was not provided for namespace-scoped object %v", gvk))
				continue
			}
			dynamicResource = a.client.Resource(gvr).Namespace(ns)

		case meta.RESTScopeNameRoot:
			if ns != "" {
				// TODO: Differentiate between server-fixable vs client-fixable errors?
				results.applyError(gvk, nn, fmt.Errorf("namespace %q was provided for cluster-scoped object %v", obj.GetNamespace(), gvk))
				continue
			}
			dynamicResource = a.client.Resource(gvr)

		default:
			// Internal error ... this is panic-level
			return nil, fmt.Errorf("unknown scope for gvk %s: %q", gvk, restMapping.Scope.Name())
		}
		j, err := json.Marshal(obj)
		if err != nil {
			// TODO: Differentiate between server-fixable vs client-fixable errors?
			results.applyError(gvk, nn, fmt.Errorf("failed to marshal object to JSON: %w", err))
			continue
		}

		lastApplied, err := dynamicResource.Patch(ctx, name, types.ApplyPatchType, j, a.patchOptions)
		if err != nil {
			results.applyError(gvk, nn, fmt.Errorf("error from apply: %w", err))
			continue
		}

		tracker.lastApplied = lastApplied
		results.applySuccess(gvk, nn)
		tracker.isHealthy = isHealthy(lastApplied)
		results.reportHealth(gvk, nn, tracker.isHealthy)
	}

	if a.prune {
		klog.Info("prune is enabled")
		err := func() error {
			allObjects, err := a.innerApplySet.FindAllObjectsToPrune(ctx, a.client, sets.New[types.UID]())
			if err != nil {
				return err
			}
			pruneObjects := a.excludeCurrent(allObjects)
			if err = a.deleteObjects(ctx, pruneObjects, results); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			klog.Errorf("prune failed %v", err)
			if e := a.updateParent(ctx, "superset"); e != nil {
				klog.Errorf("update parent failed %v", e)
			}
			return results, nil
		}
		klog.Info("prune succeed")
	}
	if err := a.updateParent(ctx, "latest"); err != nil {
		klog.Errorf("update parent failed %v", err)
	}
	return results, nil
}

// BeforeApply updates the parent and manifests labels and annotations.
// This method is adjusted for kubectlapply which requests
// 1. the manifests with its RESTMappings. This is from the ControllerRESTMapper cache.
// 2. the parent in the cluster must already have labels and annotations. This means the `FetchParent` will fail at first
// until the `updateParent` updates the parent labels and annotations in the cluster. This is why the FetchParent continue wth error.
func (a *ApplySet) BeforeApply(ctx context.Context) error {
	if err := a.FetchParent(ctx); err != nil {
		klog.Errorf("fetch parent error %v", err.Error())
	}
	for _, obj := range a.trackers.items {
		gvk := obj.desired.GroupVersionKind()
		restmapping, err := a.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			klog.Errorf("unable to get restmapping for %v: %v", gvk, err)
			continue
		}
		a.innerApplySet.AddResource(restmapping, obj.desired.GetNamespace())
		if err = a.updateLabel(obj.desired); err != nil {
			klog.Errorf("unable to update label for %v: %w", gvk, err)
		}
	}
	if err := a.updateParent(ctx, "superset"); err != nil {
		klog.Errorf("before apply: update parent error %v", err.Error())
	}
	return nil
}

// updateLabel adds the "applyset.kubernetes.io/part-of: Parent-ID" label to the manifest.
func (a *ApplySet) updateLabel(obj ApplyableObject) error {
	applysetLabels := a.innerApplySet.LabelsForMember()
	data, err := obj.MarshalJSON()
	if err != nil {
		return err
	}
	var u unstructured.Unstructured
	if err = u.UnmarshalJSON(data); err != nil {
		return err
	}
	labels := u.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	for k, v := range applysetLabels {
		labels[k] = v
	}
	u.SetLabels(labels)
	newData, err := u.MarshalJSON()
	if err != nil {
		return err
	}
	json.Unmarshal(newData, obj)
	return nil
}

// updateParent updates the parent labels and annotations.
// This method leverages the kubectlapply to build the parent labels and annotations, but avoid using the
// `resource.NewHelper` and cmdutil to patch the parent.
func (a *ApplySet) updateParent(ctx context.Context, mode kubectlapply.ApplySetUpdateMode) error {
	data, err := json.Marshal(a.innerApplySet.BuildParentPatch(mode))
	if err != nil {
		return fmt.Errorf("failed to encode patch for ApplySet parent: %w", err)
	}
	if _, err = a.client.Resource(a.parent.RESTMapping().Resource).Patch(ctx, a.parent.Name(), types.ApplyPatchType, data, a.patchOptions); err != nil {
		klog.Warningf("unable to patch parent before apply: %v", err)
		return err
	}
	return nil
}

func (a *ApplySet) excludeCurrent(allObjects []kubectlapply.PruneObject) []kubectlapply.PruneObject {
	gvknnKey := func(gvk schema.GroupVersionKind, name, namespace string) string {
		return gvk.String() + name + namespace
	}
	desiredObj := make(map[string]struct{})
	for _, obj := range a.trackers.items {
		gvk := obj.desired.GroupVersionKind()
		name := obj.desired.GetName()
		ns := obj.desired.GetNamespace()
		desiredObj[gvknnKey(gvk, name, ns)] = struct{}{}
	}
	var pruneList []kubectlapply.PruneObject
	for _, p := range allObjects {
		gvk := p.Object.GetObjectKind().GroupVersionKind()
		if _, ok := desiredObj[gvknnKey(gvk, p.Name, p.Namespace)]; !ok {
			pruneList = append(pruneList, p)
		}
	}
	return pruneList
}

func (a *ApplySet) deleteObjects(ctx context.Context, pruneObjects []kubectlapply.PruneObject, results *ApplyResults) error {
	for i := range pruneObjects {
		pruneObject := &pruneObjects[i]

		name := pruneObject.Name
		namespace := pruneObject.Namespace
		mapping := pruneObject.Mapping
		gvk := pruneObject.Object.GetObjectKind().GroupVersionKind()
		nn := types.NamespacedName{Namespace: namespace, Name: name}

		if err := a.client.Resource(mapping.Resource).Namespace(namespace).Delete(ctx, name, a.deleteOptions); err != nil {
			klog.Error("unable to delete resource ")
			results.pruneError(gvk, nn, fmt.Errorf("error from delete: %w", err))
		} else {
			klog.Infof("pruned resource %v", pruneObject.String())
			results.pruneSuccess(gvk, nn)
		}
	}
	return nil
}

func (a *ApplySet) FetchParent(ctx context.Context) error {
	object, err := a.client.Resource(a.parent.RESTMapping().Resource).Namespace(a.parent.Namespace()).Get(
		ctx, a.parent.Name(), metav1.GetOptions{})
	if err != nil {
		return err
	}
	//kubectlapply requires the tooling and id to exist.
	{
		annotations := object.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		annotations[kubectlapply.ApplySetToolingAnnotation] = a.tooling
		if _, ok := annotations[kubectlapply.ApplySetGRsAnnotation]; !ok {
			annotations[kubectlapply.ApplySetGRsAnnotation] = ""
		}
		object.SetAnnotations(annotations)

		labels := object.GetLabels()
		if labels == nil {
			labels = make(map[string]string)
		}
		labels[kubectlapply.ApplySetParentIDLabel] = a.innerApplySet.ID()
		object.SetLabels(labels)
	}
	return a.innerApplySet.FetchParent(object)
}
