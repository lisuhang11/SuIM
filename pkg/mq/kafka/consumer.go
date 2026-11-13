package kafka

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/IBM/sarama"

	"github.com/lisuhang11/SuIM/pkg/mq"
)

type pendingMessage struct {
	msg     *sarama.ConsumerMessage
	session sarama.ConsumerGroupSession
}

// Consumer is a consumer-group reader implementing mq.Consumer.
// Subscribe blocks and dispatches messages until ctx is canceled.
type Consumer struct {
	topics     []string
	groupID    string
	autoCommit bool
	group      sarama.ConsumerGroup
	ctx        context.Context
	cancel     context.CancelFunc
	ch         chan *pendingMessage
	once       sync.Once
}

// NewConsumer starts a background Consume loop for the given topics/group.
// When autoCommit is false, Message.Ack marks and commits the offset (Redis XAck-like).
// When autoCommit is true, Ack only marks the message for the next auto-commit.
func NewConsumer(ctx context.Context, conf *Config, groupID string, topics []string, autoCommit bool) (*Consumer, error) {
	if conf == nil {
		return nil, fmt.Errorf("kafka: nil config")
	}
	if groupID == "" {
		return nil, fmt.Errorf("kafka: empty group id")
	}
	if len(topics) == 0 {
		return nil, fmt.Errorf("kafka: empty topics")
	}
	if len(conf.Addr) == 0 {
		return nil, fmt.Errorf("kafka: empty addr")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	saramaCfg, err := BuildConsumerGroupConfig(conf, sarama.OffsetNewest, autoCommit)
	if err != nil {
		return nil, err
	}
	group, err := NewSaramaConsumerGroup(saramaCfg, conf.Addr, groupID)
	if err != nil {
		return nil, err
	}
	c := &Consumer{
		topics:     append([]string(nil), topics...),
		groupID:    groupID,
		autoCommit: autoCommit,
		group:      group,
		ch:         make(chan *pendingMessage, 64),
	}
	c.ctx, c.cancel = context.WithCancel(ctx)
	go c.loop()
	return c, nil
}

func (c *Consumer) Setup(sarama.ConsumerGroupSession) error   { return nil }
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error { return nil }

func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	defer func() { _ = recover() }()
	for {
		select {
		case <-c.ctx.Done():
			return context.Canceled
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			select {
			case <-c.ctx.Done():
				return context.Canceled
			case c.ch <- &pendingMessage{msg: msg, session: session}:
			}
		}
	}
}

func (c *Consumer) loop() {
	defer c.closeChan()
	for {
		err := c.group.Consume(c.ctx, c.topics, c)
		switch {
		case err == nil:
			continue
		case errors.Is(err, context.Canceled), errors.Is(err, sarama.ErrClosedConsumerGroup):
			return
		default:
			slog.Warn("kafka consume", "err", err, "topics", c.topics, "group", c.groupID)
		}
	}
}

func (c *Consumer) closeChan() {
	c.once.Do(func() {
		c.cancel()
		close(c.ch)
	})
}

// Subscribe runs the handler for each message until ctx ends (SuIM Redis-style loop).
func (c *Consumer) Subscribe(ctx context.Context, fn func(context.Context, *mq.Message) error) error {
	if c == nil {
		return fmt.Errorf("kafka: nil consumer")
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-c.ctx.Done():
			return context.Canceled
		case item, ok := <-c.ch:
			if !ok {
				return sarama.ErrClosedConsumerGroup
			}
			msgCtx := ContextFromHeaders(ctx, item.msg.Headers)
			msg := &mq.Message{
				Key:   string(item.msg.Key),
				Value: append([]byte(nil), item.msg.Value...),
			}
			// Bind ack + context through package helpers on Message.
			bindKafkaMessage(msg, msgCtx, item, c.autoCommit)
			if err := fn(msg.Context(), msg); err != nil {
				slog.Error("kafka queue handler", "err", err, "topic", item.msg.Topic, "group", c.groupID)
				if ctx.Err() != nil {
					return ctx.Err()
				}
			}
		}
	}
}

// Close stops the consume loop and closes the group.
func (c *Consumer) Close() error {
	if c == nil {
		return nil
	}
	c.cancel()
	if c.group != nil {
		return c.group.Close()
	}
	return nil
}

var _ mq.Consumer = (*Consumer)(nil)
