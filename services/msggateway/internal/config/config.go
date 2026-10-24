package config

import (
	"fmt"
	"os"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
)

type Config struct {
	ListenIP            string          `yaml:"listenIP"`
	RegisterIP          string          `yaml:"registerIP"`
	RPCPort             int             `yaml:"rpcPort"`
	WSPort              int             `yaml:"wsPort"`
	MaxConn             int64           `yaml:"maxConn"`
	HandshakeTimeoutSec int             `yaml:"handshakeTimeoutSec"`
	Redis               config.Redis    `yaml:"redis"`
	Queue               Queue           `yaml:"queue"`
	Registry            register.Config `yaml:"registry"`
}

type Queue struct {
	StreamPrefix string `yaml:"streamPrefix"`
	MaxLen       int64  `yaml:"maxLen"`
}

func (c *Config) Validate() error {
	if c.RPCPort <= 0 || c.WSPort <= 0 {
		return fmt.Errorf("rpcPort and wsPort are required")
	}
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	if c.RegisterIP == "" {
		c.RegisterIP = "127.0.0.1"
	}
	if c.MaxConn <= 0 {
		c.MaxConn = 100000
	}
	if c.HandshakeTimeoutSec <= 0 {
		c.HandshakeTimeoutSec = 10
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
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	return nil
}

func ConsumerGroup() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "msggateway"
	}
	return "msggateway-" + host
}
