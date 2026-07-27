// Package config 提供网关配置加载，支持环境变量与 etcd 远程配置。
package config

import (
	"encoding/json"
	"os"
	"strconv"
	"time"
)

// ServiceAddr 描述一个后端 gRPC 服务的地址。
type ServiceAddr struct {
	Name string `json:"name"` // "user" | "relation" | "group" | "conversation" | "message"
	Addr string `json:"addr"` // "127.0.0.1:8080"
}

// RateLimit 描述单个限流级别。
type RateLimit struct {
	Enabled bool    `json:"enabled"`
	Rate    float64 `json:"rate"`  // 每秒允许请求数
	Burst   int     `json:"burst"` // 突发容量
}

// GatewayConfig 网关运行时可热更新的配置。
type GatewayConfig struct {
	ListenAddr string        `json:"listen_addr"` // HTTP 监听地址，默认 :9000
	MetricsAddr string       `json:"metrics_addr"` // Prometheus 指标端口，默认 :9090
	Backends    []ServiceAddr `json:"backends"`    // 后端 gRPC 地址列表
	JWTSecret   string        `json:"jwt_secret"`  // JWT 签名密钥
	RateLimit   RateLimit     `json:"rate_limit"`
	CORSOrigins []string      `json:"cors_origins"`
}

// Default 返回内置默认配置。
func Default() *GatewayConfig {
	return &GatewayConfig{
		ListenAddr:  ":9000",
		MetricsAddr:  ":9090",
		JWTSecret:   "change-me-in-production",
		Backends: []ServiceAddr{
			{Name: "user", Addr: "127.0.0.1:8080"},
			{Name: "relation", Addr: "127.0.0.1:8081"},
			{Name: "group", Addr: "127.0.0.1:8082"},
			{Name: "conversation", Addr: "127.0.0.1:8083"},
			{Name: "message", Addr: "127.0.0.1:8084"},
		},
		RateLimit: RateLimit{
			Enabled: true,
			Rate:    100.0,
			Burst:   200,
		},
		CORSOrigins: []string{"*"},
	}
}

// Load 从环境变量加载配置，未设置则用 Default。
func Load() *GatewayConfig {
	cfg := Default()

	if v := os.Getenv("GATEWAY_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("GATEWAY_METRICS_ADDR"); v != "" {
		cfg.MetricsAddr = v
	}
	if v := os.Getenv("GATEWAY_JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("GATEWAY_USER_ADDR"); v != "" {
		upsertBackend(cfg, "user", v)
	}
	if v := os.Getenv("GATEWAY_RELATION_ADDR"); v != "" {
		upsertBackend(cfg, "relation", v)
	}
	if v := os.Getenv("GATEWAY_GROUP_ADDR"); v != "" {
		upsertBackend(cfg, "group", v)
	}
	if v := os.Getenv("GATEWAY_CONVERSATION_ADDR"); v != "" {
		upsertBackend(cfg, "conversation", v)
	}
	if v := os.Getenv("GATEWAY_MESSAGE_ADDR"); v != "" {
		upsertBackend(cfg, "message", v)
	}
	if v := os.Getenv("GATEWAY_RATE_LIMIT_ENABLED"); v != "" {
		cfg.RateLimit.Enabled = v != "false" && v != "0"
	}
	if v := os.Getenv("GATEWAY_RATE_LIMIT"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.RateLimit.Rate = f
		}
	}
	if v := os.Getenv("GATEWAY_RATE_BURST"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			cfg.RateLimit.Burst = i
		}
	}
	if v := os.Getenv("GATEWAY_CORS_ORIGINS"); v != "" {
		// 逗号分隔，如 "http://a.com,http://b.com"
		origins := []string{}
		for _, o := range splitEnv(v) {
			origins = append(origins, o)
		}
		if len(origins) > 0 {
			cfg.CORSOrigins = origins
		}
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

// BackendAddr 按名称查找后端地址，未找到返回空字符串。
func (c *GatewayConfig) BackendAddr(name string) string {
	for _, b := range c.Backends {
		if b.Name == name {
			return b.Addr
		}
	}
	return ""
}

// Clone 深拷贝配置。
func (c *GatewayConfig) Clone() *GatewayConfig {
	clone := *c
	clone.Backends = make([]ServiceAddr, len(c.Backends))
	copy(clone.Backends, c.Backends)
	clone.CORSOrigins = make([]string, len(c.CORSOrigins))
	copy(clone.CORSOrigins, c.CORSOrigins)
	return &clone
}

// EtcdConfig 描述 etcd 连接参数。
type EtcdConfig struct {
	Endpoints   []string      // etcd 服务器地址列表
	Username    string        // etcd 用户名（可选）
	Password    string        // etcd 密码（可选）
	DialTimeout time.Duration // 连接超时
	ConfigKey   string        // 远程配置 key，默认 /suim/gateway/config
}

// LoadEtcdConfig 从环境变量加载 etcd 连接参数。
func LoadEtcdConfig() *EtcdConfig {
	return &EtcdConfig{
		Endpoints:   envEndpoints("ETCD_ENDPOINTS", []string{"127.0.0.1:2379"}),
		Username:    os.Getenv("ETCD_USERNAME"),
		Password:    os.Getenv("ETCD_PASSWORD"),
		DialTimeout: envDuration("ETCD_DIAL_TIMEOUT", 5*time.Second),
		ConfigKey:   envStr("ETCD_CONFIG_KEY", "/suim/gateway/config"),
	}
}

// ---- 辅助函数 ----

func upsertBackend(cfg *GatewayConfig, name, addr string) {
	for i, b := range cfg.Backends {
		if b.Name == name {
			cfg.Backends[i].Addr = addr
			return
		}
	}
	cfg.Backends = append(cfg.Backends, ServiceAddr{Name: name, Addr: addr})
}

func splitEnv(s string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			result = append(result, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		result = append(result, s[start:])
	}
	return result
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envEndpoints(key string, fallback []string) []string {
	if v := os.Getenv(key); v != "" {
		return splitEnv(v)
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
