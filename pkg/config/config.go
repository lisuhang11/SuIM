package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigPathEnv points at a mounted config directory. When set, Load reads
// the base name of path from that directory instead of the original location.
const ConfigPathEnv = "CONFIG_PATH"

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
	Secret              string `yaml:"secret"`
	ExpireSeconds       int64  `yaml:"expireSeconds"`       // refresh TTL
	AccessExpireSeconds int64  `yaml:"accessExpireSeconds"` // access TTL; 0 → 3600
	Issuer              string `yaml:"issuer"`
	Audience            string `yaml:"audience"`
}

// Load reads a YAML file into out. CONFIG_PATH can redirect the file to a
// mounted directory, and SUIMENV_* variables override selected values.
func Load(path string, out any) error {
	if dir := strings.TrimSpace(os.Getenv(ConfigPathEnv)); dir != "" {
		path = filepath.Join(dir, filepath.Base(path))
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(b, &raw); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	applyEnvOverrides(raw)
	merged, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("encode config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(merged, out); err != nil {
		return fmt.Errorf("parse config %s: %w", path, err)
	}
	return nil
}
