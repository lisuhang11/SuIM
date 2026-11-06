package config

import (
	"fmt"
	"os"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/webhook"
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
	Webhooks            Webhooks        `yaml:"webhooks"`
}

type Webhooks struct {
	URL          string              `yaml:"url"`
	AfterOnline  webhook.AfterConfig `yaml:"afterUserOnline"`
	AfterOffline webhook.AfterConfig `yaml:"afterUserOffline"`
	AfterKickOff webhook.AfterConfig `yaml:"afterUserKickOff"`
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
	w := c.Webhooks
	return webhook.RequireURL(w.URL, w.AfterOnline.Enable, w.AfterOffline.Enable, w.AfterKickOff.Enable)
}

func ConsumerGroup() string {
	host, _ := os.Hostname()
	if host == "" {
		host = "msggateway"
	}
	return "msggateway-" + host
}
