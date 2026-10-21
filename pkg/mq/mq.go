package mq

import "context"

const (
	// TopicToRedis carries messages that still need a sequence number.
	TopicToRedis = "toRedis"
	// TopicToMySQL carries sequenced messages waiting to be stored.
	TopicToMySQL = "toMysql"
	// TopicToPush carries messages waiting to be pushed.
	TopicToPush = "toPush"
)

// Message is one queue record. Ack removes it after the handler has finished.
type Message struct {
	Key   string
	Value []byte
	ctx   context.Context
	ack   func(context.Context) error
}

// Context returns the context attached when the message was received.
func (m *Message) Context() context.Context {
	if m == nil || m.ctx == nil {
		return context.Background()
	}
	return m.ctx
}

// Ack confirms the message. A nil ack function succeeds.
func (m *Message) Ack(ctx context.Context) error {
	if m == nil || m.ack == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return m.ack(ctx)
}

// Producer appends records to one topic.
type Producer interface {
	Send(ctx context.Context, key string, value []byte) error
}

// Consumer reads one topic until ctx is canceled.
// The handler must Ack the message after it has been processed.
type Consumer interface {
	Subscribe(ctx context.Context, fn func(context.Context, *Message) error) error
}
