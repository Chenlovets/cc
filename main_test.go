package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLRUCache_Capacity(t *testing.T) {
	tests := []struct {
		name    string
		cap     int
		ops     func(c *LRUCache)
		present []string
		absent  []string
	}{
		{
			name: "evicts LRU when full",
			cap:  3,
			ops: func(c *LRUCache) {
				c.Put("a", "1", 0)
				c.Put("b", "2", 0)
				c.Put("c", "3", 0)
				c.Get("a") // a becomes most recent, b is LRU
				c.Put("d", "4", 0)
			},
			present: []string{"a", "c", "d"},
			absent:  []string{"b"},
		},
		{
			name: "capacity 1",
			cap:  1,
			ops: func(c *LRUCache) {
				c.Put("a", "1", 0)
				c.Put("b", "2", 0)
			},
			present: []string{"b"},
			absent:  []string{"a"},
		},
		{
			name: "update existing key does not evict",
			cap:  2,
			ops: func(c *LRUCache) {
				c.Put("a", "1", 0)
				c.Put("b", "2", 0)
				c.Put("a", "3", 0)
			},
			present: []string{"a", "b"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewLRUCache(tt.cap)
			tt.ops(c)
			for _, k := range tt.present {
				_, ok := c.Get(k)
				assert.True(t, ok, "key %q should be present", k)
			}
			for _, k := range tt.absent {
				_, ok := c.Get(k)
				assert.False(t, ok, "key %q should be absent", k)
			}
		})
	}
}

func TestLRUCache_GetMissing(t *testing.T) {
	v, ok := NewLRUCache(2).Get("nope")
	assert.False(t, ok)
	assert.Equal(t, "", v)
}

func TestLRUCache_PutOverwriteValue(t *testing.T) {
	c := NewLRUCache(2)
	c.Put("k", "old", 0)
	c.Put("k", "new", 0)
	v, ok := c.Get("k")
	assert.True(t, ok)
	assert.Equal(t, "new", v)
}

func TestLRUCache_TTL(t *testing.T) {
	c := NewLRUCache(4)
	c.Put("never", "1", 0)
	c.Put("neg", "2", -1) // ttl <= 0 means never expires
	c.Put("short", "3", 1)
	c.Put("long", "4", 10)

	time.Sleep(1100 * time.Millisecond)

	_, ok := c.Get("short")
	assert.False(t, ok, "expired key should be gone")
	assert.NotContains(t, c.m, "short", "expired key should be removed on Get")
	for _, k := range []string{"never", "neg", "long"} {
		_, ok := c.Get(k)
		assert.True(t, ok, "key %q should still be present", k)
	}
}

func TestLRUCache_EvictExpiredBeforeLRU(t *testing.T) {
	c := NewLRUCache(3)
	c.Put("a", "1", 1) // will expire
	c.Put("b", "2", 0) // LRU tail among non-expired
	c.Put("c", "3", 0)

	time.Sleep(1100 * time.Millisecond)
	c.Put("d", "4", 0) // must evict expired a, not LRU b

	for _, k := range []string{"b", "c", "d"} {
		_, ok := c.Get(k)
		assert.True(t, ok, "key %q should be kept", k)
	}
	_, ok := c.Get("a")
	assert.False(t, ok)
}

func TestLRUCache_PutRefreshesTTL(t *testing.T) {
	c := NewLRUCache(2)
	c.Put("k", "old", 1)
	time.Sleep(600 * time.Millisecond)
	c.Put("k", "new", 10)
	time.Sleep(600 * time.Millisecond) // 1.2s since first Put, past the original ttl

	v, ok := c.Get("k")
	assert.True(t, ok)
	assert.Equal(t, "new", v)
}

func TestLRUCache_PutTTLToNeverExpire(t *testing.T) {
	c := NewLRUCache(2)
	c.Put("k", "v1", 1)
	c.Put("k", "v2", 0) // overwrite with no expiry, heap must be fixed
	time.Sleep(1100 * time.Millisecond)

	v, ok := c.Get("k")
	assert.True(t, ok)
	assert.Equal(t, "v2", v)
}
