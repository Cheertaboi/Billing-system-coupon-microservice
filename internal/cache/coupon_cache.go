package cache

import "sync"

type CouponCache struct {
	mu    sync.RWMutex
	store map[string]interface{}
}

func NewCouponCache() *CouponCache {
	return &CouponCache{
		store: make(map[string]interface{}),
	}
}

func (c *CouponCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.store[key]
	return val, ok
}

func (c *CouponCache) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[key] = value
}
