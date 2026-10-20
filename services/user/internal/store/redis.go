package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/services/user/internal/model"
)

const (
	snapshotPrefix = "suim:token:"
	denyPrefix     = "suim:token:deny:"
)

type snapshot struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	PlatformID int32     `json:"platform_id"`
	Type       string    `json:"token_type"`
	ExpiresAt  time.Time `json:"expires_at"`
	IsRevoked  bool      `json:"is_revoked"`
}

func NewRedis(cfg config.Redis) (redis.UniversalClient, error) {
	if len(cfg.Address) == 0 {
		return nil, fmt.Errorf("redis address is empty")
	}
	switch cfg.RedisMode {
	case "cluster":
		return redis.NewClusterClient(&redis.ClusterOptions{
			Addrs:      cfg.Address,
			Username:   cfg.Username,
			Password:   cfg.Password,
			MaxRetries: cfg.MaxRetry,
			PoolSize:   cfg.PoolSize,
		}), nil
	case "sentinel":
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:     cfg.SentinelMode.MasterName,
			SentinelAddrs:  cfg.SentinelMode.SentinelAddrs,
			Username:       cfg.Username,
			Password:       cfg.Password,
			DB:             cfg.DB,
			MaxRetries:     cfg.MaxRetry,
			PoolSize:       cfg.PoolSize,
			RouteByLatency: cfg.SentinelMode.RouteByLatency,
			RouteRandomly:  cfg.SentinelMode.RouteRandomly,
		}), nil
	default:
		return redis.NewClient(&redis.Options{
			Addr:       cfg.Address[0],
			Username:   cfg.Username,
			Password:   cfg.Password,
			DB:         cfg.DB,
			MaxRetries: cfg.MaxRetry,
			PoolSize:   cfg.PoolSize,
		}), nil
	}
}

func (s *Store) putSnapshot(ctx context.Context, tok *model.Token) error {
	ttl := time.Until(tok.ExpiresAt)
	if ttl <= 0 {
		return nil
	}
	b, err := json.Marshal(snapshot{
		ID:         tok.ID,
		UserID:     tok.UserID,
		PlatformID: tok.PlatformID,
		Type:       tok.Type,
		ExpiresAt:  tok.ExpiresAt.UTC(),
		IsRevoked:  tok.IsRevoked,
	})
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, snapshotPrefix+tok.Hash, b, ttl).Err()
}

func (s *Store) getSnapshot(ctx context.Context, hash string) (*model.Token, error) {
	raw, err := s.rdb.Get(ctx, snapshotPrefix+hash).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var snap snapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return nil, err
	}
	return &model.Token{
		ID:         snap.ID,
		UserID:     snap.UserID,
		PlatformID: snap.PlatformID,
		Hash:       hash,
		Type:       snap.Type,
		ExpiresAt:  snap.ExpiresAt,
		IsRevoked:  snap.IsRevoked,
	}, nil
}

func (s *Store) dropSnapshots(ctx context.Context, tokens []model.Token) error {
	if len(tokens) == 0 {
		return nil
	}
	keys := make([]string, 0, len(tokens))
	pipe := s.rdb.Pipeline()
	for _, tok := range tokens {
		keys = append(keys, snapshotPrefix+tok.Hash)
		ttl := time.Until(tok.ExpiresAt)
		if ttl <= 0 {
			ttl = time.Minute
		}
		pipe.Set(ctx, denyPrefix+tok.Hash, "1", ttl)
	}
	pipe.Del(ctx, keys...)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *Store) deny(ctx context.Context, tok *model.Token) error {
	ttl := time.Until(tok.ExpiresAt)
	if ttl <= 0 {
		ttl = time.Minute
	}
	return s.rdb.Set(ctx, denyPrefix+tok.Hash, "1", ttl).Err()
}

func (s *Store) denied(ctx context.Context, hash string) (bool, error) {
	n, err := s.rdb.Exists(ctx, denyPrefix+hash).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
