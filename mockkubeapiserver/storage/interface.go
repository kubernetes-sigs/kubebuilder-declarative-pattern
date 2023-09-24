package storage

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Storage is a pluggable store of objects
type Storage interface {
	// FindResource returns the ResourceInfo for a group-resource.
	// The ResourceInfo allows CRUD operations on that resource.
	FindResource(gr schema.GroupResource) ResourceInfo

	// AllResources returns the metadata for all resources.
	AllResources() []metav1.APIResource

	// AddObject can be called to "sideload" an object, useful for testing.
	AddObject(obj *unstructured.Unstructured) error

	// RegisterType is used to register a built-in type.
	RegisterType(gvk schema.GroupVersionKind, resource string, scope meta.RESTScope)

	// AddStorageHook registers a hook, that will be called whenever any object changes.
	AddStorageHook(hook Hook)

	// UpdateCRD should be called whenever a CRD changes (likely by a hook).
	UpdateCRD(ev *WatchEvent) error
}

// WatchCallback is the function signature for the callback function when objects are changed.
type WatchCallback func(ev *WatchEvent) error
