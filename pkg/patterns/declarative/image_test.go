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

func Test_applyImageRegistry(t *testing.T) {
	u1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test-app",
				},
				"name": "frontend",
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app":  "guestbook",
						"tier": "frontend",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":  "guestbook",
							"tier": "frontend",
						},
					},
					"spec": map[string]interface{}{
						"imagePullSecrets": []interface{}{
							map[string]interface{}{
								"name": "secretsecret",
							},
						},
						"containers": []interface{}{
							map[string]interface{}{
								"image": "dummy-registry/gb-frontend:v4",
								"name":  "php-redis",
							},
						},
					},
				},
			},
		},
	}
	expectedDeployment1, _ := manifest.NewObject(u1)

	dp1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test-app",
				},
				"name": "frontend",
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app":  "guestbook",
						"tier": "frontend",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":  "guestbook",
							"tier": "frontend",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image": "gcr.io/google-samples/gb-frontend:v4",
								"name":  "php-redis",
							},
						},
					},
				},
			},
		},
	}
	dpObj1, _ := manifest.NewObject(dp1)

	u2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test-app",
				},
				"name": "frontend",
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app":  "guestbook",
						"tier": "frontend",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":  "guestbook",
							"tier": "frontend",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image": "gcr.io/google-samples/gb-frontend:v4",
								"name":  "php-redis",
							},
						},
					},
				},
			},
		},
	}
	expectedDeployment2, _ := manifest.NewObject(u2)

	dp2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test-app",
				},
				"name": "frontend",
			},
			"spec": map[string]interface{}{
				"replicas": 3,
				"selector": map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"app":  "guestbook",
						"tier": "frontend",
					},
				},
				"template": map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"app":  "guestbook",
							"tier": "frontend",
						},
					},
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"image": "gcr.io/google-samples/gb-frontend:v4",
								"name":  "php-redis",
							},
						},
					},
				},
			},
		},
	}
	dpObj2, _ := manifest.NewObject(dp2)

	errDp1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test-app",
				},
				"name": "frontend",
			},
		},
	}
	errDpObj1, _ := manifest.NewObject(errDp1)

	errDp2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"labels": map[string]interface{}{
					"app": "test-app",
				},
				"name": "frontend",
			},
		},
	}
	errDpObj2, _ := manifest.NewObject(errDp2)

	var testcases = []struct {
		name                string
		manifest            *manifest.Objects
		expectedObject      *manifest.Object
		registry            string
		secret              string
		error               bool
		expectedErrorString string
	}{
		{
			name: "success pattern",
			manifest: &manifest.Objects{
				Items: []*manifest.Object{
					dpObj1,
				},
			},
			expectedObject:      expectedDeployment1,
			registry:            "dummy-registry",
			secret:              "secretsecret",
			error:               false,
			expectedErrorString: "",
		},
		{
			name: "registry and secret are nil",
			manifest: &manifest.Objects{
				Items: []*manifest.Object{
					dpObj2,
				},
			},
			expectedObject:      expectedDeployment2,
			registry:            "",
			secret:              "",
			error:               false,
			expectedErrorString: "",
		},
		{
			name: "error on mutate registry",
			manifest: &manifest.Objects{
				Items: []*manifest.Object{
					errDpObj1,
				},
			},
			registry:            "dummy-registry",
			secret:              "",
			error:               true,
			expectedErrorString: "error applying private registry: containers not found",
		},
		{
			name: "error on mutate secret",
			manifest: &manifest.Objects{
				Items: []*manifest.Object{
					errDpObj2,
				},
			},
			registry:            "",
			secret:              "dummy-secret",
			error:               true,
			expectedErrorString: "error applying image pull secret: pod spec not found",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			err := applyImageRegistry(ctx, nil, tc.manifest, tc.registry, tc.secret)
			if tc.error == false {
				if diff := cmp.Diff(nil, err); diff != "" {
					t.Errorf("error result mismatch (-want +got):\n%s", diff)
				}
				if diff := cmp.Diff(tc.expectedObject, tc.manifest.Items[0], cmpopts.IgnoreUnexported(manifest.Object{})); diff != "" {
					t.Errorf("object result mismatch (-want +got):\n%s", diff)
				}
			} else {
				if diff := cmp.Diff(tc.expectedErrorString, err.Error()); diff != "" {
					t.Errorf("error result mismatch (-want +got):\n%s", diff)
				}
			}
		})
	}
}
