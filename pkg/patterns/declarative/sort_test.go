/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package declarative

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

func Test_Sort(t *testing.T) {
	crd1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1beta1",
			"kind":       "CustomResourceDefinition",
			"metadata": map[string]interface{}{
				"name":      "test-crd",
				"namespace": "kube-system",
			},
		},
	}
	crdObj1, _ := manifest.NewObject(crd1)

	ns1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata": map[string]interface{}{
				"name":      "test-crd",
				"namespace": "kube-system",
			},
		},
	}
	nsObj1, _ := manifest.NewObject(ns1)

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

	clusterRole1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRole",
			"metadata": map[string]interface{}{
				"name":      "test-clusterrole",
				"namespace": "kube-system",
			},
		},
	}
	clusterRoleObj1, _ := manifest.NewObject(clusterRole1)

	clusterRoleBinding1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "rbac.authorization.k8s.io/v1",
			"kind":       "ClusterRoleBinding",
			"metadata": map[string]interface{}{
				"name":      "test-clusterrolebinding",
				"namespace": "kube-system",
			},
		},
	}
	clusterRoleBindingObj1, _ := manifest.NewObject(clusterRoleBinding1)

	cm1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "test-configmap",
			},
		},
	}
	cmObj1, _ := manifest.NewObject(cm1)

	secret1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Secret",
			"metadata": map[string]interface{}{
				"name": "test-secret",
			},
		},
	}
	secretObj1, _ := manifest.NewObject(secret1)

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

	dp2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "extensions/v1beta1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "frontend222",
			},
		},
	}
	dpObj2, _ := manifest.NewObject(dp2)

	hpa1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "autoscaling/v1",
			"kind":       "HorizontalPodAutoscaler",
			"metadata": map[string]interface{}{
				"name": "test-autoscaler",
			},
		},
	}
	hpaObj1, _ := manifest.NewObject(hpa1)

	svc1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]interface{}{
				"name": "test-service",
			},
		},
	}
	svcObj1, _ := manifest.NewObject(svc1)

	pod1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Pod",
			"metadata": map[string]interface{}{
				"name": "test-pod",
			},
		},
	}
	podObj1, _ := manifest.NewObject(pod1)

	var testcases = []struct {
		name     string
		input    *manifest.Objects
		expected *manifest.Objects
	}{
		{
			name: "multiple objects",
			input: &manifest.Objects{
				Items: []*manifest.Object{
					saObj1,
					crdObj1,
					nsObj1,
					saObj2,
					dpObj1,
					dpObj2,
					clusterRoleObj1,
					hpaObj1,
					podObj1,
					svcObj1,
					clusterRoleBindingObj1,
					cmObj1,
					secretObj1,
				},
			},
			expected: &manifest.Objects{
				Items: []*manifest.Object{
					crdObj1,
					nsObj1,
					saObj2,
					saObj1,
					clusterRoleObj1,
					clusterRoleBindingObj1,
					cmObj1,
					secretObj1,
					podObj1,
					dpObj1,
					dpObj2,
					hpaObj1,
					svcObj1,
				},
			},
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			tc.input.Sort(DefaultObjectOrder(ctx))
			if diff := cmp.Diff(tc.expected, tc.input, cmpopts.IgnoreUnexported(manifest.Object{})); diff != "" {
				t.Errorf("sort result mismatch (-want +got):\n%s", diff)
			}
		})
	}

}
