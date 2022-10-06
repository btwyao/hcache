package hcache

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

/*
缓存实现规则：
每个key在调用handle函数获取最新值后保存下来，有效时间为RefreshWindow,过了有效期后就要重新调用handle更新
handle函数执行时间有最大限制HandleTimeoutWindow
每个key在同一时间只会有一个handle函数在执行
某个key的handle函数在返回err后可以设置在HandleIntervalWindow之后再执行第二次调用
handle函数返回err时GetOrSetWithHandle接口返回obj（如果缓存中有的话）和err
*/

// Config 缓存配置
type Config struct {
	// Time after which entry should set again
	RefreshWindow time.Duration
	// Interval between GetOrSetWithHandle for the same key when the handle return err
	HandleIntervalWindow time.Duration
	// timeout for the handle func in GetOrSetWithHandle
	HandleTimeoutWindow time.Duration
	// Time after which entry can be removed, if set, it's need > RefreshWindow+config.HandleIntervalWindow+config.HandleTimeoutWindow
	ExpireWindow time.Duration
	// Interval between removing entries (clean up).
	// If set to <= 0 then no action is performed. Setting to < 1 second is counterproductive — it has a one second resolution.
	CleanWindow time.Duration
}

// CacheItem 缓存项
type HCacheItem struct {
	obj      interface{}
	tRefresh int64
	mHandle  sync.Mutex
}

// HCache 缓存
type HCache struct {
	config Config
	cache  *cache.Cache
}

func (c *HCache) getWithoutRefresh(k string) (*HCacheItem, bool) {
	data, ok := c.cache.Get(k)
	if ok {
		item := data.(*HCacheItem)
		if time.Now().UnixMilli() <= item.tRefresh {
			return item, true
		}
		return item, false
	}
	return nil, false
}

// GetOrSetWithHandle loadorstore
func (c *HCache) GetOrSetWithHandle(ctx context.Context, k string,
	handle func(ctx context.Context) (interface{}, error)) (interface{}, error) {
	var item *HCacheItem
	for {
		data, ok := c.cache.Get(k)
		if ok {
			item = data.(*HCacheItem)
			break
		}
		c.cache.Add(k, &HCacheItem{}, cache.DefaultExpiration)
	}
	item.mHandle.Lock()
	defer item.mHandle.Unlock()
	if time.Now().UnixMilli() <= item.tRefresh {
		return item.obj, nil
	}
	ctx, cancel := context.WithTimeout(ctx, c.config.HandleTimeoutWindow)
	defer cancel()
	x, err := handle(ctx)
	if err != nil {
		item.tRefresh = time.Now().Add(c.config.HandleIntervalWindow).UnixMilli()
		c.cache.SetDefault(k, item)
		return item.obj, err
	}
	item.obj = x
	item.tRefresh = time.Now().Add(c.config.RefreshWindow).UnixMilli()
	c.cache.SetDefault(k, item)
	return x, nil
}

func NewCache(config Config) (*HCache, error) {
	if config.RefreshWindow <= 0 {
		return nil, errors.New("RefreshWindow need > 0")
	}
	if config.HandleTimeoutWindow <= 0 {
		return nil, errors.New("HandleTimeoutWindow need > 0")
	}
	hCache := &HCache{
		config: config,
	}
	if config.ExpireWindow > 0 &&
		config.ExpireWindow < config.RefreshWindow+config.HandleIntervalWindow+config.HandleTimeoutWindow {
		return nil, errors.New("ExpireWindow need >= RefreshWindow+HandleIntervalWindow+HandleTimeoutWindow")
	}
	hCache.cache = cache.New(config.ExpireWindow, config.CleanWindow)
	return hCache, nil
}
