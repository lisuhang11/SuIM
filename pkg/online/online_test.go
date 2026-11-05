package online

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestParseOnlinePayload(t *testing.T) {
	userID, platformIDs, err := Parse("1:5:u100")
	if err != nil || userID != "u100" || len(platformIDs) != 2 || platformIDs[0] != 1 || platformIDs[1] != 5 {
		t.Fatalf("parsed %s %v %v", userID, platformIDs, err)
	}
	userID, platformIDs, err = Parse("u100")
	if err != nil || userID != "u100" || len(platformIDs) != 0 {
		t.Fatalf("offline %s %v %v", userID, platformIDs, err)
	}
}

func TestOnlineSortedSet(t *testing.T) {
	rdb := openRedis(t)
	ctx := context.Background()
	userID := "ol" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = rdb.Del(context.Background(), Key(userID)).Err() })
	status := New(rdb)

	changed, platformIDs, err := status.Set(ctx, userID, []int32{1, 5}, nil)
	if err != nil || !changed || !samePlatforms(platformIDs, 1, 5) {
		t.Fatalf("set changed=%v platforms=%v err=%v", changed, platformIDs, err)
	}
	ttl, err := rdb.TTL(ctx, Key(userID)).Result()
	if err != nil || ttl <= 0 || ttl > Expire {
		t.Fatalf("ttl %s err=%v", ttl, err)
	}
	changed, _, err = status.Set(ctx, userID, []int32{1}, nil)
	if err != nil || changed {
		t.Fatalf("renew changed=%v err=%v", changed, err)
	}
	if err := rdb.ZAdd(ctx, Key(userID), redis.Z{Score: 1, Member: "9"}).Err(); err != nil {
		t.Fatal(err)
	}
	got, err := status.Get(ctx, userID)
	if err != nil || !samePlatforms(got, 1, 5) {
		t.Fatalf("get %v err=%v", got, err)
	}
	changed, platformIDs, err = status.Set(ctx, userID, nil, []int32{1})
	if err != nil || !changed || !samePlatforms(platformIDs, 5) {
		t.Fatalf("offline changed=%v platforms=%v err=%v", changed, platformIDs, err)
	}
	got, err = status.Get(ctx, userID)
	if err != nil || !samePlatforms(got, 5) {
		t.Fatalf("left %v err=%v", got, err)
	}
	if _, _, err := status.Set(ctx, userID, nil, []int32{5}); err != nil {
		t.Fatal(err)
	}
	n, err := rdb.Exists(ctx, Key(userID)).Result()
	if err != nil || n != 0 {
		t.Fatalf("empty key exists=%d err=%v", n, err)
	}
}

func TestOnlinePublish(t *testing.T) {
	rdb := openRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userID := "ol" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = rdb.Del(context.Background(), Key(userID)).Err() })
	status := New(rdb)
	before, err := rdb.PubSubNumSub(ctx, Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(chan []int32, 1)
	go status.Subscribe(ctx, func(id string, platformIDs []int32) {
		if id == userID {
			select {
			case seen <- append([]int32(nil), platformIDs...):
			default:
			}
		}
	})
	waitSubscribers(t, ctx, rdb, before[Channel])
	if _, _, err := status.Set(ctx, userID, []int32{4}, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case platformIDs := <-seen:
		if !samePlatforms(platformIDs, 4) {
			t.Fatalf("published %v", platformIDs)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("online change was not published")
	}
}

func samePlatforms(got []int32, want ...int32) bool {
	if len(got) != len(want) {
		return false
	}
	seen := map[int32]int{}
	for _, id := range got {
		seen[id]++
	}
	for _, id := range want {
		seen[id]--
	}
	for _, n := range seen {
		if n != 0 {
			return false
		}
	}
	return true
}

func waitSubscribers(t *testing.T, ctx context.Context, rdb redis.UniversalClient, before int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		n, err := rdb.PubSubNumSub(ctx, Channel).Result()
		if err == nil && n[Channel] > before {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscribers %v err=%v", n, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func openRedis(t *testing.T) redis.UniversalClient {
	t.Helper()
	env := readEnv(t, "../../.env")
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: env["REDIS_PASSWORD"]})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		t.Skipf("redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func readEnv(t *testing.T, path string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || !strings.Contains(line, "=") {
			continue
		}
		k, v, _ := strings.Cut(line, "=")
		out[k] = v
	}
	return out
}
