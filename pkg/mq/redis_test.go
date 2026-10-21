package mq

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestRedisRoundTrip(t *testing.T) {
	env := readEnv(filepath.Join("..", "..", ".env"))
	password := env["REDIS_PASSWORD"]
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379", Password: password})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis is not reachable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	topic := "toMysql-test-" + time.Now().Format("150405.000000")
	stream := streamName("mq", topic)
	t.Cleanup(func() { _ = rdb.Del(context.Background(), stream).Err() })

	producer := NewRedisProducer(rdb, "mq", topic, 100)
	consumer := NewRedisConsumer(rdb, "mq", topic, topic)
	got := make(chan *Message, 1)
	runCtx, stop := context.WithCancel(context.Background())
	defer stop()
	errCh := make(chan error, 1)
	go func() {
		errCh <- consumer.Subscribe(runCtx, func(_ context.Context, msg *Message) error {
			if err := msg.Ack(context.Background()); err != nil {
				return err
			}
			got <- msg
			return nil
		})
	}()

	deadline := time.Now().Add(3 * time.Second)
	var sendErr error
	for {
		sendErr = producer.Send(context.Background(), "si_a_b", []byte("body"))
		if sendErr == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(sendErr)
		}
		time.Sleep(20 * time.Millisecond)
	}

	select {
	case msg := <-got:
		if msg.Key != "si_a_b" || string(msg.Value) != "body" {
			t.Fatalf("message key %s value %q", msg.Key, msg.Value)
		}
	case err := <-errCh:
		t.Fatal(err)
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for redis message")
	}
	stop()
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
