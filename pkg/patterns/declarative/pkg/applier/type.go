package applier

import (
	"context"
)

type Applier interface {
	Apply(ctx context.Context, namespace string, manifest string, validate bool, extraArgs ...string) error
}
