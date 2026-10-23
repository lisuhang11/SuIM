package mq

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const claimIdle = 2 * time.Minute

// RedisProducer appends to a Redis stream.
type RedisProducer struct {
	rdb    redis.UniversalClient
	stream string
	maxLen int64
}

func NewRedisProducer(rdb redis.UniversalClient, prefix, topic string, maxLen int64) *RedisProducer {
	return &RedisProducer{rdb: rdb, stream: streamName(prefix, topic), maxLen: maxLen}
}

func (p *RedisProducer) Send(ctx context.Context, key string, value []byte) error {
	args := &redis.XAddArgs{
		Stream: p.stream,
		Values: map[string]any{"key": key, "body": value},
	}
	if p.maxLen > 0 {
		args.MaxLen = p.maxLen
		args.Approx = true
	}
	if err := p.rdb.XAdd(ctx, args).Err(); err != nil {
		return fmt.Errorf("xadd %s: %w", p.stream, err)
	}
	return nil
}

// RedisConsumer reads a Redis stream with a consumer group.
type RedisConsumer struct {
	rdb      redis.UniversalClient
	stream   string
	group    string
	consumer string
	start    string
}

func NewRedisConsumer(rdb redis.UniversalClient, prefix, topic, group string) *RedisConsumer {
	if group == "" {
		group = topic
	}
	host, _ := os.Hostname()
	if host == "" {
		host = "msgtransfer"
	}
	return &RedisConsumer{
		rdb:      rdb,
		stream:   streamName(prefix, topic),
		group:    group,
		consumer: fmt.Sprintf("%s-%d", host, os.Getpid()),
	}
}

// ConsumeNew starts a new consumer group at the end of the stream.
// A restarted process with the same group still receives messages it has not acked.
func (c *RedisConsumer) ConsumeNew() {
	if c != nil && c.start == "" {
		c.start = "$"
	}
}

func (c *RedisConsumer) Subscribe(ctx context.Context, fn func(context.Context, *Message) error) error {
	if err := c.ensureGroup(ctx); err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := c.claim(ctx, fn); err != nil {
			return err
		}
		streams, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group:    c.group,
			Consumer: c.consumer,
			Streams:  []string{c.stream, ">"},
			Count:    32,
			Block:    2 * time.Second,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				continue
			}
			return fmt.Errorf("xreadgroup %s: %w", c.stream, err)
		}
		for _, stream := range streams {
			for i := range stream.Messages {
				if err := c.dispatch(ctx, fn, &stream.Messages[i]); err != nil && ctx.Err() != nil {
					return ctx.Err()
				}
			}
		}
	}
}

func (c *RedisConsumer) ensureGroup(ctx context.Context) error {
	start := c.start
	if start == "" {
		start = "0"
	}
	err := c.rdb.XGroupCreateMkStream(ctx, c.stream, c.group, start).Err()
	if err == nil || strings.Contains(err.Error(), "BUSYGROUP") {
		return nil
	}
	return fmt.Errorf("xgroup create %s: %w", c.stream, err)
}

func (c *RedisConsumer) claim(ctx context.Context, fn func(context.Context, *Message) error) error {
	for {
		msgs, next, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream:   c.stream,
			Group:    c.group,
			Consumer: c.consumer,
			MinIdle:  claimIdle,
			Start:    "0-0",
			Count:    32,
		}).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("xautoclaim %s: %w", c.stream, err)
		}
		for i := range msgs {
			if err := c.dispatch(ctx, fn, &msgs[i]); err != nil && ctx.Err() != nil {
				return ctx.Err()
			}
		}
		if next == "0-0" || len(msgs) == 0 {
			return nil
		}
	}
}

func (c *RedisConsumer) dispatch(ctx context.Context, fn func(context.Context, *Message) error, raw *redis.XMessage) error {
	key, _ := raw.Values["key"].(string)
	msg := &Message{
		Key:   key,
		Value: fieldBytes(raw.Values["body"]),
		ctx:   ctx,
		ack: func(ackCtx context.Context) error {
			return c.rdb.XAck(ackCtx, c.stream, c.group, raw.ID).Err()
		},
	}
	if err := fn(ctx, msg); err != nil {
		slog.Error("redis queue handler", "err", err, "stream", c.stream, "id", raw.ID)
		return err
	}
	return nil
}

func fieldBytes(v any) []byte {
	switch t := v.(type) {
	case string:
		return []byte(t)
	case []byte:
		return t
	default:
		return nil
	}
}

func streamName(prefix, topic string) string {
	prefix = strings.Trim(prefix, ":")
	if prefix == "" {
		prefix = "mq"
	}
	return prefix + ":" + topic
}
