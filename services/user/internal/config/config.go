package config

import (
	"fmt"
	"strings"

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
	JWT        config.JWT      `yaml:"jwt"`
	LocalCache LocalCache      `yaml:"localCache"`
	Registry   register.Config `yaml:"registry"`
	Webhooks   Webhooks        `yaml:"webhooks"`
	SMS        SMS             `yaml:"sms"`
}

type SMS struct {
	Pepper           string   `yaml:"pepper"`
	Provider         string   `yaml:"provider"`
	URL              string   `yaml:"url"`
	CodeTTLSeconds   int      `yaml:"codeTTLSeconds"`
	RetryTTLSeconds  int      `yaml:"retryTTLSeconds"`
	PhonePerMinute   int      `yaml:"phonePerMinute"`
	PhonePerHour     int      `yaml:"phonePerHour"`
	PhonePerDay      int      `yaml:"phonePerDay"`
	IPPerMinute      int      `yaml:"ipPerMinute"`
	DevicePerMinute  int      `yaml:"devicePerMinute"`
	MaxAttempts      int      `yaml:"maxAttempts"`
	CaptchaRequired  bool     `yaml:"captchaRequired"`
	CaptchaURL       string   `yaml:"captchaURL"`
	DebugLogCode     bool     `yaml:"debugLogCode"`
	BlacklistPhones  []string `yaml:"blacklistPhones"`
	BlacklistIPs     []string `yaml:"blacklistIPs"`
	BlacklistDevices []string `yaml:"blacklistDevices"`
}

type Webhooks struct {
	URL                    string               `yaml:"url"`
	BeforeUserRegister     webhook.BeforeConfig `yaml:"beforeUserRegister"`
	AfterUserRegister      webhook.AfterConfig  `yaml:"afterUserRegister"`
	BeforeUpdateUserInfo   webhook.BeforeConfig `yaml:"beforeUpdateUserInfo"`
	AfterUpdateUserInfo    webhook.AfterConfig  `yaml:"afterUpdateUserInfo"`
	BeforeUpdateUserInfoEx webhook.BeforeConfig `yaml:"beforeUpdateUserInfoEx"`
	AfterUpdateUserInfoEx  webhook.AfterConfig  `yaml:"afterUpdateUserInfoEx"`
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
	if err := c.normalizeSMS(); err != nil {
		return err
	}
	w := c.Webhooks
	return webhook.RequireURL(w.URL,
		w.BeforeUserRegister.Enable, w.AfterUserRegister.Enable,
		w.BeforeUpdateUserInfo.Enable, w.AfterUpdateUserInfo.Enable,
		w.BeforeUpdateUserInfoEx.Enable, w.AfterUpdateUserInfoEx.Enable,
	)
}

func (c *Config) normalizeSMS() error {
	s := &c.SMS
	s.Pepper = strings.TrimSpace(s.Pepper)
	if s.Pepper == "" {
		return fmt.Errorf("sms pepper is required")
	}
	if s.Pepper == strings.TrimSpace(c.JWT.Secret) {
		return fmt.Errorf("sms pepper must differ from the jwt secret")
	}
	if s.Provider == "" {
		s.Provider = "log"
	}
	switch s.Provider {
	case "log":
	case "http":
		if strings.TrimSpace(s.URL) == "" {
			return fmt.Errorf("sms url is required")
		}
	default:
		return fmt.Errorf("sms provider is invalid")
	}
	if s.CodeTTLSeconds <= 0 {
		s.CodeTTLSeconds = 300
	}
	if s.RetryTTLSeconds <= 0 {
		s.RetryTTLSeconds = 120
	}
	if s.PhonePerMinute <= 0 {
		s.PhonePerMinute = 1
	}
	if s.PhonePerHour <= 0 {
		s.PhonePerHour = 5
	}
	if s.PhonePerDay <= 0 {
		s.PhonePerDay = 10
	}
	if s.IPPerMinute <= 0 {
		s.IPPerMinute = 20
	}
	if s.DevicePerMinute <= 0 {
		s.DevicePerMinute = 10
	}
	if s.MaxAttempts <= 0 {
		s.MaxAttempts = 5
	}
	if s.CaptchaRequired && strings.TrimSpace(s.CaptchaURL) == "" {
		return fmt.Errorf("sms captchaURL is required")
	}
	return nil
}
