package config

import (
	"fmt"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/webhook"
)

// Config 是推送服务的启动配置。
type Config struct {
	ListenIP   string          `yaml:"listenIP"`
	RegisterIP string          `yaml:"registerIP"`
	Port       int             `yaml:"port"`
	Redis      config.Redis    `yaml:"redis"`
	Registry   register.Config `yaml:"registry"`
	Queue      Queue           `yaml:"queue"`
	Push       Push            `yaml:"push"`
	Webhooks   Webhooks        `yaml:"webhooks"`
}

// Queue 控制 toPush 消费者的订阅方式。
type Queue struct {
	StreamPrefix  string `yaml:"streamPrefix"`
	MaxLen        int64  `yaml:"maxLen"`
	ConsumerGroup string `yaml:"consumerGroup"`
}

// Push 选择离线厂商并设置各家参数。
type Push struct {
	Enable               string  `yaml:"enable"`
	GeTui                GeTui   `yaml:"getui"`
	FCM                  FCM     `yaml:"fcm"`
	JPush                JPush   `yaml:"jpush"`
	IOSPush              IOSPush `yaml:"iosPush"`
	MaxConcurrentWorkers int     `yaml:"maxConcurrentWorkers"`
}

// GeTui 是个推 REST v2 的鉴权与下发参数。
type GeTui struct {
	PushURL      string `yaml:"pushURL"`
	AppKey       string `yaml:"appKey"`
	MasterSecret string `yaml:"masterSecret"`
	Intent       string `yaml:"intent"`
	ChannelID    string `yaml:"channelID"`
	ChannelName  string `yaml:"channelName"`
}

// FCM 是 Firebase 服务账号凭据的位置。
type FCM struct {
	CredentialsFile string `yaml:"credentialsFile"`
	AuthURL         string `yaml:"authURL"`
}

// JPush 是极光 REST v3 的鉴权与跳转参数。
type JPush struct {
	AppKey       string `yaml:"appKey"`
	MasterSecret string `yaml:"masterSecret"`
	PushURL      string `yaml:"pushURL"`
	PushIntent   string `yaml:"pushIntent"`
}

// IOSPush 是 iOS 通知的声音与角标设置。
type IOSPush struct {
	PushSound  string `yaml:"pushSound"`
	BadgeCount bool   `yaml:"badgeCount"`
	Production bool   `yaml:"production"`
}

// Webhooks 控制离线推送前的同步回调。
type Webhooks struct {
	URL               string               `yaml:"url"`
	BeforeOfflinePush webhook.BeforeConfig `yaml:"beforeOfflinePush"`
}

// Validate 补齐推送服务的默认值并检查必需项。
func (c *Config) Validate() error {
	if len(c.Redis.Address) == 0 {
		return fmt.Errorf("redis address is required")
	}
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	if c.ListenIP == "" {
		c.ListenIP = "0.0.0.0"
	}
	if c.RegisterIP == "" {
		c.RegisterIP = "127.0.0.1"
	}
	if c.Port <= 0 {
		c.Port = 10180
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
	if c.Queue.ConsumerGroup == "" {
		c.Queue.ConsumerGroup = "push"
	}
	if c.Push.Enable == "" {
		c.Push.Enable = "dummy"
	}
	c.Push.Enable = strings.ToLower(strings.TrimSpace(c.Push.Enable))
	if c.Push.MaxConcurrentWorkers < 1 {
		c.Push.MaxConcurrentWorkers = 8
	}
	if c.Webhooks.BeforeOfflinePush.Enable && c.Webhooks.URL == "" {
		return fmt.Errorf("webhook url is required when beforeOfflinePush is enabled")
	}
	return nil
}
