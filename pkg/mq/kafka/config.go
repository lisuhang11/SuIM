// Package kafka wraps IBM sarama for SuIM message queues.
//
// Layout follows OpenIM's tools/mq/kafka: shared Config → producer / consumer-group
// builders, TLS/SASL, health checks, and adapters that satisfy SuIM's mq.Producer
// and mq.Consumer interfaces (byte payloads + Ack).
package kafka

import "fmt"

// TLSConfig is optional mutual TLS for broker connections.
type TLSConfig struct {
	EnableTLS          bool   `yaml:"enableTLS"`
	CACrt              string `yaml:"caCrt"`
	ClientCrt          string `yaml:"clientCrt"`
	ClientKey          string `yaml:"clientKey"`
	ClientKeyPwd       string `yaml:"clientKeyPwd"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

// Config is broker connection + producer/consumer tuning + SuIM topic map.
type Config struct {
	Username                  string    `yaml:"username"`
	Password                  string    `yaml:"password"`
	ProducerAck               string    `yaml:"producerAck"` // no_response | wait_for_local | wait_for_all
	CompressType              string    `yaml:"compressType"`
	MaxMessageBytes           int       `yaml:"maxMessageBytes"`
	ConsumerFetchDefaultBytes int       `yaml:"consumerFetchDefaultBytes"`
	ConsumerFetchMaxBytes     int       `yaml:"consumerFetchMaxBytes"`
	Addr                      []string  `yaml:"addr"`
	TLS                       TLSConfig `yaml:"tls"`

	ToRedisTopic string `yaml:"toRedisTopic"`
	ToMysqlTopic string `yaml:"toMysqlTopic"`
	ToPushTopic  string `yaml:"toPushTopic"`

	ToRedisGroupID string `yaml:"toRedisGroupID"`
	ToMysqlGroupID string `yaml:"toMysqlGroupID"`
	ToPushGroupID  string `yaml:"toPushGroupID"`
}

// ApplyDefaults fills empty topic / group names used by SuIM pipelines.
func (c *Config) ApplyDefaults() {
	if c == nil {
		return
	}
	if c.ProducerAck == "" {
		c.ProducerAck = "wait_for_all"
	}
	if c.CompressType == "" {
		c.CompressType = "none"
	}
	if c.ToRedisTopic == "" {
		c.ToRedisTopic = "toRedis"
	}
	if c.ToMysqlTopic == "" {
		c.ToMysqlTopic = "toMysql"
	}
	if c.ToPushTopic == "" {
		c.ToPushTopic = "toPush"
	}
	if c.ToRedisGroupID == "" {
		c.ToRedisGroupID = "toRedis"
	}
	if c.ToMysqlGroupID == "" {
		c.ToMysqlGroupID = "toMysql"
	}
	if c.ToPushGroupID == "" {
		c.ToPushGroupID = "msggateway"
	}
}

// Validate checks required broker settings after defaults.
func (c *Config) Validate() error {
	if c == nil {
		return fmt.Errorf("kafka config is required")
	}
	c.ApplyDefaults()
	if len(c.Addr) == 0 {
		return fmt.Errorf("kafka addr is required")
	}
	return nil
}
