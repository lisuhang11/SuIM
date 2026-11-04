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
	Redis      config.Redis    `yaml:"redis"`
	LocalCache LocalCache      `yaml:"localCache"`
	Registry   register.Config `yaml:"registry"`
	Webhooks   Webhooks        `yaml:"webhooks"`
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

type Webhooks struct {
	URL                   string               `yaml:"url"`
	BeforeAddFriend       webhook.BeforeConfig `yaml:"beforeAddFriend"`
	AfterAddFriend        webhook.AfterConfig  `yaml:"afterAddFriend"`
	BeforeAddFriendAgree  webhook.BeforeConfig `yaml:"beforeAddFriendAgree"`
	AfterAddFriendAgree   webhook.AfterConfig  `yaml:"afterAddFriendAgree"`
	BeforeSetFriendRemark webhook.BeforeConfig `yaml:"beforeSetFriendRemark"`
	AfterSetFriendRemark  webhook.AfterConfig  `yaml:"afterSetFriendRemark"`
	AfterDeleteFriend     webhook.AfterConfig  `yaml:"afterDeleteFriend"`
	BeforeAddBlack        webhook.BeforeConfig `yaml:"beforeAddBlack"`
	AfterRemoveBlack      webhook.AfterConfig  `yaml:"afterRemoveBlack"`
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
	if !c.LocalCache.Enable() {
		return fmt.Errorf("localCache topic, slotNum, slotSize, successExpire and failedExpire are required")
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	w := c.Webhooks
	if err := webhook.RequireURL(w.URL,
		w.BeforeAddFriend.Enable, w.AfterAddFriend.Enable,
		w.BeforeAddFriendAgree.Enable, w.AfterAddFriendAgree.Enable,
		w.BeforeSetFriendRemark.Enable, w.AfterSetFriendRemark.Enable,
		w.AfterDeleteFriend.Enable, w.BeforeAddBlack.Enable, w.AfterRemoveBlack.Enable,
	); err != nil {
		return err
	}
	return nil
}
