// Package config 提供应用配置，从环境变量中读取各项参数。
package config

import (
	"os"
	"strconv"
	"time"
)

// Config 保存服务级配置，包括监听地址、数据库连接信息和 JWT 令牌有效期。
type Config struct {
	// ServerAddr gRPC 监听地址。
	ServerAddr string

	// 数据库连接参数。
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string

	// JWT 令牌有效期。
	AccessTokenTTL  time.Duration // 访问令牌有效期，默认 24h
	RefreshTokenTTL time.Duration // 刷新令牌有效期，默认 720h（30 天）
}

// Load 从环境变量读取配置，未设置时使用默认值。
func Load() *Config {
	return &Config{
		ServerAddr:      env("SERVER_ADDR", ":8080"),
		DBHost:          env("DB_HOST", "127.0.0.1"),
		DBPort:          envInt("DB_PORT", 3306),
		DBUser:          env("DB_USER", "root"),
		DBPassword:      env("DB_PASSWORD", ""),
		DBName:          env("DB_NAME", "suim"),
		AccessTokenTTL:  envDuration("JWT_ACCESS_TTL", 24*time.Hour),
		RefreshTokenTTL: envDuration("JWT_REFRESH_TTL", 720*time.Hour), // 30 天
	}
}

// DSN 返回 MySQL 数据源名称。
func (c *Config) DSN() string {
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + strconv.Itoa(c.DBPort) + ")/" + c.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}

// env 获取字符串环境变量，未设置则返回默认值。
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt 获取整数环境变量，未设置则返回默认值。
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

// envDuration 获取 duration 类型的环境变量（支持 "1h"、"30m" 等格式），未设置或格式错误则返回默认值。
func envDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
