package kafka

import (
	"context"
	"errors"
	"fmt"

	"github.com/IBM/sarama"

	"github.com/lisuhang11/SuIM/pkg/mq"
)

var errEmptyMsg = errors.New("kafka message is empty")

// Producer is a topic-scoped sync producer implementing mq.Producer.
type Producer struct {
	topic    string
	producer sarama.SyncProducer
}

// NewProducer builds a SuIM mq.Producer for one Kafka topic.
func NewProducer(conf *Config, topic string) (*Producer, error) {
	if conf == nil {
		return nil, fmt.Errorf("kafka: nil config")
	}
	if topic == "" {
		return nil, fmt.Errorf("kafka: empty topic")
	}
	if len(conf.Addr) == 0 {
		return nil, fmt.Errorf("kafka: empty addr")
	}
	saramaCfg, err := BuildProducerConfig(*conf)
	if err != nil {
		return nil, err
	}
	producer, err := NewSyncProducer(saramaCfg, conf.Addr)
	if err != nil {
		return nil, err
	}
	return &Producer{topic: topic, producer: producer}, nil
}

// Send publishes key/value with hash partitioning (same key → same partition).
func (p *Producer) Send(ctx context.Context, key string, value []byte) error {
	if p == nil || p.producer == nil {
		return fmt.Errorf("kafka: nil producer")
	}
	if len(value) == 0 {
		return errEmptyMsg
	}
	msg := &sarama.ProducerMessage{
		Topic:   p.topic,
		Key:     sarama.StringEncoder(key),
		Value:   sarama.ByteEncoder(value),
		Headers: HeadersFromContext(ctx),
	}
	if _, _, err := p.producer.SendMessage(msg); err != nil {
		return fmt.Errorf("kafka: send %s: %w", p.topic, err)
	}
	return nil
}

// Close shuts down the underlying sync producer.
func (p *Producer) Close() error {
	if p == nil || p.producer == nil {
		return nil
	}
	return p.producer.Close()
}

var _ mq.Producer = (*Producer)(nil)
