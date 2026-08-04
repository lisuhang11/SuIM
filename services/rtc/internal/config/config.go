// Package config 提供 rtc 服务配置，从 YAML 配置文件加载。
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config rtc 服务全局配置。
type Config struct {
	ServerAddr    string   `yaml:"server_addr"`
	ServiceAddr   string   `yaml:"service_addr"`
	EtcdEndpoints []string `yaml:"etcd_endpoints"`
	DBHost        string   `yaml:"db_host"`
	DBPort        int      `yaml:"db_port"`
	DBUser        string   `yaml:"db_user"`
	DBPassword    string   `yaml:"db_password"`
	DBName        string   `yaml:"db_name"`
	JWTSecret     string   `yaml:"jwt_secret"`
	LiveKitURL    string   `yaml:"livekit_url"`
	LiveKitAPIKey string   `yaml:"livekit_api_key"`
	LiveKitAPISecret string `yaml:"livekit_api_secret"`
	RingTimeoutSec int     `yaml:"ring_timeout_sec"`
}

func defaults() *Config {
	return &Config{
		ServerAddr:       ":8087",
		ServiceAddr:      "127.0.0.1:8087",
		EtcdEndpoints:    []string{"127.0.0.1:2379"},
		DBHost:           "127.0.0.1",
		DBPort:           3306,
		DBUser:           "root",
		DBPassword:       "",
		DBName:           "suim",
		JWTSecret:        "change-me-in-production",
		LiveKitURL:       "ws://localhost:7880",
		LiveKitAPIKey:    "devkey",
		LiveKitAPISecret: "secret",
		RingTimeoutSec:   45,
	}
}

// LoadFromFile 从 YAML 文件加载配置。
func LoadFromFile(path string) *Config {
	cfg := defaults()

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			slog.Warn("[rtc] failed to parse config file, using defaults", "path", path, "error", err)
		}
	} else {
		slog.Warn("[rtc] config file not found, using defaults", "path", path)
	}

	if v := os.Getenv("SERVER_ADDR"); v != "" {
		cfg.ServerAddr = v
	}
	if v := os.Getenv("SERVICE_ADDR"); v != "" {
		cfg.ServiceAddr = v
	}
	if v := os.Getenv("ETCD_ENDPOINTS"); v != "" {
		cfg.EtcdEndpoints = strings.Split(v, ",")
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.DBHost = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.DBPort = p
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.DBUser = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.DBPassword = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.DBName = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("LIVEKIT_URL"); v != "" {
		cfg.LiveKitURL = v
	}
	if v := os.Getenv("LIVEKIT_API_KEY"); v != "" {
		cfg.LiveKitAPIKey = v
	}
	if v := os.Getenv("LIVEKIT_API_SECRET"); v != "" {
		cfg.LiveKitAPISecret = v
	}
	if v := os.Getenv("RING_TIMEOUT_SEC"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			cfg.RingTimeoutSec = sec
		}
	}
	return cfg
}

// DSN 返回 MySQL 数据源名称。
func (c *Config) DSN() string {
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + strconv.Itoa(c.DBPort) + ")/" + c.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}
