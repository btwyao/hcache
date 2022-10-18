package hcache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHnlq715LRUCache(t *testing.T) {
	hc, err := NewHnlq715LRUCache(5, time.Second, Config{HandleTimeoutWindow: time.Second, HandleConcurrent: 100})
	assert.True(t, err == nil, err)
	o, err := hc.GetOrSetWithHandle(context.Background(), "test1", func(ctx context.Context) (interface{}, error) {
		return 1, nil
	})
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 1, o)

	for i := 0; i < 6; i++ {
		key := fmt.Sprintf("key%d", i)
		hc.GetOrSetWithHandle(context.Background(), key, func(ctx context.Context) (interface{}, error) {
			return i, nil
		})
	}
	assert.True(t, hc.Contains("key0") == false, hc.Contains("key0"))
	inHandle := false
	o, err = hc.GetOrSetWithHandle(context.Background(), "key0", func(ctx context.Context) (interface{}, error) {
		inHandle = true
		return 8, nil
	})
	assert.True(t, inHandle, inHandle)
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 8, o)

	tick := time.NewTicker(time.Second)
	<-tick.C
	assert.True(t, hc.Contains("key0") == false, hc.Contains("key0"))
	inHandle = false
	o, err = hc.GetOrSetWithHandle(context.Background(), "key0", func(ctx context.Context) (interface{}, error) {
		inHandle = true
		return 9, nil
	})
	assert.True(t, inHandle, inHandle)
	assert.True(t, err == nil, err)
	assert.True(t, o.(int) == 9, o)
}
