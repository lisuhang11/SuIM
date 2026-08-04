// Package cache 提供 Redis 客户端与群资料旁路缓存助手（由 service 调用）。
package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Options Redis 连接配置。
type Options struct {
	Addr     string
	Password string
	DB       int
}

// NewRedisClient 创建并 Ping Redis；Addr 为空时返回 (nil, nil)，表示禁用缓存。
func NewRedisClient(opts Options) (*redis.Client, error) {
	if opts.Addr == "" {
		return nil, nil
	}
	client := redis.NewClient(&redis.Options{
		Addr:     opts.Addr,
		Password: opts.Password,
		DB:       opts.DB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping %s: %w", opts.Addr, err)
	}
	slog.Info("redis connected", "addr", opts.Addr, "db", opts.DB)
	return client, nil
}
