package declarative

import (
	"context"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/applier"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative/pkg/manifest"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/target"
)

// Hook is the base interface implemented by a hook
type Hook interface {
}

// ApplyOperation contains the details of an Apply operation
type ApplyOperation struct {
	// Subject is the object we are reconciling
	Subject DeclarativeObject

	// Objects is the set of objects we are applying
	Objects *manifest.Objects

	// ApplierOptions is the set of options passed to the applier
	ApplierOptions *applier.ApplierOptions

	// Target allows us to apply to a different cluster from the local one
	RemoteTarget *target.CachedTarget
}

// AfterApply is implemented by hooks that want to be called after every apply operation
type AfterApply interface {
	AfterApply(ctx context.Context, op *ApplyOperation) error
}

// BeforeApply is implemented by hooks that want to be called before every apply operation
type BeforeApply interface {
	BeforeApply(ctx context.Context, op *ApplyOperation) error
}
