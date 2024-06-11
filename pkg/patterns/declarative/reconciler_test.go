/*
Copyright 2024 The Kubernetes Authors.

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
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

func Test_TransformNestedManifests(t *testing.T) {
	inputManifest := `apiVersion: v1
data:
  manifest.yaml: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      labels:
        app: test-app
      name: frontend
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      strategy: {}
      template:
        metadata:
          labels:
            app: test-app
        spec:
          containers:
          - image: busybox
            name: busybox
kind: ConfigMap
metadata:
  name: foo
  namespace: test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: test-app
  name: backend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  strategy: {}
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - image: busybox
        name: busybox
---
apiVersion: v1
data:
  manifest.yaml: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      labels:
        app: test-app
      name: frontend
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      strategy: {}
      template:
        metadata:
          labels:
            app: test-app
        spec:
          containers:
          - image: busybox
            name: busybox
kind: ConfigMap
metadata:
  name: cm-with-nested-deployment
  namespace: test-image-transform
`
	var testCases = []struct {
		name            string
		inputManifest   string
		registry        string
		imagePullSecret string
		expected        string
	}{
		{
			name:            "transform with registry and imagePullSecret",
			inputManifest:   inputManifest,
			registry:        "gcr.io/foo/bar",
			imagePullSecret: "some-secret",
			expected: `apiVersion: v1
data:
  manifest.yaml: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      labels:
        app: test-app
      name: frontend
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      strategy: {}
      template:
        metadata:
          labels:
            app: test-app
        spec:
          containers:
          - image: busybox
            name: busybox
kind: ConfigMap
metadata:
  name: foo
  namespace: test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: test-app
  name: backend
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app
  strategy: {}
  template:
    metadata:
      labels:
        app: test-app
    spec:
      containers:
      - image: gcr.io/foo/bar/busybox
        name: busybox
      imagePullSecrets:
      - name: some-secret
---
apiVersion: v1
data:
  manifest.yaml: |
    apiVersion: apps/v1
    kind: Deployment
    metadata:
      labels:
        app: test-app
      name: frontend
    spec:
      replicas: 1
      selector:
        matchLabels:
          app: test-app
      strategy: {}
      template:
        metadata:
          labels:
            app: test-app
        spec:
          containers:
          - image: gcr.io/foo/bar/busybox
            name: busybox
          imagePullSecrets:
          - name: some-secret
kind: ConfigMap
metadata:
  name: cm-with-nested-deployment
  namespace: test-image-transform
`,
		},
		{
			name:          "transform without registry or imagePullSecret",
			inputManifest: inputManifest,
			expected:      inputManifest,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			objects, err := manifest.ParseObjects(ctx, tc.inputManifest)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			r := Reconciler{
				options: reconcilerParams{
					nestedManifestFn: func(m *manifest.Object) ([][]string, error) {
						if m.Kind == "ConfigMap" && m.GetName() == "cm-with-nested-deployment" &&
							m.GetNamespace() == "test-image-transform" {
							return [][]string{
								{"data", "manifest.yaml"},
							}, nil
						}
						return nil, nil
					},
					objectTransformations: []ObjectTransform{
						ImageRegistryTransform(tc.registry, tc.imagePullSecret),
					},
				},
			}

			if err := r.transformManifest(ctx, nil, objects); err != nil {
				t.Fatal(err)
			}

			out, err := objects.ToYAML()
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.expected, out); diff != "" {
				t.Fatalf("result mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
