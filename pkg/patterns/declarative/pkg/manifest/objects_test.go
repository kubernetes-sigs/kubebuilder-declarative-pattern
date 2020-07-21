package manifest

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func Test_Object(t *testing.T) {
	tests := []struct {
		name           string
		inputManifest  string
		expectedObject []*Object
		expectedBlobs  []string
	}{
		{
			name: "simple applied manifest",
			inputManifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: foo-operator
  namespace: kube-system`,
			expectedObject: []*Object{
				{
					object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ServiceAccount",
							"metadata": map[string]interface{}{
								"name":      "foo-operator",
								"namespace": "kube-system",
							},
						},
					},
				},
			},
			expectedBlobs: []string{},
		},
		{
			name: "simple kustomization manifest",
			inputManifest: `---
resources:
	- services.yaml
	- deployment.yaml
configMapGenerator:
- name: coredns
	namespace: kube-system
	files:
	- Corefile`,
			expectedObject: []*Object{},
			expectedBlobs: []string{
				`resources:
	- services.yaml
	- deployment.yaml
configMapGenerator:
- name: coredns
	namespace: kube-system
	files:
	- Corefile
`,
			},
		},
		{
			name: "a simple and kustomization manifest",
			inputManifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: foo-operator
  namespace: kube-system
---
resources:
	- services.yaml
	- deployment.yaml
configMapGenerator:
- name: coredns
	namespace: kube-system
	files:
	- Corefile`,
			expectedObject: []*Object{
				{
					object: &unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "ServiceAccount",
							"metadata": map[string]interface{}{
								"name":      "foo-operator",
								"namespace": "kube-system",
							},
						},
					},
				},
			},
			expectedBlobs: []string{
				`resources:
	- services.yaml
	- deployment.yaml
configMapGenerator:
- name: coredns
	namespace: kube-system
	files:
	- Corefile
`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			returnedObj, err := ParseObjects(ctx, tt.inputManifest)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}

			if len(tt.expectedObject) != len(returnedObj.Items) {
				t.Errorf("Expected length of %v to be %v but is %v", returnedObj.Items, len(tt.expectedObject),
					len(returnedObj.Items))
			}

			if len(tt.expectedBlobs) != len(returnedObj.Blobs) {
				t.Errorf("Expected length of %v to be %v but is %v", returnedObj.Blobs, len(tt.expectedBlobs),
					len(returnedObj.Blobs))
			}

			for i, actual := range returnedObj.Blobs {
				actualStr := string(actual)
				expectedStr := tt.expectedBlobs[i]
				if expectedStr != actualStr {
					t.Fatalf("unexpected result, expected ========\n%v\n\nactual ========\n%v\n", expectedStr, actualStr)
				}
			}

			for i, actual := range returnedObj.Items {
				actualBytes, err := actual.JSON()
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				expectedBytes, err := tt.expectedObject[i].JSON()
				if err != nil {
					t.Fatalf("unexpected err: %v", err)
				}
				actualStr := string(actualBytes)
				expectedStr := string(expectedBytes)
				if expectedStr != actualStr {
					t.Fatalf("unexpected result, expected ========\n%v\n\nactual ========\n%v\n", expectedStr, actualStr)
				}
			}
		})
	}
}
