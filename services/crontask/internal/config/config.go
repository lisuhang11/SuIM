package config

import (
	"fmt"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
)

type Config struct {
	Redis    config.Redis    `yaml:"redis"`
	Registry register.Config `yaml:"registry"`
	CronTask CronTask        `yaml:"cronTask"`
}

type CronTask struct {
	CronExecuteTime   string   `yaml:"cronExecuteTime"`
	RetainChatRecords int      `yaml:"retainChatRecords"`
	FileExpireTime    int      `yaml:"fileExpireTime"`
	DeleteObjectType  []string `yaml:"deleteObjectType"`
}

func (c *Config) Validate() error {
	if len(c.Redis.Address) == 0 {
		return fmt.Errorf("redis address is required")
	}
	if c.Redis.RedisMode == "" {
		c.Redis.RedisMode = "standalone"
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	if c.CronTask.CronExecuteTime == "" {
		c.CronTask.CronExecuteTime = "0 2 * * *"
	}
	return nil
}
