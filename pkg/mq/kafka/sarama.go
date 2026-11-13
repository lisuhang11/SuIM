package kafka

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/IBM/sarama"
)

func applyAuth(kfk *sarama.Config, conf *Config) error {
	if conf.Username != "" || conf.Password != "" {
		kfk.Net.SASL.Enable = true
		kfk.Net.SASL.User = conf.Username
		kfk.Net.SASL.Password = conf.Password
	}
	if conf.TLS.EnableTLS {
		tlsCfg, err := newTLSConfig(conf.TLS.ClientCrt, conf.TLS.ClientKey, conf.TLS.CACrt, []byte(conf.TLS.ClientKeyPwd), conf.TLS.InsecureSkipVerify)
		if err != nil {
			return err
		}
		kfk.Net.TLS.Config = tlsCfg
		kfk.Net.TLS.Enable = true
	}
	return nil
}

// BuildConsumerGroupConfig builds a sarama consumer-group config.
func BuildConsumerGroupConfig(conf *Config, initial int64, autoCommitEnable bool) (*sarama.Config, error) {
	if conf == nil {
		return nil, fmt.Errorf("kafka: nil config")
	}
	kfk := sarama.NewConfig()
	kfk.Version = sarama.V2_0_0_0
	kfk.Consumer.Offsets.Initial = initial
	kfk.Consumer.Offsets.AutoCommit.Enable = autoCommitEnable
	kfk.Consumer.Return.Errors = false
	if conf.ConsumerFetchDefaultBytes > 0 {
		kfk.Consumer.Fetch.Default = int32(conf.ConsumerFetchDefaultBytes)
	}
	if conf.ConsumerFetchMaxBytes > 0 {
		kfk.Consumer.Fetch.Max = int32(conf.ConsumerFetchMaxBytes)
	}
	if err := applyAuth(kfk, conf); err != nil {
		return nil, err
	}
	return kfk, nil
}

// BuildProducerConfig builds a sync-producer config (hash partitioner, acks, compression).
func BuildProducerConfig(conf Config) (*sarama.Config, error) {
	kfk := sarama.NewConfig()
	kfk.Producer.Return.Successes = true
	kfk.Producer.Return.Errors = true
	kfk.Producer.Partitioner = sarama.NewHashPartitioner
	if err := applyAuth(kfk, &conf); err != nil {
		return nil, err
	}
	switch strings.ToLower(conf.ProducerAck) {
	case "no_response":
		kfk.Producer.RequiredAcks = sarama.NoResponse
	case "wait_for_local":
		kfk.Producer.RequiredAcks = sarama.WaitForLocal
	case "wait_for_all", "":
		kfk.Producer.RequiredAcks = sarama.WaitForAll
	default:
		kfk.Producer.RequiredAcks = sarama.WaitForAll
	}
	if conf.CompressType == "" {
		kfk.Producer.Compression = sarama.CompressionNone
	} else if err := kfk.Producer.Compression.UnmarshalText(bytes.ToLower([]byte(conf.CompressType))); err != nil {
		return nil, fmt.Errorf("kafka: compressType %q: %w", conf.CompressType, err)
	}
	if conf.MaxMessageBytes > 0 {
		kfk.Producer.MaxMessageBytes = conf.MaxMessageBytes
	}
	return kfk, nil
}

// NewSyncProducer creates a sarama SyncProducer.
func NewSyncProducer(conf *sarama.Config, addr []string) (sarama.SyncProducer, error) {
	producer, err := sarama.NewSyncProducer(addr, conf)
	if err != nil {
		return nil, fmt.Errorf("kafka: new sync producer: %w", err)
	}
	return producer, nil
}

// NewSaramaConsumerGroup creates a sarama ConsumerGroup.
func NewSaramaConsumerGroup(conf *sarama.Config, addr []string, groupID string) (sarama.ConsumerGroup, error) {
	cg, err := sarama.NewConsumerGroup(addr, groupID, conf)
	if err != nil {
		return nil, fmt.Errorf("kafka: new consumer group %s: %w", groupID, err)
	}
	return cg, nil
}
