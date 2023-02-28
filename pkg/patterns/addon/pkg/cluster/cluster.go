package cluster

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Cluster interface {
	GetName() string
	GetConfig(context.Context) *rest.Config
	GetRestMapper() meta.RESTMapper
	GetDynamicClient() dynamic.Interface
	GetClient() client.Client
}
