package remotetargethook

import (
	"context"
	"fmt"

	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/target"
)

type RemoteTargetHook struct {
	resolver target.RemoteTargetResolver
	cache    *target.Cache
}

var _ declarative.BeforeApply = &RemoteTargetHook{}

func (h *RemoteTargetHook) BeforeApply(ctx context.Context, op *declarative.ApplyOperation) error {
	targetClusterKey, replace, err := h.resolver.ResolveKey(ctx, op.Subject)
	if err != nil {
		return fmt.Errorf("error resolving target cluster identifier: %w", err)
	}

	if !replace {
		return nil
	}

	fn := func(ctx context.Context) (*target.RESTInfo, error) {
		return h.resolver.Resolve(ctx, op.Subject, targetClusterKey)
	}
	target, err := h.cache.Get(ctx, targetClusterKey, fn)
	if err != nil {
		return fmt.Errorf("error resolving target cluster: %w", err)
	}
	op.ApplierOptions.RESTConfig = target.RESTConfig()
	op.ApplierOptions.RESTMapper = target.RESTMapper()
	op.RemoteTarget = target

	return nil
}
