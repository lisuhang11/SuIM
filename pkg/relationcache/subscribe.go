package relationcache

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

func subscribeDeletes(ctx context.Context, rdb redis.UniversalClient, topic string, del func(context.Context, ...string)) {
	if rdb == nil || topic == "" || del == nil {
		return
	}
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
				var keys []string
				if err := json.Unmarshal([]byte(msg.Payload), &keys); err != nil {
					slog.Warn("relation cache delete payload", "err", err)
					continue
				}
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
