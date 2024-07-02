package testharness

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func RunGoldenTests(t *testing.T, basedir string, fn func(h *Harness, dir string)) {
	entries, err := os.ReadDir(basedir)
	if err != nil {
		t.Fatalf("ReadDir(%q) failed: %v", basedir, err)
	}
	files := make([]fs.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatalf("failed to get FileInfo %v: %v", info, err)
		}
		files = append(files, info)
	}
	count := 0
	for _, file := range files {
		name := file.Name()
		absPath := filepath.Join(basedir, name)
		count++
		t.Run(name, func(t *testing.T) {
			h := New(t)
			fn(h, absPath)
		})
	}
	// Likely a typo in basedir (?)
	if count == 0 {
		t.Errorf("no golden tests found in %q", basedir)
	}
}

func (h *Harness) CompareGoldenFile(p string, got string) {
	if os.Getenv("WRITE_GOLDEN_OUTPUT") != "" {
		// Short-circuit when the output is correct
		b, err := os.ReadFile(p)
		if err == nil && bytes.Equal(b, []byte(got)) {
			return
		}

		if err := os.WriteFile(p, []byte(got), 0644); err != nil {
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
