package logic

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/localcache"
	svcconfig "github.com/lisuhang11/SuIM/services/user/internal/config"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

// NewTokenCache builds the process-local token cache. Login and logout publish the
// revoked hashes on Redis so other instances drop the same keys.
func NewTokenCache(ctx context.Context, rdb redis.UniversalClient, cfg svcconfig.LocalCache) (localcache.Cache[*model.Token], func()) {
	if !cfg.Enable() {
		return nil, func() {}
	}
	cache := localcache.New[*model.Token](
		localcache.WithLocalSlotNum(cfg.SlotNum),
		localcache.WithLocalSlotSize(cfg.SlotSize),
		localcache.WithLocalSuccessTTL(time.Duration(cfg.SuccessExpire)*time.Second),
		localcache.WithLocalFailedTTL(time.Duration(cfg.FailedExpire)*time.Second),
		localcache.WithLinkDisable(),
		localcache.WithDeleteKeyBefore(func(ctx context.Context, keys ...string) {
			if len(keys) == 0 {
				return
			}
			if err := rdb.Publish(ctx, cfg.Topic, strings.Join(keys, ",")).Err(); err != nil {
				slog.Error("publish token cache delete", "err", err)
			}
		}),
	)
	subCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		subscribeTokenDeletes(subCtx, rdb, cfg.Topic, cache.DelLocal)
	}()
	return cache, func() {
		cancel()
		<-done
		cache.Stop()
	}
}

func subscribeTokenDeletes(ctx context.Context, rdb redis.UniversalClient, topic string, del func(context.Context, ...string)) {
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
				keys := strings.Split(msg.Payload, ",")
				cleaned := keys[:0]
				for _, key := range keys {
					if key != "" {
						cleaned = append(cleaned, key)
					}
				}
				if len(cleaned) > 0 {
					del(ctx, cleaned...)
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
