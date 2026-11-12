package redisclient

import (
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/config"
)

func New(cfg config.Redis) (redis.UniversalClient, error) {
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
