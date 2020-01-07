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

package applier

import (
	"context"
	"errors"
	"io/ioutil"
	"os/exec"
	"reflect"
	"testing"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
)

// collector is a commandSite implementation that stubs cmd.Run() calls for tests
type collector struct {
	Error error
	Cmds  []*exec.Cmd
}

func (s *collector) Run(c *exec.Cmd) error {
	s.Cmds = append(s.Cmds, c)
	return s.Error
}

func TestKubectlApply(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		manifest   manifest.Objects
		validate   bool
		args       []string
		err        error
		expectArgs []string
	}{
		{
			name:       "manifest",
			namespace:  "",
			expectArgs: []string{"kubectl", "apply", "--validate=false", "-f", "-"},
		},
		{
			name:       "manifest with apply",
			namespace:  "kube-system",
			expectArgs: []string{"kubectl", "apply", "-n", "kube-system", "--validate=false", "-f", "-"},
		},
		{
			name:       "manifest with validate",
			namespace:  "",
			validate:   true,
			expectArgs: []string{"kubectl", "apply", "--validate=true", "-f", "-"},
		},
		{
			name:       "error propagation",
			expectArgs: []string{"kubectl", "apply", "--validate=false", "-f", "-"},
			err:        errors.New("error"),
		},
		{
			name:       "manifest with prune",
			namespace:  "kube-system",
			args:       []string{"--prune=true", "--prune-whitelist=hello-world"},
			expectArgs: []string{"kubectl", "apply", "-n", "kube-system", "--validate=false", "--prune=true", "--prune-whitelist=hello-world", "-f", "-"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := collector{Error: test.err}
			kubectl := &ExecKubectl{cmdSite: &cs}
			err := kubectl.Apply(context.Background(), test.namespace, &test.manifest, test.validate, test.args...)

			if test.err != nil && err == nil {
				t.Error("expected error to occur")
			} else if test.err == nil && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if len(cs.Cmds) != 1 {
				t.Errorf("expected 1 command to be invoked, got: %d", len(cs.Cmds))
			}

			cmd := cs.Cmds[0]
			if !reflect.DeepEqual(cmd.Args, test.expectArgs) {
				t.Errorf("argument mismatch, expected: %v, got: %v", test.expectArgs, cmd.Args)
			}

			manifestJSON, err := test.manifest.JSONManifest()
			if err != nil {
				t.Errorf("unable to convert manifest to JSON: %v", err)
			}

			stdinBytes, err := ioutil.ReadAll(cmd.Stdin)
			if stdin := string(stdinBytes); stdin != manifestJSON {
				t.Errorf("manifest mismatch, expected: %v, got: %v", manifestJSON, stdin)
			}
		})
	}

}
