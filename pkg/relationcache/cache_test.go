package relationcache

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type friendStub struct {
	friends atomic.Int32
	blacks  atomic.Int32
}

func (s *friendStub) IsFriend(context.Context, string, string) (bool, bool, error) {
	s.friends.Add(1)
	return true, false, nil
}

func (s *friendStub) IsBlack(context.Context, string, string) (bool, bool, error) {
	s.blacks.Add(1)
	return false, true, nil
}

func TestRPCDropsWhenEitherListChanges(t *testing.T) {
	ctx := context.Background()
	stub := &friendStub{}
	cache, stop := NewRPC(ctx, nil, stub, DefaultConfig())
	defer stop()

	if left, right, err := cache.IsFriend(ctx, "a", "b"); err != nil || !left || right {
		t.Fatalf("friend %v %v err=%v", left, right, err)
	}
	if _, _, err := cache.IsFriend(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if stub.friends.Load() != 1 {
		t.Fatalf("friend calls %d", stub.friends.Load())
	}
	cache.dropLocal(ctx, FriendIDsKey("b"))
	if _, _, err := cache.IsFriend(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if stub.friends.Load() != 2 {
		t.Fatalf("friend calls after peer drop %d", stub.friends.Load())
	}
	cache.dropLocal(ctx, FriendIDsKey("a"))
	if _, _, err := cache.IsFriend(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if stub.friends.Load() != 3 {
		t.Fatalf("friend calls after own drop %d", stub.friends.Load())
	}

	if left, right, err := cache.IsBlack(ctx, "a", "b"); err != nil || left || !right {
		t.Fatalf("black %v %v err=%v", left, right, err)
	}
	cache.dropLocal(ctx, BlackIDsKey("a"))
	if _, _, err := cache.IsBlack(ctx, "a", "b"); err != nil {
		t.Fatal(err)
	}
	if stub.blacks.Load() != 2 {
		t.Fatalf("black calls %d", stub.blacks.Load())
	}
}

func TestListsReloadAfterDropAndPublish(t *testing.T) {
	rdb := testRedis(t)
	ctx := context.Background()
	owner := "cache-owner-" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), FriendIDsKey(owner), VersionKey(owner)).Err()
	})

	local, stopLocal := NewLists(ctx, rdb, DefaultConfig())
	defer stopLocal()
	other, stopOther := NewLists(ctx, rdb, DefaultConfig())
	defer stopOther()
	waitSubscribers(t, rdb, Topic, 2)

	var localFetches, otherFetches atomic.Int32
	localFetch := func(context.Context) ([]string, error) {
		localFetches.Add(1)
		return []string{"friend"}, nil
	}
	otherFetch := func(context.Context) ([]string, error) {
		otherFetches.Add(1)
		return []string{"friend"}, nil
	}
	if ids, err := local.FriendIDs(ctx, owner, localFetch); err != nil || len(ids) != 1 {
		t.Fatalf("local ids %v err=%v", ids, err)
	}
	if ids, err := other.FriendIDs(ctx, owner, otherFetch); err != nil || len(ids) != 1 || otherFetches.Load() != 0 {
		t.Fatalf("other ids %v fetches=%d err=%v", ids, otherFetches.Load(), err)
	}
	if _, err := local.FriendIDs(ctx, owner, localFetch); err != nil {
		t.Fatal(err)
	}
	if localFetches.Load() != 1 {
		t.Fatalf("local fetches %d", localFetches.Load())
	}

	local.DropFriends(ctx, owner)
	if n, err := rdb.Exists(ctx, FriendIDsKey(owner)).Result(); err != nil || n != 0 {
		t.Fatalf("redis key remains n=%d err=%v", n, err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for otherFetches.Load() == 0 && time.Now().Before(deadline) {
		if _, err := other.FriendIDs(ctx, owner, otherFetch); err != nil {
			t.Fatal(err)
		}
		if otherFetches.Load() == 0 {
			time.Sleep(20 * time.Millisecond)
		}
	}
	if otherFetches.Load() != 1 {
		t.Fatalf("other instance did not drop the list, fetches=%d", otherFetches.Load())
	}
}

func waitSubscribers(t *testing.T, rdb *redis.Client, topic string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		n, err := rdb.PubSubNumSub(context.Background(), topic).Result()
		if err == nil && int(n[topic]) >= want {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("cache subscribers did not start")
}

func testRedis(t *testing.T) *redis.Client {
	t.Helper()
	env := readEnv(t, filepath.Join("..", "..", ".env"))
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: env["REDIS_PASSWORD"]})
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis is not reachable: %v", err)
	}
	return rdb
}

func readEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	env := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok {
			env[key] = value
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return env
}
