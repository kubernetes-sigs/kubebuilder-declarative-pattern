package testharness

import (
	"os"
	"testing"
)

type Harness struct {
	*testing.T
}

func New(t *testing.T) *Harness {
	h := &Harness{T: t}
	t.Cleanup(h.Cleanup)
	return h
}

func (h *Harness) Cleanup() {

}

func (h *Harness) TempDir() string {
	tmpdir, err := os.MkdirTemp("", "test")
	if err != nil {
		h.Fatalf("failed to make temp directory: %v", err)
	}
	h.T.Cleanup(func() {
		if err := os.RemoveAll(tmpdir); err != nil {
			h.Errorf("error cleaning up temp directory %q: %v", tmpdir, err)
		}
	})
	return tmpdir
}

func (h *Harness) MustReadFile(p string) []byte {
	b, err := os.ReadFile(p)
	if err != nil {
		h.Fatalf("error from ReadFile(%q): %v", p, err)
	}
	return b
}
