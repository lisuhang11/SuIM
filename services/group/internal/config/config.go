// Package config 提供应用配置，从环境变量中读取各项参数。
package config

import (
	"os"
	"strconv"
)

// Config 保存服务级配置，包括监听地址、user 服务地址和数据库连接信息。
type Config struct {
	// ServerAddr gRPC 监听地址。
	ServerAddr string

	// UserServiceAddr user 服务的 gRPC 地址，用于在变更群组成员前校验引用的用户是否存在。
	UserServiceAddr string

	// 数据库连接参数。
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
}

// Load 从环境变量读取配置，未设置时使用默认值。
func Load() *Config {
	return &Config{
		ServerAddr:      env("SERVER_ADDR", ":8082"),
		UserServiceAddr: env("USER_SERVICE_ADDR", "127.0.0.1:8080"),
		DBHost:          env("DB_HOST", "127.0.0.1"),
		DBPort:          envInt("DB_PORT", 3306),
		DBUser:          env("DB_USER", "root"),
		DBPassword:      env("DB_PASSWORD", ""),
		DBName:          env("DB_NAME", "suim"),
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
