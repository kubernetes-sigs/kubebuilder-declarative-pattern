package testharness

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func RunGoldenTests(t *testing.T, basedir string, fn func(h *Harness, dir string)) {
	files, err := ioutil.ReadDir(basedir)
	if err != nil {
		t.Fatalf("ReadDir(%q) failed: %v", basedir, err)
	}
	for _, file := range files {
		name := file.Name()
		absPath := filepath.Join(basedir, name)
		t.Run(name, func(t *testing.T) {
			h := New(t)
			fn(h, absPath)
		})
	}
}

func (h *Harness) CompareGoldenFile(p string, got string) {
	if os.Getenv("WRITE_GOLDEN_OUTPUT") != "" {
		// Short-circuit when the output is correct
		b, err := ioutil.ReadFile(p)
		if err == nil && bytes.Equal(b, []byte(got)) {
			return
		}

		if err := ioutil.WriteFile(p, []byte(got), 0644); err != nil {
			h.Fatalf("failed to write golden output %s: %v", p, err)
		}
		h.Errorf("wrote output to %s", p)
	} else {
		want := string(h.MustReadFile(p))
		if diff := cmp.Diff(want, got); diff != "" {
			h.Errorf("unexpected diff in %s: %s", p, diff)
		}
	}
}
