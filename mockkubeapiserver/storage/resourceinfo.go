package storage

import (
	"context"
	"encoding/json"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/forked"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v4/merge"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
)

// ResourceInfo exposes the storage for a particular resource (group-kind),
// supporting CRUD operations for accessing the objects of that kind.
type ResourceInfo interface {
	GVK() schema.GroupVersionKind
	ListGVK() schema.GroupVersionKind
	ParseableType() *typed.ParseableType

	// SetsGeneration is true if we should automatically set metadata.generation for this resource kind.
	SetsGeneration() bool

	GetObject(ctx context.Context, id types.NamespacedName) (*unstructured.Unstructured, bool, error)

	ListObjects(ctx context.Context, filter ListFilter) (*unstructured.UnstructuredList, error)
	Watch(ctx context.Context, opt WatchOptions, callback WatchCallback) error

	CreateObject(ctx context.Context, id types.NamespacedName, u *unstructured.Unstructured) error
	UpdateObject(ctx context.Context, id types.NamespacedName, u *unstructured.Unstructured) error
	DeleteObject(ctx context.Context, id types.NamespacedName) (*unstructured.Unstructured, error)
}

type ListFilter struct {
	Namespace string
}

type WatchOptions struct {
	Namespace string
}

func DoServerSideApply(ctx context.Context, r ResourceInfo, live *unstructured.Unstructured, patchYAML []byte, options metav1.PatchOptions) (*unstructured.Unstructured, bool, error) {
	parserType := r.ParseableType()
	if parserType == nil || !parserType.IsValid() {
		return nil, false, fmt.Errorf("no type info for %v", r.GVK())
	}

	updater := merge.Updater{}

	liveObject, err := parserType.FromUnstructured(live.Object)
	if err != nil {
		return nil, false, fmt.Errorf("error parsing live object: %w", err)
	}

	configObject, err := parserType.FromYAML(typed.YAMLObject(patchYAML))
	if err != nil {
		return nil, false, fmt.Errorf("error parsing patch object: %w", err)
	}
	force := false
	if options.Force != nil {
		force = *options.Force
	}
	var managers fieldpath.ManagedFields
	manager := metav1.ManagedFieldsEntry{
		Manager: options.FieldManager,
	}
	// TODO: This is surprising ... the manager key is not the manager key, but rather the json-encoded form of the object
	// (at least Decode assumes this)
	managerJSON, err := json.Marshal(manager)
	if err != nil {
		return nil, false, fmt.Errorf("error encoding manager: %w", err)
	}

	apiVersion := fieldpath.APIVersion(r.GVK().GroupVersion().String())
	mergedObject, newManagers, err := updater.Apply(liveObject, configObject, apiVersion, managers, string(managerJSON), force)
	if err != nil {
		return nil, false, fmt.Errorf("error applying patch: %w", err)
	}
	if mergedObject == nil {
		// This indicates that the object was unchanged
		return nil, false, nil
	}

	if mergedObject == nil {
		return nil, false, fmt.Errorf("merged object was nil: %w", err)
	}

	u := &unstructured.Unstructured{}
	u.Object = mergedObject.AsValue().Unstructured().(map[string]interface{})

	times := make(map[string]*metav1.Time)
	if err := forked.EncodeObjectManagedFields(u, newManagers, times); err != nil {
		return nil, false, fmt.Errorf("error from EncodeObjectManagedFields: %w", err)
	}

	return u, true, nil
}
