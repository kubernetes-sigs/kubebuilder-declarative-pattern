package applier

import (
	"context"
)

type Applier interface {
	Applyx(ctx context.Context, namespace string, manifest string, validate bool, extraArgs ...string) error
}
