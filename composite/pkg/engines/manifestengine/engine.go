package manifestengine

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

type Engine struct {
	restMapper    meta.RESTMapper
	dynamicClient dynamic.Interface
}

func NewEngine(restMapper meta.RESTMapper, dynamicClient dynamic.Interface) *Engine {
	return &Engine{
		restMapper:    restMapper,
		dynamicClient: dynamicClient,
	}
}

func (e *Engine) BuildObjects(ctx context.Context, fileName string, definition string, subject *unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	objects, err := manifest.ParseObjects(ctx, definition)
	if err != nil {
		return nil, err
	}
	var ret []*unstructured.Unstructured
	for _, obj := range objects.GetItems() {
		ret = append(ret, obj.UnstructuredObject())
	}
	return ret, nil
}
