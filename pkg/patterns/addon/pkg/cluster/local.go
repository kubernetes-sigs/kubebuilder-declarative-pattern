package cluster

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ Cluster = &LocalCluster{}

func NewLocal(mgr controllerruntime.Manager) *LocalCluster {
	return &LocalCluster{Manager: mgr}
}

type LocalCluster struct {
	Manager controllerruntime.Manager
}

func (c *LocalCluster) GetClient() client.Client {
	return c.Manager.GetClient()
}

func (c *LocalCluster) GetName() string {
	return "local"
}

func (c *LocalCluster) GetConfig(context.Context) *rest.Config {
	return c.Manager.GetConfig()
}

func (r *LocalCluster) GetRestMapper() meta.RESTMapper {
	return r.Manager.GetRESTMapper()
}

func (r *LocalCluster) GetDynamicClient() dynamic.Interface {
	return dynamic.NewForConfigOrDie(r.Manager.GetConfig())
}
