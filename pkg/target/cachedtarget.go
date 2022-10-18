package target

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/rest"
)

type CachedTarget struct {
	clusterKey string
	info       *RESTInfo
}

func (t *CachedTarget) RESTConfig() *rest.Config {
	return t.info.RESTConfig
}

func (t *CachedTarget) RESTMapper() meta.RESTMapper {
	return t.info.RESTMapper
}

func (t *CachedTarget) ClusterKey() string {
	return t.clusterKey
}
