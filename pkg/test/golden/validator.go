/*
Copyright 2018 The Kubernetes Authors.

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

package golden

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"os/exec"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	"sigs.k8s.io/controller-runtime/pkg/runtime/scheme"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/addon/pkg/loaders"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/mocks"
)

func NewValidator(t *testing.T, b *scheme.Builder) *validator {
	v := &validator{T: t, scheme: runtime.NewScheme()}
	if err := b.AddToScheme(v.scheme); err != nil {
		t.Fatalf("error from AddToScheme: %v", err)
	}

	v.T.Helper()
	addon.Init()
	v.findChannelsPath()

	v.mgr.Scheme = v.scheme
	return v
}

type validator struct {
	T       *testing.T
	scheme  *runtime.Scheme
	TestDir string
	mgr     mocks.Manager
}

// findChannelsPath will search for a channels directory, which is helpful when running under bazel
func (v *validator) findChannelsPath() {
	t := v.T
	// Remove this call from the test error stack frame, it is useless for
	// figuring out what test failed.
	t.Helper()

	if _, err := os.Stat(loaders.FlagChannel); err == nil {
		t.Logf("found channels in %v", loaders.FlagChannel)
		return
	}

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("error getting wd: %v", err)
	}
	t.Logf("cwd = %s", cwd)

	p, err := filepath.Abs(loaders.FlagChannel)
	if err != nil {
		t.Fatalf("error determining absolute channel path: %v", err)
	}

	// Strip the "channels" suffix
	p = filepath.Dir(p)

	// We walk "up" the directory tree, looking for a channels
	// subdirectory in each parent directory in the hierarchy of
	// the cwd.  This means we find the channels subdirectory for
	// our operator, even when we're running tests from a
	// subdirectory.
	n := 0
	for {
		n++
		if n > 100 {
			// Sanity check to prevent infinite recursion
			t.Errorf("stuck in loop looking for channels directory")
			break
		}

		c := filepath.Join(p, "channels")
		_, err := os.Stat(c)
		if os.IsNotExist(err) {
			// Expected - look to parent dir
			if p == filepath.Dir(p) {
				// We have hit the root
				t.Logf("unable to find channel directory")
				break
			} else {
				p = filepath.Dir(p)
				continue
			}
		} else if err != nil {
			t.Errorf("error finding channel directory: %v", err)
			break
		} else {
			loaders.FlagChannel = c
			t.Logf("found channels in %v", c)
			break
		}
	}
	t.Logf("flagChannel = %s", loaders.FlagChannel)
}

func (v *validator) Manager() *mocks.Manager {
	return &v.mgr
}

func (v *validator) Validate(r declarative.Reconciler) {
	t := v.T
	t.Helper()

	serializer := json.NewSerializer(json.DefaultMetaFactory, v.scheme, v.scheme, false)
	yamlizer := json.NewYAMLSerializer(json.DefaultMetaFactory, v.scheme, v.scheme)

	metadataAccessor := meta.NewAccessor()

	basedir := "tests"
	if v.TestDir != "" {
		basedir = v.TestDir
	}

	files, err := ioutil.ReadDir(basedir)
	if err != nil {
		t.Fatalf("error reading dir %s: %v", basedir, err)
	}

	ctx := context.TODO()

	for _, f := range files {
		p := filepath.Join(basedir, f.Name())
		t.Logf("Filepath: %s", p)
		if f.IsDir() {
			// TODO: support fs trees?
			t.Errorf("unexpected directory in tests directory: %s", p)
			continue
		}

		if strings.HasSuffix(p, "~") {
			// Ignore editor temp files (for sanity)
			t.Logf("ignoring editor temp file %s", p)
			continue
		}

		if !strings.HasSuffix(p, ".in.yaml") {
			if !strings.HasSuffix(p, ".out.yaml") {
				t.Errorf("unexpected file in tests directory: %s", p)
			}
			continue
		}

		b, err := ioutil.ReadFile(p)
		if err != nil {
			t.Errorf("error reading file %s: %v", p, err)
			continue
		}

		objs, err := manifest.ParseObjects(ctx, string(b))
		if err != nil {
			t.Errorf("error parsing file %s: %v", p, err)
			continue
		}

		if len(objs.Items) != 1 {
			t.Errorf("expected exactly one item in %s", p)
			continue
		}

		crJSON, err := objs.Items[0].JSON()
		if err != nil {
			t.Errorf("error converting CR to json in %s: %v", p, err)
			continue
		}

		cr, _, err := serializer.Decode(crJSON, nil, nil)
		if err != nil {
			t.Errorf("error parsing CR in %s: %v", p, err)
			continue
		}

		namespace, err := metadataAccessor.Namespace(cr)
		if err != nil {
			t.Errorf("error getting namespace in %s: %v", p, err)
			continue
		}

		name, err := metadataAccessor.Name(cr)
		if err != nil {
			t.Errorf("error getting name in %s: %v", p, err)
			continue
		}

		nsn := types.NamespacedName{Namespace: namespace, Name: name}

		objects, err := r.BuildDeploymentObjects(ctx, nsn, cr.(declarative.DeclarativeObject))
		if err != nil {
			t.Errorf("error building deployment objects: %v", err)
			continue
		}

		var actualYAML string
		{
			var b bytes.Buffer

			for i, o := range objects.Items {
				if i != 0 {
					b.WriteString("\n---\n\n")
				}
				u := o.UnstructuredObject()
				if err := yamlizer.Encode(u, &b); err != nil {
					t.Fatalf("error encoding to yaml: %v", err)
				}
			}

			actualYAML = b.String()
		}

		expectedPath := strings.Replace(p, ".in.yaml", ".out.yaml", -1)
		var expectedYAML string
		{
			b, err := ioutil.ReadFile(expectedPath)
			if err != nil {
				t.Errorf("error reading file %s: %v", expectedPath, err)
				continue
			}
			expectedYAML = string(b)
		}

		if actualYAML != expectedYAML {
			if os.Getenv("HACK_AUTOFIX_EXPECTED_OUTPUT") != "" {
				t.Logf("HACK_AUTOFIX_EXPECTED_OUTPUT is set; replacing expected output in %s", expectedPath)
				if err := ioutil.WriteFile(expectedPath, []byte(actualYAML), 0644); err != nil {
					t.Fatalf("error writing expected output to %s: %v", expectedPath, err)
				}
				continue
			}

			if err := diffFiles(t, expectedPath, actualYAML); err != nil {
				t.Logf("failed to run system diff, falling back to string diff: %v", err)
				t.Logf("diff: %s", diff.StringDiff(actualYAML, expectedYAML))
			}

			t.Errorf("unexpected diff between actual and expected YAML. See previous output for details.")
			t.Logf(`To regenerate the output based on this result, rerun this test with HACK_AUTOFIX_EXPECTED_OUTPUT="true"`)
		}

	}
}

func diffFiles(t *testing.T, expectedPath, actual string) error {
	t.Helper()
	writeTmp := func(content string) (string, error) {
		tmp, err := ioutil.TempFile("", "*.yaml")
		if err != nil {
			return "", err
		}
		defer func() {
			tmp.Close()
		}()
		if _, err := tmp.Write([]byte(content)); err != nil {
			return "", err
		}
		return tmp.Name(), nil
	}

	actualTmp, err := writeTmp(actual)
	if err != nil {
		return errors.Wrapf(err, "write actual yaml to temp file failed")
	}
	t.Logf("Wrote actual to %s", actualTmp)

	// pls to use unified diffs, kthxbai?
	cmd := exec.Command("diff", "-u", expectedPath, actualTmp)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Wrapf(err, "set up stdout pipe from diff failed")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrapf(err, "start command failed")
	}

	diff, err := ioutil.ReadAll(stdout)
	if err != nil {
		return errors.Wrapf(err, "read from diff stdout failed")
	}

	if err := cmd.Wait(); err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return errors.Wrapf(err, "wait for command to finish failed")
		}
		t.Logf("Diff exited %s", exitErr)
	}

	expectedAbs, err := filepath.Abs(expectedPath)
	if err != nil {
		t.Logf("getting absolute path for %s failed: %s", expectedPath, err)
		expectedAbs = expectedPath
	}

	t.Logf("View diff: meld %s %s", expectedAbs, actualTmp)
	t.Logf("Diff: expected - + actual\n%s", diff)
	return nil
}
