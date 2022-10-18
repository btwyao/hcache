package hcache

import (
	"time"

	lru "github.com/hnlq715/golang-lru"
	"github.com/patrickmn/go-cache"
)

type PatrickmnCache struct {
	*cache.Cache
	CacheHandler
}

func (c *PatrickmnCache) Set(k string, v interface{}) {
	c.Cache.SetDefault(k, v)
}

func NewPatrickmnCache(defaultExpiration time.Duration, cleanupInterval time.Duration, config Config) (*PatrickmnCache, error) {
	c := cache.New(defaultExpiration, cleanupInterval)
	wrap := &PatrickmnCache{Cache: c}
	hc, err := NewCacheHandler(wrap, config)
	if err != nil {
		return nil, err
	}
	wrap.CacheHandler = hc
	return wrap, nil
}

type Hnlq715LRUCache struct {
	*lru.Cache
	CacheHandler
}

func (c *Hnlq715LRUCache) Set(k string, v interface{}) {
	c.Cache.Add(k, v)
}

func (c *Hnlq715LRUCache) Get(k string) (interface{}, bool) {
	return c.Cache.Get(k)
}

func NewHnlq715LRUCache(size int, expire time.Duration, config Config) (*Hnlq715LRUCache, error) {
	c, err := lru.NewWithExpire(size, expire)
	if err != nil {
		return nil, err
	}
	wrap := &Hnlq715LRUCache{Cache: c}
	hc, err := NewCacheHandler(wrap, config)
	if err != nil {
		return nil, err
	}
	wrap.CacheHandler = hc
	return wrap, nil
}
