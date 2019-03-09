package mocks

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeCache struct {
}

func (FakeCache) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return errors.NewNotFound(schema.GroupResource{}, "")
}

func (FakeCache) List(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
	panic("implement me")
}

func (FakeCache) GetInformer(obj runtime.Object) (toolscache.SharedIndexInformer, error) {
	panic("implement me")
}

func (FakeCache) GetInformerForKind(gvk schema.GroupVersionKind) (toolscache.SharedIndexInformer, error) {
	panic("implement me")
}

func (FakeCache) Start(stopCh <-chan struct{}) error {
	panic("implement me")
}

func (FakeCache) WaitForCacheSync(stop <-chan struct{}) bool {
	panic("implement me")
}

func (FakeCache) IndexField(obj runtime.Object, field string, extractValue client.IndexerFunc) error {
	panic("implement me")
}
