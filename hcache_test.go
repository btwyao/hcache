package hcache

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type cacheForTest struct {
	kv sync.Map
	CacheHandler
}

func (c *cacheForTest) Get(k string) (interface{}, bool) {
	return c.kv.Load(k)
}

func (c *cacheForTest) Set(k string, v interface{}) {
	c.kv.Store(k, v)
}

func (c *cacheForTest) Delete(k string) {
	c.kv.Delete(k)
}

func (c *cacheForTest) Clean() {
	c.kv = sync.Map{}
}

func newCacheForTest(config Config) (*cacheForTest, error) {
	wrap := &cacheForTest{}
	hc, err := NewCacheHandler(wrap, config)
	if err != nil {
		return nil, err
	}
	wrap.CacheHandler = hc
	return wrap, nil
}

func TestHandleRetSuc(t *testing.T) {
	hc, err := newCacheForTest(Config{HandleTimeoutWindow: time.Second, HandleConcurrent: 100})
	assert.True(t, err == nil, err)

	o, err := hc.GetOrSetWithHandle(context.Background(), "testRetSuc", func(ctx context.Context) (interface{}, error) {
		return 1, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 1, o)

	o, err = hc.GetOrSetWithHandle(context.Background(), "testRetSuc", func(ctx context.Context) (interface{}, error) {
		return 2, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 1, o)

	hc.Delete("testRetSuc")
	o, err = hc.GetOrSetWithHandle(context.Background(), "testRetSuc", func(ctx context.Context) (interface{}, error) {
		return 3, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 3, o)
}

func TestHandleRetTimeout(t *testing.T) {
	hc, err := newCacheForTest(Config{HandleTimeoutWindow: time.Second, HandleConcurrent: 100})
	assert.True(t, err == nil, err)

	start := time.Now()
	o, err := hc.GetOrSetWithHandle(context.Background(), "testRetTimeout", func(ctx context.Context) (interface{}, error) {
		tick := time.NewTicker(2 * time.Second)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout")
		case <-tick.C:
			return 1, nil
		}
	})
	assert.True(t, time.Since(start) >= time.Second, time.Since(start))
	assert.True(t, err != nil && err.Error() == "timeout", err)
	assert.True(t, o == nil, o)

	o, err = hc.GetOrSetWithHandle(context.Background(), "testRetTimeout", func(ctx context.Context) (interface{}, error) {
		return 2, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 2, o)
}

func TestHandleConcurrency(t *testing.T) {
	hc, err := newCacheForTest(Config{HandleTimeoutWindow: time.Second, HandleConcurrent: 1000})
	assert.True(t, err == nil, err)

	g := sync.WaitGroup{}
	gCount := 1000
	g.Add(gCount)
	var handleCnt int32
	for i := 0; i < gCount; i++ {
		key := fmt.Sprintf("key%d", i%10)
		go func() {
			defer g.Done()
			for j := 0; j < 100; j++ {
				o, err := hc.GetOrSetWithHandle(context.Background(), key, func(ctx context.Context) (interface{}, error) {
					atomic.AddInt32(&handleCnt, 1)
					tick := time.NewTicker(10 * time.Millisecond)
					select {
					case <-ctx.Done():
						return nil, fmt.Errorf("timeout")
					case <-tick.C:
						return 1, nil
					}
				})
				assert.True(t, err == nil, err)
				assert.True(t, o.(int) == 1, o)
			}
		}()
	}
	g.Wait()
	assert.True(t, handleCnt == 10, handleCnt)
	assert.True(t, len(hc.CacheHandler.(*cacheHandler).handleCh) == 0, len(hc.CacheHandler.(*cacheHandler).handleCh))
	assert.True(t, len(hc.CacheHandler.(*cacheHandler).keyLockMap) == 0, len(hc.CacheHandler.(*cacheHandler).keyLockMap))

	g.Add(gCount)
	hc.Clean()
	for i := 0; i < gCount; i++ {
		key := fmt.Sprintf("key%d", i%10)
		go func() {
			defer g.Done()
			for j := 0; j < 100; j++ {
				deleteItemPercent := rand.Intn(2)
				if deleteItemPercent < 1 {
					hc.Delete(key)
				}
				o, err := hc.GetOrSetWithHandle(context.Background(), key, func(ctx context.Context) (interface{}, error) {
					tick := time.NewTicker(10 * time.Millisecond)
					select {
					case <-ctx.Done():
						return nil, fmt.Errorf("timeout")
					case <-tick.C:
						return key, nil
					}
				})
				assert.True(t, err == nil, err)
				assert.True(t, o.(string) == key, o)
			}
		}()
	}
	g.Wait()
}
