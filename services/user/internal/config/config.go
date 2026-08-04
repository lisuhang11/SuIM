// Package config 提供应用配置，从 YAML 配置文件加载。
package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config 保存服务级配置，包括监听地址、数据库连接信息和 JWT 令牌有效期。
type Config struct {
	ServerAddr       string        `yaml:"server_addr"`
	ServiceAddr      string        `yaml:"service_addr"`
	EtcdEndpoints    []string      `yaml:"etcd_endpoints"`
	DBHost           string        `yaml:"db_host"`
	DBPort           int           `yaml:"db_port"`
	DBUser           string        `yaml:"db_user"`
	DBPassword       string        `yaml:"db_password"`
	DBName           string        `yaml:"db_name"`
	JWTSecret        string        `yaml:"jwt_secret"`
	AccessTokenTTL   time.Duration `yaml:"access_token_ttl"`
	RefreshTokenTTL  time.Duration `yaml:"refresh_token_ttl"`
	RedisAddr        string        `yaml:"redis_addr"`
	RedisPassword    string        `yaml:"redis_password"`
	RedisDB          int           `yaml:"redis_db"`
	UserInfoCacheTTL time.Duration `yaml:"user_info_cache_ttl"`
}

// defaults 返回内置默认配置。
func defaults() *Config {
	return &Config{
		ServerAddr:      ":8080",
		ServiceAddr:     "127.0.0.1:8080",
		EtcdEndpoints:   []string{"127.0.0.1:2379"},
		DBHost:          "127.0.0.1",
		DBPort:          3306,
		DBUser:          "root",
		DBPassword:      "",
		DBName:          "suim",
		JWTSecret:        "change-me-in-production",
		AccessTokenTTL:   24 * time.Hour,
		RefreshTokenTTL:  720 * time.Hour,
		RedisAddr:        "127.0.0.1:6379",
		RedisPassword:    "suim-redis",
		RedisDB:          0,
		UserInfoCacheTTL: 12 * time.Hour,
	}
}

// LoadFromFile 从 YAML 文件加载配置。
func LoadFromFile(path string) *Config {
	cfg := defaults()

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			slog.Warn("[user] failed to parse config file, using defaults", "path", path, "error", err)
		}
	} else {
		slog.Warn("[user] config file not found, using defaults", "path", path)
	}

	// Override from environment variables (for Docker / container deployment).
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
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.AccessTokenTTL = d
		}
	}
	if v := os.Getenv("JWT_REFRESH_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.RefreshTokenTTL = d
		}
	}
	if v := os.Getenv("REDIS_ADDR"); v != "" {
		cfg.RedisAddr = v
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.RedisPassword = v
	}
	if v := os.Getenv("REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.RedisDB = n
		}
	}
	if v := os.Getenv("USER_INFO_CACHE_TTL"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.UserInfoCacheTTL = d
		}
	}
	return cfg
}

// DSN 返回 MySQL 数据源名称。
func (c *Config) DSN() string {
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + strconv.Itoa(c.DBPort) + ")/" + c.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}
