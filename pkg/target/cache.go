package target

import (
	"context"
	"sync"
)

type Cache struct {
	mutex   sync.Mutex
	targets map[string]*cacheLine
}

type cacheLine struct {
	mutex  sync.Mutex
	target *CachedTarget
}

func (c *Cache) Get(ctx context.Context, clusterKey string, builder func(ctx context.Context) (*RESTInfo, error)) (*CachedTarget, error) {
	c.mutex.Lock()
	line := c.targets[clusterKey]
	if line == nil {
		line = &cacheLine{}
		c.targets[clusterKey] = line
	}
	c.mutex.Unlock()

	return line.Get(ctx, clusterKey, builder)
}

func (c *cacheLine) Get(ctx context.Context, clusterKey string, builder func(ctx context.Context) (*RESTInfo, error)) (*CachedTarget, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.target != nil {
		return c.target, nil
	}
	info, err := builder(ctx)
	if err != nil {
		return nil, err
	}
	target := &CachedTarget{clusterKey: clusterKey, info: info}
	c.target = target
	return target, nil
}
