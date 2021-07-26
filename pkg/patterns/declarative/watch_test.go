package declarative

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

func Test_uniqueGroupVersionKind(t *testing.T) {
	sa1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":      "foo-operator",
				"namespace": "kube-system",
			},
		},
	}
	saObj1, _ := manifest.NewObject(sa1)

	sa2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ServiceAccount",
			"metadata": map[string]interface{}{
				"name":      "bar-operator",
				"namespace": "kube-system",
			},
		},
	}
	saObj2, _ := manifest.NewObject(sa2)

	dp1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "frontend111",
			},
		},
	}
	dpObj1, _ := manifest.NewObject(dp1)

	tests := []struct {
		name           string
		inputObjects   *manifest.Objects
		expectedSchema []schema.GroupVersionKind
	}{
		{
			name: "single object",
			inputObjects: &manifest.Objects{
				Items: []*manifest.Object{
					saObj1,
				},
			},
			expectedSchema: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "ServiceAccount",
				},
			},
		},
		{
			name: "double same object",
			inputObjects: &manifest.Objects{
				Items: []*manifest.Object{
					saObj1,
					saObj1,
				},
			},
			expectedSchema: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "ServiceAccount",
				},
			},
		},
		{
			name: "double same type of object",
			inputObjects: &manifest.Objects{
				Items: []*manifest.Object{
					saObj1,
					saObj2,
				},
			},
			expectedSchema: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "ServiceAccount",
				},
			},
		},
		{
			name: "multiple objects",
			inputObjects: &manifest.Objects{
				Items: []*manifest.Object{
					dpObj1,
					saObj2,
				},
			},
			expectedSchema: []schema.GroupVersionKind{
				{
					Group:   "",
					Version: "v1",
					Kind:    "ServiceAccount",
				},
				{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
			},
		},
		{
			name: "empty objects",
			inputObjects: &manifest.Objects{
				Items: []*manifest.Object{},
			},
			expectedSchema: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSchema := uniqueGroupVersionKind(tt.inputObjects)
			if diff := cmp.Diff(tt.expectedSchema, actualSchema); diff != "" {
				t.Errorf("schema mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
