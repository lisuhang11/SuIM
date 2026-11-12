package config

import (
	"fmt"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	"github.com/lisuhang11/SuIM/pkg/register"
)

type Config struct {
	ListenIP   string          `yaml:"listenIP"`
	RegisterIP string          `yaml:"registerIP"`
	Port       int             `yaml:"port"`
	Redis      config.Redis    `yaml:"redis"`
	Registry   register.Config `yaml:"registry"`
	LiveKit    livekit.Config  `yaml:"livekit"`
	Agents     AgentsConfig    `yaml:"agents"`
}

// AgentsConfig controls LiveKit Agent dispatch from the rtc service.
type AgentsConfig struct {
	Enabled      bool     `yaml:"enabled"`
	Stream       string   `yaml:"stream"`
	BotUserIDs   []string `yaml:"botUserIds"`
	Identity     string   `yaml:"identity"`
	JoinOnAccept bool     `yaml:"joinOnAccept"`
}

func (c *Config) Validate() error {
	if c.Port <= 0 {
		return fmt.Errorf("port is required")
	}
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	if c.RegisterIP == "" {
		c.RegisterIP = "127.0.0.1"
	}
	if len(c.Redis.Address) == 0 {
		return fmt.Errorf("redis address is required")
	}
	if c.Redis.RedisMode == "" {
		c.Redis.RedisMode = "standalone"
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	return c.LiveKit.Validate()
}
