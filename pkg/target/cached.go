package target

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type RESTInfo struct {
	RESTConfig *rest.Config
	RESTMapper meta.RESTMapper
}

type RemoteTargetResolver interface {
	ResolveKey(ctx context.Context, subject client.Object) (string, bool, error)
	Resolve(ctx context.Context, subject client.Object, key string) (*RESTInfo, error)
}
