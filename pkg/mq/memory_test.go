package mq

import (
	"context"
	"testing"
	"time"
)

func TestMemoryDelivers(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bus := NewMemory()
	got := make(chan *Message, 1)
	go func() {
		_ = bus.Consumer("toMysql").Subscribe(ctx, func(_ context.Context, msg *Message) error {
			got <- msg
			return nil
		})
	}()
	time.Sleep(20 * time.Millisecond)
	if err := bus.Producer("toMysql").Send(ctx, "k", []byte("v")); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-got:
		if msg.Key != "k" || string(msg.Value) != "v" {
			t.Fatalf("message %+v", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}
