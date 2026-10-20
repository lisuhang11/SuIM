package localcache

import (
	"context"
	"hash/fnv"
	"unsafe"

	"github.com/lisuhang11/SuIM/pkg/localcache/link"
	"github.com/lisuhang11/SuIM/pkg/localcache/lru"
)

type Cache[V any] interface {
	Get(ctx context.Context, key string, fetch func(ctx context.Context) (V, error)) (V, error)
	GetLink(ctx context.Context, key string, fetch func(ctx context.Context) (V, error), link ...string) (V, error)
	Del(ctx context.Context, key ...string)
	DelLocal(ctx context.Context, key ...string)
	Stop()
}

func LRUStringHash(key string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write(unsafe.Slice(unsafe.StringData(key), len(key)))
	return h.Sum64()
}

func New[V any](opts ...Option) Cache[V] {
	opt := defaultOption()
	for _, o := range opts {
		o(opt)
	}

	c := &cache[V]{opt: opt}
	if opt.localSlotNum > 0 && opt.localSlotSize > 0 {
		createSimpleLRU := func() lru.LRU[string, V] {
			if opt.expirationEvict {
				return lru.NewExpirationLRU[string, V](opt.localSlotSize, opt.localSuccessTTL, opt.localFailedTTL, opt.target, c.onEvict)
			}
			return lru.NewLazyLRU[string, V](opt.localSlotSize, opt.localSuccessTTL, opt.localFailedTTL, opt.target, c.onEvict)
		}
		if opt.localSlotNum == 1 {
			c.local = createSimpleLRU()
		} else {
			c.local = lru.NewSlotLRU[string, V](opt.localSlotNum, LRUStringHash, createSimpleLRU)
		}
		if opt.linkSlotNum > 0 {
			c.link = link.New(opt.linkSlotNum)
		}
	}
	return c
}

type cache[V any] struct {
	opt   *option
	link  link.Link
	local lru.LRU[string, V]
}

func (c *cache[V]) onEvict(key string, value V) {
	if c.link == nil {
		return
	}
	// 淘汰回调还持有 LRU 的锁，关联 key 的删除放到别的 goroutine，避免同槽重入死锁。
	if lks := c.link.Del(key); len(lks) > 0 {
		go c.delLinked(key, lks)
	}
}

func (c *cache[V]) delLinked(src string, keys map[string]struct{}) {
	for k := range keys {
		if src != k {
			c.local.Del(k)
		}
	}
}

func (c *cache[V]) del(key ...string) {
	if c.local == nil {
		return
	}
	for _, k := range key {
		c.local.Del(k)
		if c.link == nil {
			continue
		}
		for lk := range c.link.Del(k) {
			c.local.Del(lk)
		}
	}
}

func (c *cache[V]) Get(ctx context.Context, key string, fetch func(ctx context.Context) (V, error)) (V, error) {
	return c.GetLink(ctx, key, fetch)
}

func (c *cache[V]) GetLink(ctx context.Context, key string, fetch func(ctx context.Context) (V, error), links ...string) (V, error) {
	if c.local == nil {
		return fetch(ctx)
	}
	return c.local.Get(key, func() (V, error) {
		if len(links) > 0 && c.link != nil {
			c.link.Link(key, links...)
		}
		return fetch(ctx)
	})
}

func (c *cache[V]) Del(ctx context.Context, key ...string) {
	for _, fn := range c.opt.delFn {
		fn(ctx, key...)
	}
	c.del(key...)
}

func (c *cache[V]) DelLocal(ctx context.Context, key ...string) {
	c.del(key...)
}

func (c *cache[V]) Stop() {
	if c.local != nil {
		c.local.Stop()
	}
}
