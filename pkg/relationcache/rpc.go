package relationcache

import (
	"context"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/localcache"
)

// Friends is the relation RPC used when the local result is missing.
type Friends interface {
	IsFriend(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
	IsBlack(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
}

type pair struct {
	Left  bool
	Right bool
}

// RPC caches IsFriend and IsBlack inside the calling process.
// A result is tied to both users' list keys, so a change on either side drops it.
type RPC struct {
	next    Friends
	friends localcache.Cache[pair]
	blacks  localcache.Cache[pair]
}

// NewRPC subscribes to relation cache deletes and drops linked friend and block results.
func NewRPC(ctx context.Context, rdb redis.UniversalClient, next Friends, cfg Config) (*RPC, func()) {
	cfg = cfg.apply()
	opts := []localcache.Option{
		localcache.WithLocalSlotNum(cfg.SlotNum),
		localcache.WithLocalSlotSize(cfg.SlotSize),
		localcache.WithLinkSlotNum(cfg.SlotNum),
		localcache.WithLocalSuccessTTL(cfg.SuccessExpire),
		localcache.WithLocalFailedTTL(cfg.FailedExpire),
	}
	cache := &RPC{
		next:    next,
		friends: localcache.New[pair](opts...),
		blacks:  localcache.New[pair](opts...),
	}
	subCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		subscribeDeletes(subCtx, rdb, cfg.Topic, cache.dropLocal)
	}()
	return cache, func() {
		cancel()
		<-done
		cache.friends.Stop()
		cache.blacks.Stop()
	}
}

func (c *RPC) dropLocal(ctx context.Context, keys ...string) {
	c.friends.DelLocal(ctx, keys...)
	c.blacks.DelLocal(ctx, keys...)
}

func (c *RPC) IsFriend(ctx context.Context, user1, user2 string) (bool, bool, error) {
	got, err := c.friends.GetLink(ctx, friendPairKey(user1, user2), func(ctx context.Context) (pair, error) {
		left, right, err := c.next.IsFriend(ctx, user1, user2)
		return pair{Left: left, Right: right}, err
	}, FriendIDsKey(user1), FriendIDsKey(user2))
	return got.Left, got.Right, err
}

func (c *RPC) IsBlack(ctx context.Context, user1, user2 string) (bool, bool, error) {
	got, err := c.blacks.GetLink(ctx, blackPairKey(user1, user2), func(ctx context.Context) (pair, error) {
		left, right, err := c.next.IsBlack(ctx, user1, user2)
		return pair{Left: left, Right: right}, err
	}, BlackIDsKey(user1), BlackIDsKey(user2))
	return got.Left, got.Right, err
}

func friendPairKey(user1, user2 string) string { return "is-friend:" + user1 + ":" + user2 }

func blackPairKey(user1, user2 string) string { return "is-black:" + user1 + ":" + user2 }
