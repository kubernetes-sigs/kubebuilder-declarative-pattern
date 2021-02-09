package declarative

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"

	"github.com/stretchr/testify/assert"
)

func Test_uniqueGroupVersionKind(t *testing.T) {
	sa := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":      "foo-operator",
				"namespace": "kube-system",
			},
		},
	}
	saObj, _ := manifest.NewObject(sa)

	tests := []struct {
		name         string
		inputObjects *manifest.Objects
		expectSchema []schema.GroupVersionKind
	}{
		{
			name: "single object",
			inputObjects: &manifest.Objects{
				Items: []*manifest.Object{
					saObj,
				},
			},
			expectSchema: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "ServiceAccount",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSchema := uniqueGroupVersionKind(tt.inputObjects)
			assert.Equal(t, tt.expectSchema, actualSchema)
		})
	}
}
