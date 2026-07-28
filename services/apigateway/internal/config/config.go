// Package config 提供网关配置加载，支持 YAML 文件与 etcd 远程配置。
package config

import (
	"encoding/json"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// RateLimit 描述单个限流级别。
type RateLimit struct {
	Enabled bool    `json:"enabled" yaml:"enabled"`
	Rate    float64 `json:"rate"    yaml:"rate"`  // 每秒允许请求数
	Burst   int     `json:"burst"   yaml:"burst"` // 突发容量
}

// GatewayConfig 网关运行时可热更新的配置。
// 后端服务地址已改为通过 etcd 服务发现解析，不再在此配置。
type GatewayConfig struct {
	ListenAddr  string    `json:"listen_addr"  yaml:"listen_addr"`  // HTTP 监听地址，默认 :9000
	MetricsAddr string    `json:"metrics_addr" yaml:"metrics_addr"` // Prometheus 指标端口，默认 :9090
	JWTSecret   string    `json:"jwt_secret"   yaml:"jwt_secret"`   // JWT 签名密钥
	ServiceAddr string    `json:"service_addr" yaml:"service_addr"` // 注册到 etcd 的地址
	RateLimit   RateLimit `json:"rate_limit"   yaml:"rate_limit"`
	CORSOrigins []string  `json:"cors_origins" yaml:"cors_origins"`

	// etcd 连接参数。
	EtcdEndpoints   []string      `json:"etcd_endpoints"    yaml:"etcd_endpoints"`
	EtcdUsername    string        `json:"etcd_username"     yaml:"etcd_username"`
	EtcdPassword    string        `json:"etcd_password"     yaml:"etcd_password"`
	EtcdDialTimeout time.Duration `json:"etcd_dial_timeout" yaml:"etcd_dial_timeout"`
	EtcdConfigKey   string        `json:"etcd_config_key"   yaml:"etcd_config_key"`
}

// Default 返回内置默认配置。
func Default() *GatewayConfig {
	return &GatewayConfig{
		ListenAddr:      ":9000",
		MetricsAddr:     ":9090",
		JWTSecret:       "change-me-in-production",
		ServiceAddr:     "127.0.0.1:9000",
		RateLimit:       RateLimit{Enabled: true, Rate: 100.0, Burst: 200},
		CORSOrigins:     []string{"*"},
		EtcdEndpoints:   []string{"127.0.0.1:2379"},
		EtcdDialTimeout: 5 * time.Second,
		EtcdConfigKey:   "/suim/gateway/config",
	}
}

// LoadFromFile 从 YAML 文件加载配置。
func LoadFromFile(path string) *GatewayConfig {
	cfg := Default()

	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			slog.Warn("[gateway] failed to parse config file, using defaults", "path", path, "error", err)
		}
	} else {
		slog.Warn("[gateway] config file not found, using defaults", "path", path)
	}

	// Override from environment variables (for Docker / container deployment).
	if v := os.Getenv("SERVICE_ADDR"); v != "" {
		cfg.ServiceAddr = v
	}
	if v := os.Getenv("ETCD_ENDPOINTS"); v != "" {
		cfg.EtcdEndpoints = strings.Split(v, ",")
	}
	if v := os.Getenv("GATEWAY_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("GATEWAY_METRICS_ADDR"); v != "" {
		cfg.MetricsAddr = v
	}
	if v := os.Getenv("GATEWAY_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("GATEWAY_RATE_LIMIT_ENABLED"); v != "" {
		cfg.RateLimit.Enabled = v == "true" || v == "1"
	}
	if v := os.Getenv("GATEWAY_RATE_LIMIT"); v != "" {
		if r, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.RateLimit.Rate = r
		}
	}
	if v := os.Getenv("GATEWAY_RATE_BURST"); v != "" {
		if b, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit.Burst = b
		}
	}
	if v := os.Getenv("GATEWAY_CORS_ORIGINS"); v != "" {
		cfg.CORSOrigins = strings.Split(v, ",")
	}
	return cfg
}

// FromJSON 从 JSON 字节解析配置（供 etcd watch 使用）。
func FromJSON(data []byte) (*GatewayConfig, error) {
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ToJSON 将配置序列化为 JSON。
func (c *GatewayConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}

// Clone 深拷贝配置。
func (c *GatewayConfig) Clone() *GatewayConfig {
	clone := *c
	clone.CORSOrigins = make([]string, len(c.CORSOrigins))
	copy(clone.CORSOrigins, c.CORSOrigins)
	return &clone
}

// ToEtcdConfig 从网关配置提取 etcd 连接参数。
func (c *GatewayConfig) ToEtcdConfig() *EtcdConfig {
	return &EtcdConfig{
		Endpoints:   c.EtcdEndpoints,
		Username:    c.EtcdUsername,
		Password:    c.EtcdPassword,
		DialTimeout: c.EtcdDialTimeout,
		ConfigKey:   c.EtcdConfigKey,
	}
}

// EtcdConfig 描述 etcd 连接参数。
type EtcdConfig struct {
	Endpoints   []string      // etcd 服务器地址列表
	Username    string        // etcd 用户名（可选）
	Password    string        // etcd 密码（可选）
	DialTimeout time.Duration // 连接超时
	ConfigKey   string        // 远程配置 key，默认 /suim/gateway/config
}
