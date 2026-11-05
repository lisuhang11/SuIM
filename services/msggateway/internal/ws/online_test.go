package ws

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/lisuhang11/SuIM/pkg/online"
	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
)

func TestGetUsersOnlineStatusReadsRedis(t *testing.T) {
	rdb := openOnlineRedis(t)
	ctx := context.Background()
	userID := "ol" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = rdb.Del(context.Background(), online.Key(userID)).Err() })
	status := online.New(rdb)
	server := &Server{hub: NewHub(), online: status}
	server.hub.Add(&Client{id: "conn-1", userID: userID, platformID: 1, background: true})
	if _, _, err := status.Set(ctx, userID, []int32{1, 5}, nil); err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetUsersOnlineStatus(ctx, &gatewaypb.GetUsersOnlineStatusReq{UserIds: []string{userID}})
	if err != nil || len(resp.GetUsers()) != 1 {
		t.Fatalf("users %d err=%v", len(resp.GetUsers()), err)
	}
	byPlatform := map[int32]*gatewaypb.Platform{}
	for _, platform := range resp.GetUsers()[0].GetPlatforms() {
		byPlatform[platform.GetPlatformId()] = platform
	}
	local := byPlatform[1]
	remote := byPlatform[5]
	if local == nil || local.GetConnId() != "conn-1" || !local.GetIsBackground() || remote == nil || remote.GetConnId() != "" {
		t.Fatalf("platforms %+v %+v", local, remote)
	}
}

func TestKeepLocalRestoresPlatform(t *testing.T) {
	rdb := openOnlineRedis(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	userID := "ol" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = rdb.Del(context.Background(), online.Key(userID)).Err() })
	status := online.New(rdb)
	server := &Server{hub: NewHub(), online: status}
	server.hub.Add(&Client{userID: userID, platformID: 1})
	before, err := rdb.PubSubNumSub(ctx, online.Channel).Result()
	if err != nil {
		t.Fatal(err)
	}
	go server.WatchOnline(ctx)
	waitOnlineSubscribers(t, ctx, rdb, before[online.Channel])
	if _, _, err := status.Set(ctx, userID, []int32{1}, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := status.Set(ctx, userID, nil, []int32{1}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		got, err := status.Get(ctx, userID)
		if err == nil && len(got) == 1 && got[0] == 1 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("platform was not restored: %v err=%v", got, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitOnlineSubscribers(t *testing.T, ctx context.Context, rdb redis.UniversalClient, before int64) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		n, err := rdb.PubSubNumSub(ctx, online.Channel).Result()
		if err == nil && n[online.Channel] > before {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("subscribers %v err=%v", n, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func openOnlineRedis(t *testing.T) redis.UniversalClient {
	t.Helper()
	env := readOnlineEnv(t, "../../../../.env")
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

func readOnlineEnv(t *testing.T, path string) map[string]string {
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
