package parser

import (
	"context"
	"path/filepath"
	"testing"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/test/testharness"
	"sigs.k8s.io/yaml"
)

func TestReadSimpleCRD(t *testing.T) {
	testharness.RunGoldenTests(t, "testdata", func(h *testharness.Harness, dir string) {
		ctx := context.TODO()
		b := h.MustReadFile(filepath.Join(dir, "input.yaml"))

		node, err := Parse(ctx, b)
		if err != nil {
			t.Errorf("error from Parse: %v", err)
		}

		schemas, err := buildOpenAPISchema(ctx, node)
		if err != nil {
			t.Errorf("error from buildOpenAPISchema: %v", err)
		}

		schemasYAML, err := yaml.Marshal(schemas)
		if err != nil {
			t.Errorf("error building yaml: %v", err)
		}
		h.CompareGoldenFile(filepath.Join(dir, "expected.yaml"), string(schemasYAML))
	})

}
