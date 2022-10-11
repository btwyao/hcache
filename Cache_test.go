package hcache

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCacheInit(t *testing.T) {
	_, err := NewCache(Config{})
	assert.True(t, err != nil, err)
	_, err = NewCache(Config{RefreshWindow: time.Second})
	assert.True(t, err != nil, err)
	_, err = NewCache(Config{
		RefreshWindow:       time.Second,
		HandleTimeoutWindow: time.Second,
	})
	assert.True(t, err == nil, err)
	_, err = NewCache(Config{
		RefreshWindow:       time.Second,
		HandleTimeoutWindow: time.Second,
		ExpireWindow:        time.Second,
	})
	assert.True(t, err != nil, err)
	_, err = NewCache(Config{
		RefreshWindow:        time.Second,
		HandleTimeoutWindow:  time.Second,
		HandleIntervalWindow: time.Second,
		ExpireWindow:         3 * time.Second,
	})
	assert.True(t, err == nil, err)
}

func TestHandleRetSuc(t *testing.T) {
	c, _ := NewCache(Config{
		RefreshWindow:        time.Millisecond,
		HandleTimeoutWindow:  time.Millisecond,
		HandleIntervalWindow: time.Millisecond,
		ExpireWindow:         3 * time.Millisecond,
		CleanWindow:          time.Millisecond,
	})
	o, err := c.GetOrSetWithHandle(context.Background(), "testRetSuc", func(ctx context.Context) (interface{}, error) {
		return 1, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 1, o)

	o, err = c.GetOrSetWithHandle(context.Background(), "testRetSuc", func(ctx context.Context) (interface{}, error) {
		return 2, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 1, o)

	tick := time.NewTicker(c.config.RefreshWindow * 2)
	<-tick.C
	o, err = c.GetOrSetWithHandle(context.Background(), "testRetSuc", func(ctx context.Context) (interface{}, error) {
		return 3, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 3, o)

	tick = time.NewTicker(c.config.ExpireWindow * 2)
	<-tick.C
	o, err = c.GetOrSetWithHandle(context.Background(), "testRetSuc", func(ctx context.Context) (interface{}, error) {
		return 4, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 4, o)
}

func TestHandleRetTimeout(t *testing.T) {
	c, _ := NewCache(Config{
		RefreshWindow:        time.Second,
		HandleTimeoutWindow:  time.Second,
		HandleIntervalWindow: time.Second,
		ExpireWindow:         3 * time.Second,
		CleanWindow:          time.Second,
	})
	start := time.Now()
	o, err := c.GetOrSetWithHandle(context.Background(), "testRetTimeout", func(ctx context.Context) (interface{}, error) {
		tick := time.NewTicker(2 * c.config.HandleTimeoutWindow)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout")
		case <-tick.C:
			return 1, nil
		}
	})
	assert.True(t, time.Since(start) >= c.config.HandleTimeoutWindow, time.Since(start))
	assert.True(t, err != nil && err.Error() == "timeout", err)
	assert.True(t, o == nil, o)

	o, err = c.GetOrSetWithHandle(context.Background(), "testRetTimeout", func(ctx context.Context) (interface{}, error) {
		return 2, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o == nil, o)

	tick := time.NewTicker(c.config.HandleIntervalWindow)
	<-tick.C
	o, err = c.GetOrSetWithHandle(context.Background(), "testRetTimeout", func(ctx context.Context) (interface{}, error) {
		return 3, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 3, o)

	tick = time.NewTicker(c.config.RefreshWindow)
	<-tick.C
	o, err = c.GetOrSetWithHandle(context.Background(), "testRetTimeout", func(ctx context.Context) (interface{}, error) {
		tick := time.NewTicker(2 * c.config.HandleTimeoutWindow)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout")
		case <-tick.C:
			return 4, nil
		}
	})
	assert.True(t, err != nil && err.Error() == "timeout", err)
	assert.True(t, o.(int) == 3, o)

	tick = time.NewTicker(c.config.ExpireWindow)
	<-tick.C
	o, err = c.GetOrSetWithHandle(context.Background(), "testRetTimeout", func(ctx context.Context) (interface{}, error) {
		tick := time.NewTicker(2 * c.config.HandleTimeoutWindow)
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout")
		case <-tick.C:
			return 5, nil
		}
	})
	assert.True(t, err != nil && err.Error() == "timeout", err)
	assert.True(t, o == nil, o)
}

func runHandle(t *testing.T, c *HCache) {
	g := sync.WaitGroup{}
	gCount := 1000
	g.Add(gCount)
	for i := 0; i < gCount; i++ {
		key := fmt.Sprintf("key%d", i%10)
		go func() {
			defer g.Done()
			for j := 0; j < 10; j++ {
				handleKind := rand.Intn(3) // 0: 正常返回，1: 超时，2: 返回异常
				ret := rand.Intn(1000)
				inHandle := false
				o, err := c.GetOrSetWithHandle(context.Background(), key, func(ctx context.Context) (interface{}, error) {
					inHandle = true
					switch handleKind {
					case 0:
						return ret, nil
					case 1:
						tick := time.NewTicker(2 * c.config.HandleTimeoutWindow)
						select {
						case <-ctx.Done():
							return nil, fmt.Errorf("timeout")
						case <-tick.C:
							// return ret, nil
							return nil, fmt.Errorf("timeout")
						}
					case 2:
						return nil, fmt.Errorf("something wrong")
					}
					return nil, fmt.Errorf("not be here")
				})
				if inHandle {
					switch handleKind {
					case 0:
						assert.True(t, o.(int) == ret, o)
					case 1:
						assert.True(t, err.Error() == "timeout", err)
					case 2:
						assert.True(t, err.Error() == "something wrong", err)
					}
				}
				tick := time.NewTicker(time.Duration(rand.Float64() * 1.5 * float64(c.config.ExpireWindow)))
				<-tick.C
			}
		}()
	}
	g.Wait()
}

func TestCacheHandle(t *testing.T) {
	c, _ := NewCache(Config{
		RefreshWindow:        time.Millisecond,
		HandleTimeoutWindow:  time.Millisecond,
		HandleIntervalWindow: time.Millisecond,
		ExpireWindow:         3 * time.Millisecond,
		CleanWindow:          time.Millisecond,
	})

	runHandle(t, c)
}
