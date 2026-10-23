package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
)

type Config struct {
	ListenIP          string          `yaml:"listenIP"`
	RegisterIP        string          `yaml:"registerIP"`
	Port              int             `yaml:"port"`
	SignExpireSeconds int64           `yaml:"signExpireSeconds"`
	MinIO             MinIO           `yaml:"minio"`
	MySQL             config.MySQL    `yaml:"mysql"`
	Redis             config.Redis    `yaml:"redis"`
	Registry          register.Config `yaml:"registry"`
}

// MinIO is where part bytes and finished objects are stored.
// SignEndpoint is the host written into presigned URLs. Leave it empty to use Endpoint.
type MinIO struct {
	Endpoint     string `yaml:"endpoint"`
	SignEndpoint string `yaml:"signEndpoint"`
	Bucket       string `yaml:"bucket"`
	AccessKey    string `yaml:"accessKey"`
	SecretKey    string `yaml:"secretKey"`
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
	if err := c.MinIO.validate(); err != nil {
		return err
	}
	if c.SignExpireSeconds == 0 {
		c.SignExpireSeconds = int64((24 * time.Hour).Seconds())
	}
	if c.SignExpireSeconds < 60 || c.SignExpireSeconds > int64((7*24*time.Hour).Seconds()) {
		return fmt.Errorf("signExpireSeconds must be between 60 and 604800")
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
	if c.Registry.Type == "" {
		return fmt.Errorf("registry type is required")
	}
	return nil
}

func (m *MinIO) validate() error {
	if err := absoluteHTTP(m.Endpoint); err != nil {
		return fmt.Errorf("minio endpoint: %w", err)
	}
	m.Endpoint = strings.TrimRight(m.Endpoint, "/")
	if m.SignEndpoint != "" {
		if err := absoluteHTTP(m.SignEndpoint); err != nil {
			return fmt.Errorf("minio signEndpoint: %w", err)
		}
		m.SignEndpoint = strings.TrimRight(m.SignEndpoint, "/")
	}
	if m.Bucket == "" || strings.Contains(m.Bucket, "/") {
		return fmt.Errorf("minio bucket is required")
	}
	if m.AccessKey == "" || m.SecretKey == "" {
		return fmt.Errorf("minio accessKey and secretKey are required")
	}
	return nil
}

func absoluteHTTP(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("must be an absolute http(s) url")
	}
	return nil
}
