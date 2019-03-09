package mocks

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeClient is a struct that implements client.Client for use in tests.
type FakeClient struct{}

func (FakeClient) Get(ctx context.Context, key client.ObjectKey, obj runtime.Object) error {
	return nil
}

func (FakeClient) List(ctx context.Context, opts *client.ListOptions, list runtime.Object) error {
	panic("not implemented")
}

func (FakeClient) Create(ctx context.Context, obj runtime.Object) error {
	panic("not implemented")
}

func (FakeClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOptionFunc) error {
	return nil
}

func (FakeClient) Update(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (FakeClient) Status() client.StatusWriter {
	panic("not implemented")
}
