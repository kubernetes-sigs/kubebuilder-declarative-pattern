package cluster

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	controllerrestmapper "sigs.k8s.io/kubebuilder-declarative-pattern/pkg/restmapper"
)

var _ Cluster = &RemoteCluster{}

type RemoteCluster struct {
	RestConfig *rest.Config
	restMapper meta.RESTMapper
	Dynamic    dynamic.Interface
	Client     client.Client
	name       string
}

func (c *RemoteCluster) GetClient() client.Client {
	return c.Client
}

func (c *RemoteCluster) GetName() string {
	return c.name
}

func (r *RemoteCluster) GetConfig(context.Context) *rest.Config {
	return r.RestConfig
}

func (r *RemoteCluster) GetRestMapper() meta.RESTMapper {
	return r.restMapper
}

func (r *RemoteCluster) GetDynamicClient() dynamic.Interface {
	return r.Dynamic
}

// NewRemote - sets RemoteCluster object
func NewRemote(ctx context.Context, name string, restConfig *rest.Config) *RemoteCluster {
	log := log.FromContext(ctx)
	if restConfig == nil {
		return nil
	}
	c := &RemoteCluster{
		name: name,
	}
	var err error
	c.Client, err = client.New(restConfig, client.Options{})
	if err != nil {
		log.Error(err, "unable to get client")
		return nil
	}

	restMapper, err := controllerrestmapper.NewControllerRESTMapper(restConfig)
	if err != nil {
		log.Error(err, "unable to get restmapper")
		return nil
	}
	c.restMapper = restMapper

	c.Dynamic, err = dynamic.NewForConfig(restConfig)
	if err != nil {
		log.Error(err, "unable to get dynamicClient")
		return nil
	}
	c.RestConfig = restConfig
	return c
}
