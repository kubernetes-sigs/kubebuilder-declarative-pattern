package status

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kubebuilder-declarative-pattern/pkg/patterns/declarative"
)

// NewBasic provides an implementation of declarative.Status that
// performs no preflight checks.
func NewBasic(client client.Client) declarative.Status {
	return &declarative.StatusBuilder{
		ReconciledImpl: NewAggregator(client),
		// no preflight checks
	}
}
