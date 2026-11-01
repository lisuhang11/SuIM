package store

import (
	"bufio"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestReserveStopsAtTheWindow(t *testing.T) {
	ctx := context.Background()
	rdb := testRedis(t)
	key := "group:limit:test:" + strconvNow()
	t.Cleanup(func() { _ = rdb.Del(context.Background(), key).Err() })
	q := NewQuota(rdb)
	window := func(n int64) Limit {
		return Limit{Key: key, Limit: 100, N: n, TTL: time.Minute}
	}
	if err := q.Reserve(ctx, []Limit{window(60)}); err != nil {
		t.Fatal(err)
	}
	if err := q.Reserve(ctx, []Limit{window(50)}); !errors.Is(err, ErrLimited) {
		t.Fatalf("over: %v", err)
	}
	if err := q.Reserve(ctx, []Limit{window(40)}); err != nil {
		t.Fatal(err)
	}
	if err := q.Reserve(ctx, []Limit{window(1)}); !errors.Is(err, ErrLimited) {
		t.Fatalf("full: %v", err)
	}
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	env := readDotEnv(t, filepath.Join("..", "..", "..", "..", ".env"))
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: env["REDIS_PASSWORD"]})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func strconvNow() string {
	return strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "")
}

func readDotEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
	}
	return out
}
