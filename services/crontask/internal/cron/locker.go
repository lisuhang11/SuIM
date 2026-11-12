package cron

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const lockLeaseTTL = time.Minute

type Locker interface {
	ExecuteWithLock(ctx context.Context, taskName string, task func())
}

type RedisLocker struct {
	client redis.UniversalClient
	script *redis.Script
}

func NewRedisLocker(client redis.UniversalClient) *RedisLocker {
	return &RedisLocker{
		client: client,
		script: redis.NewScript(strings.TrimSpace(`
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
`)),
	}
}

func (e *RedisLocker) key(name string) string {
	return "SUIM_CRON_LOCKED:" + name
}

func (e *RedisLocker) lock(ctx context.Context, name, owner string) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return e.client.SetNX(ctx, e.key(name), owner, lockLeaseTTL).Result()
}

func (e *RedisLocker) unlock(ctx context.Context, name, owner string) error {
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	return e.script.Run(ctx, e.client, []string{e.key(name)}, owner).Err()
}

func (e *RedisLocker) ExecuteWithLock(ctx context.Context, taskName string, task func()) {
	owner, err := newOwner()
	if err != nil {
		slog.Warn("cron lock owner", "task", taskName, "err", err)
		return
	}
	ok, err := e.lock(ctx, taskName, owner)
	if err != nil {
		slog.Warn("cron lock acquire", "task", taskName, "err", err)
		return
	}
	if !ok {
		slog.Debug("cron lock skipped", "task", taskName)
		return
	}
	defer func() {
		if err := e.unlock(ctx, taskName, owner); err != nil {
			slog.Warn("cron lock unlock", "task", taskName, "err", err)
		}
	}()
	task()
}

func newOwner() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
