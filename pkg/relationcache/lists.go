package relationcache

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/localcache"
	"github.com/lisuhang11/SuIM/pkg/localcache/lru"
)

// Lists is the relation process cache. A hit is served from this process,
// then from Redis. A miss reads MySQL through fetch and fills both layers.
// Friend requests are not cached.
type Lists struct {
	rdb       redis.UniversalClient
	topic     string
	friends   localcache.Cache[[]string]
	blacks    localcache.Cache[[]string]
	versions  localcache.Cache[Version]
	localHit  atomic.Uint64
	localMiss atomic.Uint64
	redisHit  atomic.Uint64
	mysqlLoad atomic.Uint64
}

// Stats counts cache lookups since the last reset.
// One friend or block check looks up two users, so lookups are not RPC calls.
// A local hit does not touch Redis. A local miss is either a Redis hit or a MySQL load.
type Stats struct {
	LocalHit  uint64
	LocalMiss uint64
	RedisHit  uint64
	MySQLLoad uint64
}

func (l *Lists) Stats() Stats {
	return Stats{
		LocalHit:  l.localHit.Load(),
		LocalMiss: l.localMiss.Load(),
		RedisHit:  l.redisHit.Load(),
		MySQLLoad: l.mysqlLoad.Load(),
	}
}

func (l *Lists) ResetStats() {
	l.localHit.Store(0)
	l.localMiss.Store(0)
	l.redisHit.Store(0)
	l.mysqlLoad.Store(0)
}

// NewLists starts the local caches and subscribes to deletes from other instances.
// The returned stop function cancels that subscription.
func NewLists(ctx context.Context, rdb redis.UniversalClient, cfg Config) (*Lists, func()) {
	cfg = cfg.apply()
	lists := &Lists{rdb: rdb, topic: cfg.Topic}
	opts := []localcache.Option{
		localcache.WithLocalSlotNum(cfg.SlotNum),
		localcache.WithLocalSlotSize(cfg.SlotSize),
		localcache.WithLinkDisable(),
		localcache.WithLocalSuccessTTL(cfg.SuccessExpire),
		localcache.WithLocalFailedTTL(cfg.FailedExpire),
		localcache.WithTarget(statsTarget{hit: &lists.localHit, miss: &lists.localMiss}),
	}
	lists.friends = localcache.New[[]string](opts...)
	lists.blacks = localcache.New[[]string](opts...)
	lists.versions = localcache.New[Version](opts...)
	subCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		subscribeDeletes(subCtx, rdb, cfg.Topic, lists.dropLocal)
	}()
	return lists, func() {
		cancel()
		<-done
		lists.friends.Stop()
		lists.blacks.Stop()
		lists.versions.Stop()
	}
}

func (l *Lists) dropLocal(ctx context.Context, keys ...string) {
	l.friends.DelLocal(ctx, keys...)
	l.blacks.DelLocal(ctx, keys...)
	l.versions.DelLocal(ctx, keys...)
}

// FriendIDs returns the owner's ordered friend IDs.
func (l *Lists) FriendIDs(ctx context.Context, owner string, fetch func(context.Context) ([]string, error)) ([]string, error) {
	ids, err := l.friends.Get(ctx, FriendIDsKey(owner), func(ctx context.Context) ([]string, error) {
		return l.cachedIDs(ctx, FriendIDsKey(owner), fetch)
	})
	return copyIDs(ids), err
}

// BlackIDs returns the owner's block list.
func (l *Lists) BlackIDs(ctx context.Context, owner string, fetch func(context.Context) ([]string, error)) ([]string, error) {
	ids, err := l.blacks.Get(ctx, BlackIDsKey(owner), func(ctx context.Context) ([]string, error) {
		return l.cachedIDs(ctx, BlackIDsKey(owner), fetch)
	})
	return copyIDs(ids), err
}

// Version returns the owner's friend version head.
func (l *Lists) Version(ctx context.Context, owner string, fetch func(context.Context) (Version, error)) (Version, error) {
	return l.versions.Get(ctx, VersionKey(owner), func(ctx context.Context) (Version, error) {
		if l.rdb != nil {
			head, ok, err := loadVersion(ctx, l.rdb, VersionKey(owner))
			if err != nil {
				slog.Warn("read relation version cache", "err", err)
			} else if ok {
				l.redisHit.Add(1)
				return head, nil
			}
		}
		l.mysqlLoad.Add(1)
		head, err := fetch(ctx)
		if err != nil {
			return Version{}, err
		}
		if l.rdb != nil {
			if err := storeVersion(ctx, l.rdb, VersionKey(owner), head); err != nil {
				slog.Warn("write relation version cache", "err", err)
			}
		}
		return head, nil
	})
}

func (l *Lists) cachedIDs(ctx context.Context, key string, fetch func(context.Context) ([]string, error)) ([]string, error) {
	if l.rdb != nil {
		ids, ok, err := loadIDs(ctx, l.rdb, key)
		if err != nil {
			slog.Warn("read relation id cache", "err", err)
		} else if ok {
			l.redisHit.Add(1)
			return ids, nil
		}
	}
	l.mysqlLoad.Add(1)
	ids, err := fetch(ctx)
	if err != nil {
		return nil, err
	}
	if ids == nil {
		ids = []string{}
	}
	if l.rdb != nil {
		if err := storeIDs(ctx, l.rdb, key, ids); err != nil {
			slog.Warn("write relation id cache", "err", err)
		}
	}
	return ids, nil
}

// DropFriends removes friend ID lists and version heads for the owners.
func (l *Lists) DropFriends(ctx context.Context, owners ...string) {
	keys := make([]string, 0, len(owners)*2)
	for _, owner := range owners {
		if owner == "" {
			continue
		}
		keys = append(keys, FriendIDsKey(owner), VersionKey(owner))
	}
	l.drop(ctx, keys...)
}

// DropBlacks removes block lists for the owners.
func (l *Lists) DropBlacks(ctx context.Context, owners ...string) {
	keys := make([]string, 0, len(owners))
	for _, owner := range owners {
		if owner == "" {
			continue
		}
		keys = append(keys, BlackIDsKey(owner))
	}
	l.drop(ctx, keys...)
}

func (l *Lists) drop(ctx context.Context, keys ...string) {
	if len(keys) == 0 {
		return
	}
	deleteKeys(ctx, l.rdb, l.topic, keys)
	l.dropLocal(ctx, keys...)
}

type statsTarget struct {
	hit  *atomic.Uint64
	miss *atomic.Uint64
}

func (s statsTarget) IncrGetHit()     { s.hit.Add(1) }
func (s statsTarget) IncrGetSuccess() { s.miss.Add(1) }
func (s statsTarget) IncrGetFailed()  { s.miss.Add(1) }
func (statsTarget) IncrDelHit()       {}
func (statsTarget) IncrDelNotFound()  {}

var _ lru.Target = statsTarget{}

func copyIDs(ids []string) []string {
	if len(ids) == 0 {
		return []string{}
	}
	return append([]string(nil), ids...)
}
