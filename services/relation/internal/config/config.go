package config

import (
	"fmt"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/webhook"
)

type Config struct {
	ListenIP   string          `yaml:"listenIP"`
	RegisterIP string          `yaml:"registerIP"`
	Port       int             `yaml:"port"`
	MySQL      config.MySQL    `yaml:"mysql"`
	Registry   register.Config `yaml:"registry"`
	Webhooks   Webhooks        `yaml:"webhooks"`
}

type Webhooks struct {
	URL              string               `yaml:"url"`
	BeforeAddBlack   webhook.BeforeConfig `yaml:"beforeAddBlack"`
	AfterRemoveBlack webhook.AfterConfig  `yaml:"afterRemoveBlack"`
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
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	if c.Webhooks.URL == "" && (c.Webhooks.BeforeAddBlack.Enable || c.Webhooks.AfterRemoveBlack.Enable) {
		return fmt.Errorf("webhook url is required when a webhook is enabled")
	}
	return nil
}
