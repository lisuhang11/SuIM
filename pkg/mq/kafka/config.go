// Package kafka wraps IBM sarama for SuIM message queues.
//
// Layout follows OpenIM's tools/mq/kafka: shared Config → producer / consumer-group
// builders, TLS/SASL, health checks, and adapters that satisfy SuIM's mq.Producer
// and mq.Consumer interfaces (byte payloads + Ack).
package kafka

// TLSConfig is optional mutual TLS for broker connections.
type TLSConfig struct {
	EnableTLS          bool   `yaml:"enableTLS"`
	CACrt              string `yaml:"caCrt"`
	ClientCrt          string `yaml:"clientCrt"`
	ClientKey          string `yaml:"clientKey"`
	ClientKeyPwd       string `yaml:"clientKeyPwd"`
	InsecureSkipVerify bool   `yaml:"insecureSkipVerify"`
}

// Config is broker connection + producer/consumer tuning.
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
}
