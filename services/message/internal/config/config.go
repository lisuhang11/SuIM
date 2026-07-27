// Package config 提供应用配置读取功能。
package config

import (
	"os"
	"strconv"
)

// Config 服务全局配置。
type Config struct {
	// ServerAddr gRPC 监听地址。
	ServerAddr string

	// PushAddr push 服务 gRPC 地址。
	PushAddr string

	// 数据库 MySQL 连接信息。
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
}

// Load 从环境变量读取配置，提供合理的默认值。
func Load() *Config {
	return &Config{
		ServerAddr: env("SERVER_ADDR", ":8084"),
		PushAddr:   env("PUSH_ADDR", "127.0.0.1:8085"),
		DBHost:     env("DB_HOST", "127.0.0.1"),
		DBPort:     envInt("DB_PORT", 3306),
		DBUser:     env("DB_USER", "root"),
		DBPassword: env("DB_PASSWORD", ""),
		DBName:     env("DB_NAME", "suim"),
	}
}

// DSN 返回 MySQL 数据源名称。
func (c *Config) DSN() string {
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + strconv.Itoa(c.DBPort) + ")/" + c.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}

// env 读取环境变量，不存在则返回默认值。
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt 读取整数型环境变量，不存在或解析失败则返回默认值。
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
