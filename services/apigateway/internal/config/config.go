package config

import (
	"fmt"

	"github.com/lisuhang11/SuIM/pkg/register"
)

type Config struct {
	ListenIP string          `yaml:"listenIP"`
	Port     int             `yaml:"port"`
	Registry register.Config `yaml:"registry"`
}

func (c *Config) Validate() error {
	if c.Port <= 0 {
		return fmt.Errorf("port is required")
	}
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	return nil
}
