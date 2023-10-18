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
