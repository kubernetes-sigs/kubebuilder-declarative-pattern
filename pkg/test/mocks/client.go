package mocks

import (
	"context"

	"encoding/json"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// FakeClient is a struct that implements client.Client for use in tests.
type FakeClient struct {
	ErrIfNotFound bool
	tracker       testing.ObjectTracker
	scheme        *runtime.Scheme
}

func NewClient(clientScheme *runtime.Scheme) FakeClient {
	tracker := testing.NewObjectTracker(clientScheme, scheme.Codecs.UniversalDecoder())
	return FakeClient{
		tracker: tracker,
		scheme:  clientScheme,
	}
}

func (f FakeClient) Get(ctx context.Context, key client.ObjectKey, out runtime.Object) error {
	gvr, err := getGVRFromObject(out, f.scheme)
	if err != nil {
		return err
	}
	o, err := f.tracker.Get(gvr, key.Namespace, key.Name)
	if err != nil {
		//return err
		if f.ErrIfNotFound {
			return apierrors.NewNotFound(schema.GroupResource{}, key.Name)
		}
		// TODO: should always return NotFound error here. Will need to update affected unit tests to stub the
		// necessary data first.
		return nil
	}
	j, err := json.Marshal(o)
	if err != nil {
		return err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	_, _, err = decoder.Decode(j, nil, out)
	return err
}

func (FakeClient) List(ctx context.Context, list runtime.Object, opts ...client.ListOption) error {
	panic("not implemented")
}

func (f FakeClient) Create(ctx context.Context, obj runtime.Object, opts ...client.CreateOption) error {
	createOptions := &client.CreateOptions{}
	createOptions.ApplyOptions(opts)

	for _, dryRunOpt := range createOptions.DryRun {
		if dryRunOpt == metav1.DryRunAll {
			return nil
		}
	}

	gvr, err := getGVRFromObject(obj, f.scheme)
	if err != nil {
		return err
	}
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return err
	}
	return f.tracker.Create(gvr, obj, accessor.GetNamespace())
}

func (FakeClient) Delete(ctx context.Context, obj runtime.Object, opts ...client.DeleteOption) error {
	return nil
}

func (FakeClient) DeleteAllOf(ctx context.Context, obj runtime.Object, opts ...client.DeleteAllOfOption) error {
	return nil
}

func (FakeClient) Patch(ctx context.Context, obj runtime.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

func (FakeClient) Update(ctx context.Context, obj runtime.Object, opts ...client.UpdateOption) error {
	return nil
}

func (FakeClient) Status() client.StatusWriter {
	panic("not implemented")
}

func getGVRFromObject(obj runtime.Object, scheme *runtime.Scheme) (schema.GroupVersionResource, error) {
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}
	gvr, _ := meta.UnsafeGuessKindToResource(gvk)
	return gvr, nil
}
