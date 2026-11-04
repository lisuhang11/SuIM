package store

import (
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/config"
)

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
