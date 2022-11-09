package target

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

type Cache struct {
	mutex   sync.Mutex
	targets map[string]*cacheLine
}

const LocalClusterKey = ""

func NewCache(localRESTConfig *rest.Config, localRESTMapper meta.RESTMapper) (*Cache, error) {
	c := &Cache{
		targets: make(map[string]*cacheLine),
	}
	localDynamicClient, err := dynamic.NewForConfig(localRESTConfig)
	if err != nil {
		return nil, fmt.Errorf("error building dynamic client: %w", err)
	}
	c.targets[LocalClusterKey] = &cacheLine{
		target: &Cluster{
			clusterKey: LocalClusterKey,
			info: &RESTInfo{
				RESTConfig:    localRESTConfig,
				RESTMapper:    localRESTMapper,
				DynamicClient: localDynamicClient,
			},
		},
	}
	return c, nil
}

// LocalCluster returns the default (local) cluster
// This is always populated by the NewCache constructor
func (c *Cache) LocalCluster() *Cluster {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.targets[LocalClusterKey].target
}

type cacheLine struct {
	mutex  sync.Mutex
	target *Cluster
}

func (c *Cache) Get(ctx context.Context, clusterKey string, builder func(ctx context.Context) (*RESTInfo, error)) (*Cluster, error) {
	c.mutex.Lock()
	line := c.targets[clusterKey]
	if line == nil {
		line = &cacheLine{}
		c.targets[clusterKey] = line
	}
	c.mutex.Unlock()

	return line.Get(ctx, clusterKey, builder)
}

func (c *cacheLine) Get(ctx context.Context, clusterKey string, builder func(ctx context.Context) (*RESTInfo, error)) (*Cluster, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.target != nil {
		return c.target, nil
	}
	info, err := builder(ctx)
	if err != nil {
		return nil, err
	}
	target := &Cluster{clusterKey: clusterKey, info: info}
	c.target = target
	return target, nil
}
