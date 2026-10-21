package mq

import (
	"context"
	"log/slog"
	"sync"
)

// Memory is an in-process queue used by tests and single-process runs.
type Memory struct {
	mu    sync.Mutex
	pipes map[string]*pipe
}

func NewMemory() *Memory {
	return &Memory{pipes: map[string]*pipe{}}
}

func (m *Memory) Producer(topic string) Producer {
	return &memoryProducer{pipe: m.pipe(topic)}
}

func (m *Memory) Consumer(topic string) Consumer {
	return &memoryConsumer{pipe: m.pipe(topic)}
}

func (m *Memory) pipe(topic string) *pipe {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := m.pipes[topic]
	if p == nil {
		p = &pipe{}
		m.pipes[topic] = p
	}
	return p
}

type pipe struct {
	mu   sync.Mutex
	subs []chan *Message
	buf  []*Message
}

func (p *pipe) send(m *Message) {
	p.mu.Lock()
	if len(p.subs) == 0 {
		p.buf = append(p.buf, m)
		p.mu.Unlock()
		return
	}
	subs := append([]chan *Message(nil), p.subs...)
	p.mu.Unlock()
	for _, ch := range subs {
		ch <- m
	}
}

func (p *pipe) subscribe(ctx context.Context, fn func(context.Context, *Message) error) error {
	ch := make(chan *Message, 256)
	p.mu.Lock()
	p.subs = append(p.subs, ch)
	pending := p.buf
	p.buf = nil
	p.mu.Unlock()
	for _, m := range pending {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ch <- m:
		}
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case m := <-ch:
			if err := fn(m.Context(), m); err != nil {
				slog.Error("memory queue handler", "err", err, "key", m.Key)
			}
		}
	}
}

type memoryProducer struct {
	pipe *pipe
}

func (p *memoryProducer) Send(ctx context.Context, key string, value []byte) error {
	p.pipe.send(&Message{
		Key:   key,
		Value: append([]byte(nil), value...),
		ctx:   ctx,
	})
	return nil
}

type memoryConsumer struct {
	pipe *pipe
}

func (c *memoryConsumer) Subscribe(ctx context.Context, fn func(context.Context, *Message) error) error {
	return c.pipe.subscribe(ctx, fn)
}
