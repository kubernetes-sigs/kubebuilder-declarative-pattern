//go:build without_kustomize
// +build without_kustomize

package kustomize

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func init() {
	Kustomize = &SkipKustomize{}
}

var _ Kustomizer = &SkipKustomize{}

// NoopKustomize ignore the `kustomization.yaml` file and won't run `kustomize build`.
type SkipKustomize struct{}

// Run is a no-op
func (k *SkipKustomize) Run(ctx context.Context, _ filesys.FileSystem, _ string) ([]byte, error) {
	log := log.FromContext(ctx)
	log.WithValues("kustomizer", "SkipKustomize").Info("skip running kustomize")
	return nil, nil
}
