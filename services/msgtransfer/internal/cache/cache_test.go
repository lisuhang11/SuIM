package cache

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lisuhang11/SuIM/pkg/config"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func TestMallocRestoresStoredMax(t *testing.T) {
	env := readEnv(filepath.Join("..", "..", "..", "..", ".env"))
	rdb, err := NewRedis(config.Redis{
		Address:   []string{"127.0.0.1:6379"},
		Password:  env["REDIS_PASSWORD"],
		RedisMode: "standalone",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	conversationID := "si_cache_" + time.Now().Format("150405.000000")
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(),
			seqKeyPrefix+conversationID,
			clientKey(conversationID, "c1"),
			msgKey(conversationID, 11),
			readKey(conversationID, "a"),
		).Err()
	})

	c := New(rdb, time.Minute, func(context.Context, string) (int64, error) { return 10, nil })
	before, err := c.Malloc(ctx, conversationID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if before != 10 {
		t.Fatalf("before %d", before)
	}
	again, err := c.Malloc(ctx, conversationID, 1)
	if err != nil || again != 12 {
		t.Fatalf("again %d err %v", again, err)
	}
	if err := c.RememberClientSeq(ctx, conversationID, "c1", 11); err != nil {
		t.Fatal(err)
	}
	seq, ok, err := c.ClientSeq(ctx, conversationID, "c1")
	if err != nil || !ok || seq != 11 {
		t.Fatalf("client seq %d ok %v err %v", seq, ok, err)
	}
	msg := &msgpb.MsgData{Seq: 11, SendId: "a", Content: []byte("hi")}
	if err := c.SetMessages(ctx, conversationID, []*msgpb.MsgData{msg}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetUserReadSeq(ctx, conversationID, map[string]int64{"a": 11}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetUserReadSeq(ctx, conversationID, map[string]int64{"a": 4}); err != nil {
		t.Fatal(err)
	}
	got, err := rdb.Get(ctx, readKey(conversationID, "a")).Int64()
	if err != nil || got != 11 {
		t.Fatalf("read seq %d err %v", got, err)
	}
}

func readEnv(path string) map[string]string {
	b, err := os.ReadFile(path)
	if err != nil {
		return map[string]string{}
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
