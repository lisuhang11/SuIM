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

type Webhooks struct {
	URL                      string               `yaml:"url"`
	BeforeCreateGroup        webhook.BeforeConfig `yaml:"beforeCreateGroup"`
	AfterCreateGroup         webhook.AfterConfig  `yaml:"afterCreateGroup"`
	BeforeInviteJoinGroup    webhook.BeforeConfig `yaml:"beforeInviteJoinGroup"`
	BeforeJoinGroup          webhook.BeforeConfig `yaml:"beforeJoinGroup"`
	BeforeMembersJoinGroup   webhook.BeforeConfig `yaml:"beforeMembersJoinGroup"`
	AfterJoinGroup           webhook.AfterConfig  `yaml:"afterJoinGroup"`
	AfterQuitGroup           webhook.AfterConfig  `yaml:"afterQuitGroup"`
	AfterKickGroup           webhook.AfterConfig  `yaml:"afterKickGroup"`
	AfterDismissGroup        webhook.AfterConfig  `yaml:"afterDismissGroup"`
	AfterTransferGroupOwner  webhook.AfterConfig  `yaml:"afterTransferGroupOwner"`
	BeforeSetGroupInfo       webhook.BeforeConfig `yaml:"beforeSetGroupInfo"`
	AfterSetGroupInfo        webhook.AfterConfig  `yaml:"afterSetGroupInfo"`
	BeforeSetGroupInfoEx     webhook.BeforeConfig `yaml:"beforeSetGroupInfoEx"`
	AfterSetGroupInfoEx      webhook.AfterConfig  `yaml:"afterSetGroupInfoEx"`
	BeforeSetGroupMemberInfo webhook.BeforeConfig `yaml:"beforeSetGroupMemberInfo"`
	AfterSetGroupMemberInfo  webhook.AfterConfig  `yaml:"afterSetGroupMemberInfo"`
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
	if !c.LocalCache.Enable() {
		return fmt.Errorf("localCache topic, slotNum, slotSize, successExpire and failedExpire are required")
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	w := c.Webhooks
	return webhook.RequireURL(w.URL,
		w.BeforeCreateGroup.Enable, w.AfterCreateGroup.Enable,
		w.BeforeInviteJoinGroup.Enable, w.BeforeJoinGroup.Enable, w.BeforeMembersJoinGroup.Enable,
		w.AfterJoinGroup.Enable, w.AfterQuitGroup.Enable, w.AfterKickGroup.Enable,
		w.AfterDismissGroup.Enable, w.AfterTransferGroupOwner.Enable,
		w.BeforeSetGroupInfo.Enable, w.AfterSetGroupInfo.Enable,
		w.BeforeSetGroupInfoEx.Enable, w.AfterSetGroupInfoEx.Enable,
		w.BeforeSetGroupMemberInfo.Enable, w.AfterSetGroupMemberInfo.Enable,
	)
}
