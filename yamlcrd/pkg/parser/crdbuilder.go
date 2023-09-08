package parser

import (
	"context"
	"encoding/json"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func buildCRDs(ctx context.Context, typeDefinitions []TypeDefinition) ([]*unstructured.Unstructured, error) {
	b := &crdBuilder{}

	crds := b.buildCRDs(typeDefinitions)

	if b.Err() != nil {
		return nil, b.Err()
	}
	return crds, nil
}

type crdBuilder struct {
	builder
}

func (b *crdBuilder) buildCRDs(typeDefinitions []TypeDefinition) []*unstructured.Unstructured {
	if b.Err() != nil {
		return nil
	}

	var crds []*unstructured.Unstructured

	for _, typeDefinition := range typeDefinitions {
		crds = append(crds, b.buildCRD(typeDefinition))
	}
	return crds
}

func (b *crdBuilder) buildCRD(typeDefinition TypeDefinition) *unstructured.Unstructured {
	if b.Err() != nil {
		return nil
	}

	group := typeDefinition.Group
	kind := typeDefinition.Kind
	listKind := kind + "List"
	singular := strings.ToLower(kind)
	plural := singular + "s"
	scope := typeDefinition.Scope
	if scope == "" {
		scope = "Namespaced"
	}

	name := plural + "." + group

	crd := &unstructured.Unstructured{}
	crd.SetName(name) // populates values
	crd.SetAPIVersion("apiextensions.k8s.io/v1")
	crd.SetKind("CustomResourceDefinition")

	unstructured.SetNestedField(crd.Object, group, "spec", "group")
	unstructured.SetNestedField(crd.Object, scope, "spec", "scope")
	unstructured.SetNestedField(crd.Object, kind, "spec", "names", "kind")
	unstructured.SetNestedField(crd.Object, listKind, "spec", "names", "listKind")
	unstructured.SetNestedField(crd.Object, singular, "spec", "names", "singular")
	unstructured.SetNestedField(crd.Object, plural, "spec", "names", "plural")

	var versions []interface{}

	{
		version := map[string]any{}
		version["name"] = "v1alpha1"

		openapiJSON, err := json.Marshal(typeDefinition.Schema)
		if err != nil {
			b.Errorf("error converting openapi to json: %w", err)
			return nil
		}
		openAPIUnstructured := make(map[string]any)
		if err := json.Unmarshal(openapiJSON, &openAPIUnstructured); err != nil {
			b.Errorf("error parsing openapi json: %w", err)
			return nil
		}
		version["schema"] = openAPIUnstructured
		version["served"] = true
		version["storage"] = true
		version["subresources"] = map[string]any{
			"status": map[string]any{},
		}

		versions = append(versions, version)
	}

	unstructured.SetNestedSlice(crd.Object, versions, "spec", "versions")
	return crd
}
