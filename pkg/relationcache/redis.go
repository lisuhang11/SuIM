package relationcache

import (
	"context"
	"encoding/json"
	"log/slog"
	"math/rand/v2"
	"time"

	"github.com/redis/go-redis/v9"
)

// Version is the friend version head stored in Redis.
type Version struct {
	ID      string `json:"id"`
	Version uint64 `json:"version"`
}

func loadIDs(ctx context.Context, rdb redis.UniversalClient, key string) ([]string, bool, error) {
	raw, err := rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var ids []string
	if err := json.Unmarshal(raw, &ids); err != nil {
		return nil, false, err
	}
	if ids == nil {
		ids = []string{}
	}
	return ids, true, nil
}

func storeIDs(ctx context.Context, rdb redis.UniversalClient, key string, ids []string) error {
	if ids == nil {
		ids = []string{}
	}
	body, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, body, jitterTTL(redisTTL)).Err()
}

func loadVersion(ctx context.Context, rdb redis.UniversalClient, key string) (Version, bool, error) {
	raw, err := rdb.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return Version{}, false, nil
	}
	if err != nil {
		return Version{}, false, err
	}
	var head Version
	if err := json.Unmarshal(raw, &head); err != nil {
		return Version{}, false, err
	}
	return head, true, nil
}

func storeVersion(ctx context.Context, rdb redis.UniversalClient, key string, head Version) error {
	body, err := json.Marshal(head)
	if err != nil {
		return err
	}
	return rdb.Set(ctx, key, body, jitterTTL(redisTTL)).Err()
}

func deleteKeys(ctx context.Context, rdb redis.UniversalClient, topic string, keys []string) {
	if rdb == nil || len(keys) == 0 {
		return
	}
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		slog.Warn("delete relation redis cache", "err", err)
	}
	if topic == "" {
		return
	}
	body, err := json.Marshal(keys)
	if err != nil {
		slog.Warn("encode relation cache delete", "err", err)
		return
	}
	if err := rdb.Publish(ctx, topic, body).Err(); err != nil {
		slog.Warn("publish relation cache delete", "err", err)
	}
}

func jitterTTL(base time.Duration) time.Duration {
	spread := int64(base) / 5
	if spread <= 0 {
		return base
	}
	return base + time.Duration(rand.Int64N(spread*2)-spread)
}
