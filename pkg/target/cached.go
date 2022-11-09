package target

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
)

type RESTInfo struct {
	RESTConfig *rest.Config
	RESTMapper meta.RESTMapper
}
