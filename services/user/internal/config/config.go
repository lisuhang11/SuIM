package config

import (
	"fmt"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
)

type Config struct {
	ListenIP   string          `yaml:"listenIP"`
	RegisterIP string          `yaml:"registerIP"`
	Port       int             `yaml:"port"`
	MySQL      config.MySQL    `yaml:"mysql"`
	Redis      config.Redis    `yaml:"redis"`
	JWT        config.JWT      `yaml:"jwt"`
	LocalCache LocalCache      `yaml:"localCache"`
	Registry   register.Config `yaml:"registry"`
}

type LocalCache struct {
	Topic         string `yaml:"topic"`
	SlotNum       int    `yaml:"slotNum"`
	SlotSize      int    `yaml:"slotSize"`
	SuccessExpire int    `yaml:"successExpire"`
	FailedExpire  int    `yaml:"failedExpire"`
}

func (c LocalCache) Enable() bool {
	return c.Topic != "" && c.SlotNum > 0 && c.SlotSize > 0 && c.SuccessExpire > 0 && c.FailedExpire > 0
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
	if len(c.MySQL.Address) == 0 || c.MySQL.Username == "" || c.MySQL.Database == "" {
		return fmt.Errorf("mysql address, username and database are required")
	}
	if len(c.Redis.Address) == 0 {
		return fmt.Errorf("redis address is required")
	}
	if c.Redis.RedisMode == "" {
		c.Redis.RedisMode = "standalone"
	}
	if c.JWT.Secret == "" || c.JWT.ExpireSeconds <= 0 {
		return fmt.Errorf("jwt secret and expireSeconds are required")
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	return nil
}
