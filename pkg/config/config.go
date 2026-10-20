package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type MySQL struct {
	Address     []string `yaml:"address"`
	Username    string   `yaml:"username"`
	Password    string   `yaml:"password"`
	Database    string   `yaml:"database"`
	MaxOpenConn int      `yaml:"maxOpenConn"`
	MaxIdleConn int      `yaml:"maxIdleConn"`
}

type Redis struct {
	Disable      bool     `yaml:"-"`
	Address      []string `yaml:"address"`
	Username     string   `yaml:"username"`
	Password     string   `yaml:"password"`
	RedisMode    string   `yaml:"redisMode"`
	DB           int      `yaml:"db"`
	MaxRetry     int      `yaml:"maxRetry"`
	PoolSize     int      `yaml:"poolSize"`
	SentinelMode Sentinel `yaml:"sentinelMode"`
}

type Sentinel struct {
	MasterName     string   `yaml:"masterName"`
	SentinelAddrs  []string `yaml:"sentinelsAddrs"`
	RouteByLatency bool     `yaml:"routeByLatency"`
	RouteRandomly  bool     `yaml:"routeRandomly"`
}

type JWT struct {
	Secret        string `yaml:"secret"`
	ExpireSeconds int64  `yaml:"expireSeconds"`
}

// Load reads a YAML file into out. Values come only from the file.
func Load(path string, out any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(b, out); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}
