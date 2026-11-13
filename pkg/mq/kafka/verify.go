package kafka

import (
	"context"
	"fmt"

	"github.com/IBM/sarama"
)

// CheckTopics verifies each topic exists on the cluster.
func CheckTopics(ctx context.Context, conf *Config, topics []string) error {
	_ = ctx
	if conf == nil {
		return fmt.Errorf("kafka: nil config")
	}
	kfk, err := BuildConsumerGroupConfig(conf, sarama.OffsetNewest, false)
	if err != nil {
		return err
	}
	cli, err := sarama.NewClient(conf.Addr, kfk)
	if err != nil {
		return fmt.Errorf("kafka: new client: %w", err)
	}
	defer cli.Close()

	existing, err := cli.Topics()
	if err != nil {
		return fmt.Errorf("kafka: list topics: %w", err)
	}
	have := make(map[string]bool, len(existing))
	for _, t := range existing {
		have[t] = true
	}
	for _, topic := range topics {
		if !have[topic] {
			return fmt.Errorf("kafka: topic %q not exist", topic)
		}
	}
	return nil
}

// CheckHealth connects to brokers and opens each advertised broker.
func CheckHealth(ctx context.Context, conf *Config) error {
	_ = ctx
	if conf == nil {
		return fmt.Errorf("kafka: nil config")
	}
	kfk, err := BuildConsumerGroupConfig(conf, sarama.OffsetNewest, false)
	if err != nil {
		return err
	}
	cli, err := sarama.NewClient(conf.Addr, kfk)
	if err != nil {
		return fmt.Errorf("kafka: new client: %w", err)
	}
	defer cli.Close()

	brokers := cli.Brokers()
	if len(brokers) == 0 {
		return fmt.Errorf("kafka: no brokers found")
	}
	for _, broker := range brokers {
		connected, err := broker.Connected()
		if err != nil {
			return fmt.Errorf("kafka: broker %s status: %w", broker.Addr(), err)
		}
		if connected {
			continue
		}
		if err := broker.Open(kfk); err != nil {
			return fmt.Errorf("kafka: open broker %s: %w", broker.Addr(), err)
		}
	}
	return nil
}
