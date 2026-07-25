// Package config provides application configuration.
package config

import (
	"os"
	"strconv"
)

// Config holds the service-wide configuration.
type Config struct {
	// Server is the gRPC listen address.
	ServerAddr string

	// Database holds MySQL connection info.
	DBHost     string
	DBPort     int
	DBUser     string
	DBPassword string
	DBName     string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	return &Config{
		ServerAddr: env("SERVER_ADDR", ":8080"),
		DBHost:     env("DB_HOST", "127.0.0.1"),
		DBPort:     envInt("DB_PORT", 3306),
		DBUser:     env("DB_USER", "root"),
		DBPassword: env("DB_PASSWORD", ""),
		DBName:     env("DB_NAME", "suim"),
	}
}

// DSN returns a MySQL data source name.
func (c *Config) DSN() string {
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + strconv.Itoa(c.DBPort) + ")/" + c.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
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
