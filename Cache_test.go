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

func runHandle(t *testing.T, c *HCache) {
	g := sync.WaitGroup{}
	g.Add(100000)
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i%10)
		go func() {
			for j := 0; j < 10; j++ {
				handleKind := rand.Intn(3) // 0: 正常返回，1: 超时，2: 返回异常
				ret := rand.Intn(1000)
				lastItem, _ := c.getWithoutRefresh(key)
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
						if lastItem != nil {
							assert.True(t, lastItem.obj == o, err)
						} else {
							assert.True(t, o == nil, err)
						}
					case 2:
						assert.True(t, err.Error() == "something wrong", err)
						if lastItem != nil {
							assert.True(t, lastItem.obj == o, err)
						} else {
							assert.True(t, o == nil, err)
						}
					}
				}
				tick := time.NewTicker(time.Duration(rand.Float64() * 1.5 * float64(c.config.ExpireWindow)))
				<-tick.C
			}
			g.Done()
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
	})
	runHandle(t, c)
}
