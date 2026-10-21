package config

import (
	"fmt"
	"time"

	"github.com/lisuhang11/SuIM/pkg/config"
)

type Config struct {
	MySQL              config.MySQL `yaml:"mysql"`
	Redis              config.Redis `yaml:"redis"`
	Queue              Queue        `yaml:"queue"`
	MsgCacheTTLSeconds int          `yaml:"msgCacheTTLSeconds"`
	Webhooks           Webhooks     `yaml:"webhooks"`
}

type Queue struct {
	StreamPrefix string `yaml:"streamPrefix"`
	MaxLen       int64  `yaml:"maxLen"`
	BatchSize    int    `yaml:"batchSize"`
	Workers      int    `yaml:"workers"`
	IntervalMs   int    `yaml:"intervalMs"`
}

type Webhooks struct {
	URL            string         `yaml:"url"`
	AfterMsgSaveDB AfterMsgSaveDB `yaml:"afterMsgSaveDB"`
}

type AfterMsgSaveDB struct {
	Enable       bool     `yaml:"enable"`
	Timeout      int      `yaml:"timeout"`
	AttentionIds []string `yaml:"attentionIds"`
	DeniedTypes  []int32  `yaml:"deniedTypes"`
}

func (c *Config) Validate() error {
	if len(c.MySQL.Address) == 0 || c.MySQL.Username == "" || c.MySQL.Database == "" {
		return fmt.Errorf("mysql address, username and database are required")
	}
	if len(c.Redis.Address) == 0 {
		return fmt.Errorf("redis address is required")
	}
	if c.Redis.RedisMode == "" {
		c.Redis.RedisMode = "standalone"
	}
	if c.Queue.StreamPrefix == "" {
		c.Queue.StreamPrefix = "mq"
	}
	if c.Queue.MaxLen <= 0 {
		c.Queue.MaxLen = 100000
	}
	if c.Queue.BatchSize <= 0 {
		c.Queue.BatchSize = 100
	}
	if c.Queue.Workers <= 0 {
		c.Queue.Workers = 8
	}
	if c.Queue.IntervalMs <= 0 {
		c.Queue.IntervalMs = 100
	}
	if c.MsgCacheTTLSeconds <= 0 {
		c.MsgCacheTTLSeconds = 86400
	}
	return nil
}

func (c *Config) MsgCacheTTL() time.Duration {
	return time.Duration(c.MsgCacheTTLSeconds) * time.Second
}

func (c *Config) BatchInterval() time.Duration {
	return time.Duration(c.Queue.IntervalMs) * time.Millisecond
}
