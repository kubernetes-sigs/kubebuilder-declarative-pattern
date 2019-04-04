package mocks

import (
	"context"
	"fmt"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FakeClient is a struct that implements client.Client for use in tests.
type FakeClient struct {
	c             map[client.ObjectKey]runtime.Object
	ErrIfNotFound bool
}

func NewClient(c map[client.ObjectKey]runtime.Object) FakeClient {
	return FakeClient{
		c: c,
	}
}

func (f FakeClient) Get(ctx context.Context, key client.ObjectKey, out runtime.Object) error {
	obj, ok := f.c[key]
	if !ok {
		if f.ErrIfNotFound {
			return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
		}
		// TODO: should always return NotFound error here. Will need to update affected unit tests to stub the
		// necessary data first.
		return nil
	}
	obj = obj.DeepCopyObject()

	outVal := reflect.ValueOf(out)
	objVal := reflect.ValueOf(obj)
	if !objVal.Type().AssignableTo(outVal.Type()) {
		return fmt.Errorf("cache had type %s, but %s was asked for", objVal.Type(), outVal.Type())
	}
	reflect.Indirect(outVal).Set(reflect.Indirect(objVal))
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
