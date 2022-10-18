package hcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Config 缓存配置
type Config struct {
	// timeout for the handle func in GetOrSetWithHandle
	HandleTimeoutWindow time.Duration
	HandleConcurrent    int
}

type Cache interface {
	Get(k string) (interface{}, bool)
	Set(k string, v interface{})
}

type CacheHandler interface {
	GetOrSetWithHandle(ctx context.Context, k string,
		handle func(ctx context.Context) (interface{}, error)) (interface{}, error)
}

type keyLockItem struct {
	count   int
	mHandle sync.Mutex
}

type cacheHandler struct {
	cache      Cache
	config     Config
	handleCh   chan interface{}
	keyLockMap map[string]*keyLockItem
	mKeyLock   sync.Mutex
}

func (c *cacheHandler) GetOrSetWithHandle(ctx context.Context, k string,
	handle func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	data, ok := c.cache.Get(k)
	if ok {
		return data, nil
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.HandleTimeoutWindow)
	defer cancel()
	if c.config.HandleConcurrent > 0 {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout")
		case c.handleCh <- nil:
		}
		defer func() { <-c.handleCh }()
	}
	c.mKeyLock.Lock()
	if _, ok := c.keyLockMap[k]; !ok {
		c.keyLockMap[k] = &keyLockItem{}
	}
	keyLock := c.keyLockMap[k]
	keyLock.count++
	c.mKeyLock.Unlock()
	defer func() {
		c.mKeyLock.Lock()
		keyLock.count--
		if keyLock.count <= 0 {
			delete(c.keyLockMap, k)
		}
		c.mKeyLock.Unlock()
	}()
	keyLock.mHandle.Lock()
	defer keyLock.mHandle.Unlock()

	data, ok = c.cache.Get(k)
	if ok {
		return data, nil
	}
	x, err := handle(ctx)
	if err == nil {
		c.cache.Set(k, x)
	}
	return x, err
}

func NewCacheHandler(c Cache, config Config) (CacheHandler, error) {
	if config.HandleTimeoutWindow <= 0 {
		return nil, errors.New("HandleTimeoutWindow need > 0")
	}
	hc := &cacheHandler{
		cache:      c,
		config:     config,
		keyLockMap: make(map[string]*keyLockItem),
	}
	if config.HandleConcurrent > 0 {
		hc.handleCh = make(chan interface{}, config.HandleConcurrent)
	}
	return hc, nil
}
