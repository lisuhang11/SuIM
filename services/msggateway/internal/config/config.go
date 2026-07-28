// Package config 从 YAML 配置文件加载 msg gateway 服务配置。
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 服务全局配置。
type Config struct {
	WSAddr                  string        `yaml:"ws_addr"`
	GRPCAddr                string        `yaml:"grpc_addr"`
	MetricsAddr             string        `yaml:"metrics_addr"`
	ServiceAddr             string        `yaml:"service_addr"`
	EtcdEndpoints           []string      `yaml:"etcd_endpoints"`
	MaxConnPerUser          int           `yaml:"max_conn_per_user"`
	PingInterval            time.Duration `yaml:"ping_interval"`
	ReadTimeout             time.Duration `yaml:"read_timeout"`
	WriteTimeout            time.Duration `yaml:"write_timeout"`
	GracefulShutdownTimeout time.Duration `yaml:"graceful_shutdown_timeout"`
}

// defaults 返回内置默认配置。
func defaults() *Config {
	return &Config{
		WSAddr:                  ":9001",
		GRPCAddr:                ":9091",
		MetricsAddr:             ":9092",
		ServiceAddr:             "127.0.0.1:9091",
		EtcdEndpoints:           []string{"127.0.0.1:2379"},
		MaxConnPerUser:          10,
		PingInterval:            30 * time.Second,
		ReadTimeout:             60 * time.Second,
		WriteTimeout:            10 * time.Second,
		GracefulShutdownTimeout: 30 * time.Second,
	}
}

// LoadFromFile 从 YAML 文件加载配置。
func LoadFromFile(path string) *Config {
	cfg := defaults()

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			slog.Warn("[msggateway] failed to parse config file, using defaults", "path", path, "error", err)
		}
	} else {
		slog.Warn("[msggateway] config file not found, using defaults", "path", path)
	}

	// Override from environment variables (for Docker / container deployment).
	if v := os.Getenv("SERVICE_ADDR"); v != "" {
		cfg.ServiceAddr = v
	}
	if v := os.Getenv("ETCD_ENDPOINTS"); v != "" {
		cfg.EtcdEndpoints = strings.Split(v, ",")
	}
	if v := os.Getenv("MSGGW_WS_ADDR"); v != "" {
		cfg.WSAddr = v
	}
	if v := os.Getenv("MSGGW_GRPC_ADDR"); v != "" {
		cfg.GRPCAddr = v
	}
	if v := os.Getenv("MSGGW_METRICS_ADDR"); v != "" {
		cfg.MetricsAddr = v
	}
	if v := os.Getenv("MSGGW_MAX_CONN_PER_USER"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxConnPerUser = n
		}
	}
	if v := os.Getenv("MSGGW_PING_INTERVAL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.PingInterval = d
		}
	}
	if v := os.Getenv("MSGGW_READ_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReadTimeout = d
		}
	}
	if v := os.Getenv("MSGGW_WRITE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.WriteTimeout = d
		}
	}
	if v := os.Getenv("MSGGW_SHUTDOWN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.GracefulShutdownTimeout = d
		}
	}
	return cfg
}
