package memorystorage

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder-declarative-pattern/mockkubeapiserver/storage"
	"sigs.k8s.io/structured-merge-diff/v4/typed"
)

type memoryResourceInfo struct {
	api     metav1.APIResource
	gvr     schema.GroupVersionResource
	gvk     schema.GroupVersionKind
	listGVK schema.GroupVersionKind

	parseableType *typed.ParseableType

	parent *MemoryStorage

	storage *resourceStorage
}

var _ storage.ResourceInfo = &memoryResourceInfo{}

func (r *memoryResourceInfo) GVK() schema.GroupVersionKind {
	return r.gvk
}

func (r *memoryResourceInfo) ListGVK() schema.GroupVersionKind {
	return r.listGVK
}

func (r *memoryResourceInfo) ParseableType() *typed.ParseableType {
	return r.parseableType
}

func (r *memoryResourceInfo) SetsGeneration() bool {
	// Not all resources support metadata.generation; it looks like only those with status do (?)
	// For now, exclude some well-known types that do not set metadata.generation.
	switch r.gvk.GroupKind() {
	case schema.GroupKind{Group: "", Kind: "ConfigMap"}:
		return false
	case schema.GroupKind{Group: "", Kind: "Secret"}:
		return false
	case schema.GroupKind{Group: "", Kind: "Namespace"}:
		return false

	default:
		return true
	}
}
