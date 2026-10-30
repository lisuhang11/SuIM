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
	Queue      Queue           `yaml:"queue"`
	Registry   register.Config `yaml:"registry"`
	Webhooks   Webhooks        `yaml:"webhooks"`
}

type Webhooks struct {
	URL              string    `yaml:"url"`
	BeforeSendSingle MsgBefore `yaml:"beforeSendSingleMsg"`
	AfterSendSingle  MsgAfter  `yaml:"afterSendSingleMsg"`
	BeforeSendGroup  MsgBefore `yaml:"beforeSendGroupMsg"`
	AfterSendGroup   MsgAfter  `yaml:"afterSendGroupMsg"`
	BeforeMsgModify  MsgBefore `yaml:"beforeMsgModify"`
}

type MsgBefore struct {
	Enable         bool    `yaml:"enable"`
	Timeout        int     `yaml:"timeout"`
	FailedContinue bool    `yaml:"failedContinue"`
	DeniedTypes    []int32 `yaml:"deniedTypes"`
}

type MsgAfter struct {
	Enable       bool     `yaml:"enable"`
	Timeout      int      `yaml:"timeout"`
	AttentionIds []string `yaml:"attentionIds"`
	DeniedTypes  []int32  `yaml:"deniedTypes"`
}

type Queue struct {
	StreamPrefix string `yaml:"streamPrefix"`
	MaxLen       int64  `yaml:"maxLen"`
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
	return webhook.RequireURL(w.URL,
		w.BeforeSendSingle.Enable, w.AfterSendSingle.Enable,
		w.BeforeSendGroup.Enable, w.AfterSendGroup.Enable,
		w.BeforeMsgModify.Enable,
	)
}
