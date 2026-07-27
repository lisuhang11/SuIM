// Package config 从环境变量加载 msg gateway 服务配置。
package config

import (
	"os"
	"strconv"
	"time"
)

// Config 服务全局配置。
type Config struct {
	// WSAddr WebSocket 监听地址（客户端连接）。
	WSAddr string
	// GRPCAddr gRPC 监听地址（内部服务调用）。
	GRPCAddr string
	// MetricsAddr Prometheus 指标端口。
	MetricsAddr string

	// MaxConnPerUser 单用户最大连接数（跨平台），0 表示不限。
	MaxConnPerUser int
	// PingInterval WebSocket ping 间隔。
	PingInterval time.Duration
	// ReadTimeout WebSocket 读超时。
	ReadTimeout time.Duration
	// WriteTimeout WebSocket 写超时。
	WriteTimeout time.Duration
	// GracefulShutdownTimeout 优雅关闭超时。
	GracefulShutdownTimeout time.Duration
}

// Load 从环境变量读取配置，提供默认值。
func Load() *Config {
	return &Config{
		WSAddr:                  env("MSGGW_WS_ADDR", ":9001"),
		GRPCAddr:                env("MSGGW_GRPC_ADDR", ":9091"),
		MetricsAddr:             env("MSGGW_METRICS_ADDR", ":9092"),
		MaxConnPerUser:          envInt("MSGGW_MAX_CONN_PER_USER", 10),
		PingInterval:            envDuration("MSGGW_PING_INTERVAL", 30*time.Second),
		ReadTimeout:             envDuration("MSGGW_READ_TIMEOUT", 60*time.Second),
		WriteTimeout:            envDuration("MSGGW_WRITE_TIMEOUT", 10*time.Second),
		GracefulShutdownTimeout: envDuration("MSGGW_SHUTDOWN_TIMEOUT", 30*time.Second),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
