package store

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/localcache"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// CacheConfig sizes the process-local group cache. Topic is the Redis channel
// other instances use to drop the same keys.
type CacheConfig struct {
	Topic         string
	SlotNum       int
	SlotSize      int
	SuccessExpire time.Duration
	FailedExpire  time.Duration
}

type readCache struct {
	groups    localcache.Cache[model.Group]
	members   localcache.Cache[model.Member]
	memberIDs localcache.Cache[[]string]
}

// NewReadCache caches group info, members, and member ID lists in this process.
// Deletes are published on Redis so other instances drop the same keys.
func NewReadCache(ctx context.Context, rdb redis.UniversalClient, cfg CacheConfig) (*readCache, func()) {
	publish := func(ctx context.Context, keys ...string) {
		if rdb == nil || cfg.Topic == "" || len(keys) == 0 {
			return
		}
		if err := rdb.Publish(ctx, cfg.Topic, strings.Join(keys, ",")).Err(); err != nil {
			slog.Error("publish group cache delete", "err", err)
		}
	}
	opts := []localcache.Option{
		localcache.WithLocalSlotNum(cfg.SlotNum),
		localcache.WithLocalSlotSize(cfg.SlotSize),
		localcache.WithLocalSuccessTTL(cfg.SuccessExpire),
		localcache.WithLocalFailedTTL(cfg.FailedExpire),
		localcache.WithLinkDisable(),
		localcache.WithDeleteKeyBefore(publish),
	}
	cache := &readCache{
		groups:    localcache.New[model.Group](opts...),
		members:   localcache.New[model.Member](opts...),
		memberIDs: localcache.New[[]string](opts...),
	}
	if rdb == nil || cfg.Topic == "" {
		return cache, cache.stop
	}
	subCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		subscribeDeletes(subCtx, rdb, cfg.Topic, func(ctx context.Context, keys ...string) {
			cache.groups.DelLocal(ctx, keys...)
			cache.members.DelLocal(ctx, keys...)
			cache.memberIDs.DelLocal(ctx, keys...)
		})
	}()
	return cache, func() {
		cancel()
		<-done
		cache.stop()
	}
}

func (c *readCache) stop() {
	if c == nil {
		return
	}
	c.groups.Stop()
	c.members.Stop()
	c.memberIDs.Stop()
}

func (c *readCache) dropGroupInfo(ctx context.Context, groupID string) {
	if c == nil || groupID == "" {
		return
	}
	c.groups.Del(ctx, groupKey(groupID))
}

func (c *readCache) dropMembers(ctx context.Context, groupID string, userIDs ...string) {
	if c == nil || groupID == "" {
		return
	}
	c.groups.Del(ctx, groupKey(groupID))
	c.memberIDs.Del(ctx, memberIDsKey(groupID))
	if len(userIDs) == 0 {
		return
	}
	keys := make([]string, 0, len(userIDs))
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		keys = append(keys, memberKey(groupID, userID))
	}
	if len(keys) > 0 {
		c.members.Del(ctx, keys...)
	}
}

func groupKey(groupID string) string { return "group:" + groupID }

func memberKey(groupID, userID string) string { return "member:" + groupID + ":" + userID }

func memberIDsKey(groupID string) string { return "member-ids:" + groupID }

func copyIDs(ids []string) []string {
	if ids == nil {
		return nil
	}
	return append([]string(nil), ids...)
}

func memberUserIDsOf(members []*model.Member) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.UserID)
	}
	return ids
}

func subscribeDeletes(ctx context.Context, rdb redis.UniversalClient, topic string, del func(context.Context, ...string)) {
	for ctx.Err() == nil {
		pubsub := rdb.Subscribe(ctx, topic)
		ch := pubsub.Channel()
		closed := false
		for !closed && ctx.Err() == nil {
			select {
			case <-ctx.Done():
				closed = true
			case msg, ok := <-ch:
				if !ok {
					closed = true
					break
				}
				keys := splitKeys(msg.Payload)
				if len(keys) > 0 {
					del(ctx, keys...)
				}
			}
		}
		_ = pubsub.Close()
		if ctx.Err() != nil {
			return
		}
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func splitKeys(payload string) []string {
	parts := strings.Split(payload, ",")
	keys := parts[:0]
	for _, key := range parts {
		if key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func loadOnce[T any](once *sync.Once, load func() (map[string]T, error)) func() (map[string]T, error) {
	var (
		loaded map[string]T
		err    error
	)
	return func() (map[string]T, error) {
		once.Do(func() {
			loaded, err = load()
		})
		return loaded, err
	}
}
