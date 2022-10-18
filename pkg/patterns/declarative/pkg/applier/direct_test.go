/*
Copyright 2022 The Kubernetes Authors.

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

package applier

import (
	"context"
	"fmt"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/rest/fake"
	"k8s.io/client-go/restmapper"
	"k8s.io/kubectl/pkg/cmd/apply"
	kubectltesting "k8s.io/kubectl/pkg/cmd/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
)

type directApplierTestSite struct {
	Error     error
	applyOpts *apply.ApplyOptions
}

func (d *directApplierTestSite) Run(a *apply.ApplyOptions) error {
	d.applyOpts = a
	return nil
}

// same manner with a function newUnstructuredDefaultBuilderWith on k8s.io/cli-runtime/pkg/resource/builder_test.go
func (d *directApplierTestSite) NewBuilder(opt ApplierOptions) *resource.Builder {
	return resource.NewFakeBuilder(
		func(version schema.GroupVersion) (resource.RESTClient, error) {
			return &fake.RESTClient{}, nil
		},
		func() (meta.RESTMapper, error) {
			return testrestmapper.TestOnlyStaticRESTMapper(scheme.Scheme), nil
		},
		func() (restmapper.CategoryExpander, error) {
			return resource.FakeCategoryExpander, nil
		})
}

func (d *directApplierTestSite) NewFactory(opt ApplierOptions) cmdutil.Factory {
	return kubectltesting.NewTestFactory()
}

func newDirectApplierTest(d *directApplierTestSite) Applier {
	return &DirectApplier{inner: d}
}

func TestDirectApply(t *testing.T) {
	tests := []struct {
		name               string
		namespace          string
		manifest           string
		validate           bool
		args               []string
		err                error
		expectApplyOptions *apply.ApplyOptions
		expectCheckFunc    func(opt *apply.ApplyOptions) error
	}{
		{
			name:      "manifest",
			namespace: "",
			manifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: foo-operator
  namespace: kube-system`,
			expectApplyOptions: &apply.ApplyOptions{},
			expectCheckFunc: func(opt *apply.ApplyOptions) error {
				return nil
			},
		},
		{
			name:      "manifest with namespace",
			namespace: "test-namespace",
			manifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: foo-operator
  namespace: kube-system`,
			expectApplyOptions: &apply.ApplyOptions{
				Namespace: "test-namespace",
			},
			expectCheckFunc: func(opt *apply.ApplyOptions) error {
				if opt.Namespace == "test-namespace" {
					return nil
				} else {
					return fmt.Errorf("namespace doesn't match to \"test-namespace\"")
				}
			},
		},
		// This test use fake test factory on "k8s.io/kubectl/pkg/cmd/testing", that's why this test for validate is commented out.
		// opt.Validator is always set to validation.NullSchema instance by fake test factory, so we can't check if validator is set successfully.
		//		{
		//			name:      "manifest with validate",
		//			namespace: "",
		//			manifest: `---
		//apiVersion: v1
		//kind: ServiceAccount
		//metadata:
		//  name: foo-operator
		//  namespace: kube-system`,
		//			validate: true,
		//			args:     []string{},
		//			expectCheckFunc: func(opt *apply.ApplyOptions) error {
		//				if opt.Validator != nil {
		//                  // success pattern, validator is set.
		//					return nil
		//				} else {
		//					return fmt.Errorf("validator is not nil")
		//				}
		//			},
		//		},
		{
			name:      "manifest with prune",
			namespace: "",
			manifest: `---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: foo-operator
  namespace: kube-system`,
			args: []string{"--prune"},
			expectApplyOptions: &apply.ApplyOptions{
				Prune: true,
			},
			expectCheckFunc: func(opt *apply.ApplyOptions) error {
				if opt.Prune != true {
					return fmt.Errorf("prune is not set")
				}
				return nil
			},
		},
		//		{
		//			name:      "manifest with prune and prune-whitelist",
		//			namespace: "",
		//			manifest: `---
		//apiVersion: v1
		//kind: ServiceAccount
		//metadata:
		//  name: foo-operator
		//  namespace: kube-system`,
		//			args: []string{"--prune", "--prune-whitelist=hello-world"},
		//			expectApplyOptions: &apply.ApplyOptions{
		//				Prune: true,
		//			},
		//			expectCheckFunc: func(opt *apply.ApplyOptions) error {
		//				if opt.Prune != true {
		//					return fmt.Errorf("prune is not set")
		//				}
		//				return nil
		//			},
		//		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := &directApplierTestSite{}
			testApplier := newDirectApplierTest(d)

			opts := ApplierOptions{
				Namespace: test.namespace,
				Manifest:  test.manifest,
				Validate:  test.validate,
				ExtraArgs: test.args,
			}

			err := testApplier.Apply(context.Background(), opts)
			if err != nil {
				t.Errorf("unexpected error on call Apply: %v", err)
			}

			if d.applyOpts == nil {
				t.Fatal("unexpected error: ApplyOptions is nil")
			}

			err = test.expectCheckFunc(d.applyOpts)
			if err != nil {
				t.Errorf("unexpected error on ApplyOptions: %v", err)
			}
		})
	}
}

func TestDirectApplier(t *testing.T) {
	applier := NewDirectApplier()
	runApplierGoldenTests(t, "testdata/direct", applier)
}
