package target

import (
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Cluster struct {
	clusterKey string
	info       *RESTInfo
}

func (t *Cluster) RESTConfig() *rest.Config {
	return t.info.RESTConfig
}

func (t *Cluster) RESTMapper() meta.RESTMapper {
	return t.info.RESTMapper
}

func (t *Cluster) ClusterKey() string {
	return t.clusterKey
}

func (t *Cluster) DynamicClient() dynamic.Interface {
	return t.info.DynamicClient
}
