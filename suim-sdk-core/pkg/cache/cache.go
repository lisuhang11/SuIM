package cache

import "sync"

func NewCache[K comparable, V any]() *Cache[K, V] {
	return &Cache[K, V]{}
}

// Cache is a generic sync.Map wrapper (copied from openim-sdk-core).
type Cache[K comparable, V any] struct {
	m sync.Map
}

func (c *Cache[K, V]) Load(key K) (value V, ok bool) {
	rawValue, ok := c.m.Load(key)
	if !ok {
		return
	}
	return rawValue.(V), ok
}

func (c *Cache[K, V]) Store(key K, value V) {
	c.m.Store(key, value)
}

func (c *Cache[K, V]) Delete(key K) {
	c.m.Delete(key)
}

func (c *Cache[K, V]) DeleteAll() {
	c.m.Range(func(key, _ any) bool {
		c.m.Delete(key)
		return true
	})
}
