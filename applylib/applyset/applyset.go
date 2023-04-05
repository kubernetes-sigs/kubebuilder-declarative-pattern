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
	kubectlapply "sigs.k8s.io/kubebuilder-declarative-pattern/applylib/third_party/forked/github.com/kubernetes/kubectl/pkg/cmd/apply"
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
	parentRef := &kubectlapply.ApplySetParentRef{Name: parent.Name(), Namespace: parent.Namespace(), RESTMapping: parent.RESTMapping()}
	kapplyset := kubectlapply.NewApplySet(parentRef, kubectlapply.ApplySetTooling{Name: options.Tooling}, options.RESTMapper)
	options.PatchOptions.FieldManager = kapplyset.FieldManager()
	a := &ApplySet{
		client:        options.Client,
		restMapper:    options.RESTMapper,
		patchOptions:  options.PatchOptions,
		deleteOptions: options.DeleteOptions,
		prune:         options.Prune,
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

type restMappingResult struct {
	restMapping *meta.RESTMapping
	err         error
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
	visitedUids := sets.New[types.UID]()

	// We initialize a new Kubectl ApplySet for each ApplyOnce run. This is because kubectl Applyset is designed for
	// single actuation and not for reconciliation.
	// Note: The Kubectl ApplySet will share the RESTMapper with k-d-p/ApplySet, which caches all the manifests in the past.
	parentRef := &kubectlapply.ApplySetParentRef{Name: a.parent.Name(), Namespace: a.parent.Namespace(), RESTMapping: a.parent.RESTMapping()}
	kapplyset := kubectlapply.NewApplySet(parentRef, kubectlapply.ApplySetTooling{Name: a.tooling}, a.restMapper)

	restMappings := make(map[schema.GroupVersionKind]restMappingResult)
	for i := range trackers.items {
		tracker := &trackers.items[i]
		obj := tracker.desired

		gvk := obj.GroupVersionKind()

		result, found := restMappings[gvk]
		if !found {
			restMapping, err := a.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			result = restMappingResult{
				restMapping: restMapping,
				err:         err,
			}
			restMappings[gvk] = result
		}

		// TODO: Check error is NotFound and not some transient error?
		// TODO: Make sure we don't prune if there were errors

		restMapping := result.restMapping
		if restMapping != nil {
			// cache the GVK in kubectlapply. kubectlapply will use this to calculate
			// the latest parent "applyset.kubernetes.io/contains-group-resources" annotation after applying.
			kapplyset.AddResource(restMapping, obj.GetNamespace())
		}
	}

	for i := range trackers.items {
		tracker := &trackers.items[i]
		obj := tracker.desired

		name := obj.GetName()
		ns := obj.GetNamespace()
		gvk := obj.GroupVersionKind()
		nn := types.NamespacedName{Namespace: ns, Name: name}

		restMappingResult := restMappings[gvk]
		if restMappingResult.err != nil {
			results.applyError(gvk, nn, fmt.Errorf("error getting rest mapping for %v: %w", gvk, restMappingResult.err))
			continue
		}

		restMapping := restMappingResult.restMapping
		if restMapping == nil {
			// Should be impossible
			results.applyError(gvk, nn, fmt.Errorf("rest mapping result not found for %v", gvk))
		}

		if err := a.updateManifestLabel(obj, kapplyset.LabelsForMember()); err != nil {
			klog.Errorf("unable to update label for %v/%v %v: %v", obj.GetName(), obj.GetNamespace(), gvk, err)
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
		visitedUids.Insert(lastApplied.GetUID())
		tracker.lastApplied = lastApplied
		results.applySuccess(gvk, nn)
		tracker.isHealthy = isHealthy(lastApplied)
		results.reportHealth(gvk, nn, tracker.isHealthy)
	}

	if a.prune {
		klog.V(4).Infof("Prune is enabled")
		if err := a.WithParent(kapplyset); err != nil {
			return results, err
		}
		err := func() error {
			pruneObjects, err := kapplyset.FindAllObjectsToPrune(ctx, a.client, visitedUids)
			if err != nil {
				return err
			}
			if err = a.deleteObjects(ctx, pruneObjects, results); err != nil {
				return err
			}
			return nil
		}()
		if err != nil {
			klog.Errorf("encounter error on pruning %v", err)
			if e := a.updateParentLabelsAndAnnotations(kapplyset, "superset"); e != nil {
				return results, e
			}
		}
	}
	if err := a.updateParentLabelsAndAnnotations(kapplyset, "latest"); err != nil {
		klog.Errorf("update parent failed %v", err)
	}
	return results, nil
}

func mergeMap(from, to map[string]string) map[string]string {
	if to == nil {
		to = make(map[string]string)
	}
	for k, v := range from {
		to[k] = v
	}
	return to
}

// updateLabel adds the "applyset.kubernetes.io/part-of: Parent-ID" label to the manifest.
func (a *ApplySet) updateManifestLabel(obj ApplyableObject, applysetLabels map[string]string) error {
	u, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unable to convert `ApplyableObject` to `unstructured.Unstructured` %v/%v %v",
			obj.GetName(), obj.GetNamespace(), obj.GroupVersionKind().String())
	}
	labels := u.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	newLabels := mergeMap(applysetLabels, labels)
	u.SetLabels(newLabels)
	return nil
}

// updateParentLabelsAndAnnotations updates the parent labels and annotations.
// This method leverages the kubectlapply to build the parent labels and annotations, but avoid using the
// `resource.NewHelper` and cmdutil to patch the parent.
func (a *ApplySet) updateParentLabelsAndAnnotations(kapplyset *kubectlapply.ApplySet, mode kubectlapply.ApplySetUpdateMode) error {
	parent, err := meta.Accessor(a.parent.GetSubject())
	if err != nil {
		return err
	}
	partialParent := kapplyset.BuildParentPatch(mode)
	newAnnotations := mergeMap(partialParent.Annotations, parent.GetAnnotations())
	parent.SetAnnotations(newAnnotations)
	newLabels := mergeMap(partialParent.Labels, parent.GetLabels())
	parent.SetLabels(newLabels)
	return nil
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

func (a *ApplySet) WithParent(kapplyset *kubectlapply.ApplySet) error {
	object, err := meta.Accessor(a.parent.GetSubject())
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
		labels[kubectlapply.ApplySetParentIDLabel] = kapplyset.ID()
		object.SetLabels(labels)
	}
	// This is not a cluster fetch. It builds up the parents labels and annotations information in kapplyset.
	return kapplyset.FetchParent(a.parent.GetSubject())
}
