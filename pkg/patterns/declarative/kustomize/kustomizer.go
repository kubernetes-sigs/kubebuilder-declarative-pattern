package kustomize

import (
	"context"

	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// Kustomize will be initialized differently according to the go build constraints.
var Kustomize Kustomizer

// Kustomizer interface defines a `Run` method to differentiate the `kustomize` behavior under
// different go build constraints
type Kustomizer interface {
	Run(context.Context, filesys.FileSystem, string) ([]byte, error)
}
