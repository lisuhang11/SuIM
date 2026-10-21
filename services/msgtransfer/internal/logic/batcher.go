package logic

import (
	"context"
	"hash/fnv"
	"log/slog"
	"time"

	"github.com/lisuhang11/SuIM/pkg/mq"
)

type batcher struct {
	size     int
	interval time.Duration
	in       []chan *mq.Message
	do       func(context.Context, []*mq.Message) error
}

func newBatcher(size, workers int, interval time.Duration, do func(context.Context, []*mq.Message) error) *batcher {
	if size < 1 {
		size = 1
	}
	if workers < 1 {
		workers = 1
	}
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	in := make([]chan *mq.Message, workers)
	for i := range in {
		in[i] = make(chan *mq.Message, size*2)
	}
	return &batcher{size: size, interval: interval, in: in, do: do}
}

func (b *batcher) Start(ctx context.Context) {
	for i := range b.in {
		go b.run(ctx, b.in[i])
	}
}

func (b *batcher) Put(ctx context.Context, msg *mq.Message) error {
	n := int(hashKey(msg.Key) % uint32(len(b.in)))
	select {
	case b.in[n] <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *batcher) run(ctx context.Context, in <-chan *mq.Message) {
	pending := map[string][]*mq.Message{}
	timer := time.NewTimer(b.interval)
	defer timer.Stop()
	flushKey := func(key string) {
		items := pending[key]
		if len(items) == 0 {
			return
		}
		delete(pending, key)
		b.exec(items[0].Context(), items)
	}
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-in:
			if !ok {
				return
			}
			pending[msg.Key] = append(pending[msg.Key], msg)
			if len(pending[msg.Key]) >= b.size {
				flushKey(msg.Key)
			}
		case <-timer.C:
			for key := range pending {
				flushKey(key)
			}
			timer.Reset(b.interval)
		}
	}
}

func (b *batcher) exec(ctx context.Context, msgs []*mq.Message) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("msgtransfer batch panic", "err", r)
		}
	}()
	if err := b.do(ctx, msgs); err != nil {
		key := ""
		if len(msgs) > 0 {
			key = msgs[0].Key
		}
		slog.Error("msgtransfer batch", "err", err, "key", key, "n", len(msgs))
	}
}

func hashKey(key string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	return h.Sum32()
}
