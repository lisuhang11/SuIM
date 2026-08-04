package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerAddr          string        `yaml:"server_addr"`
	ServiceAddr         string        `yaml:"service_addr"`
	EtcdEndpoints       []string      `yaml:"etcd_endpoints"`
	DBHost              string        `yaml:"db_host"`
	DBPort              int           `yaml:"db_port"`
	DBUser              string        `yaml:"db_user"`
	DBPassword          string        `yaml:"db_password"`
	DBName              string        `yaml:"db_name"`
	JWTSecret           string        `yaml:"jwt_secret"`
	MinioEndpoint       string        `yaml:"minio_endpoint"`
	MinioPublicEndpoint string        `yaml:"minio_public_endpoint"`
	MinioAccessKey      string        `yaml:"minio_access_key"`
	MinioSecretKey      string        `yaml:"minio_secret_key"`
	MinioBucket         string        `yaml:"minio_bucket"`
	MinioUseSSL         bool          `yaml:"minio_use_ssl"`
	MaxFileSize         int64         `yaml:"max_file_size"`
	UserQuota           int64         `yaml:"user_quota"`
	UploadExpiry        time.Duration `yaml:"upload_expiry"`
	DownloadExpiry      time.Duration `yaml:"download_expiry"`
	PendingRetention    time.Duration `yaml:"pending_retention"`
	FileRetention       time.Duration `yaml:"file_retention"`
	CleanupInterval     time.Duration `yaml:"cleanup_interval"`
}

func defaults() *Config {
	return &Config{ServerAddr: ":8086", ServiceAddr: "127.0.0.1:8086", EtcdEndpoints: []string{"127.0.0.1:2379"}, DBHost: "127.0.0.1", DBPort: 3306, DBUser: "root", DBName: "suim", JWTSecret: "change-me-in-production", MinioEndpoint: "127.0.0.1:10005", MinioPublicEndpoint: "127.0.0.1:10005", MinioAccessKey: "suim", MinioSecretKey: "suim-file-secret", MinioBucket: "suim-files", MaxFileSize: 100 << 20, UserQuota: 10 << 30, UploadExpiry: 15 * time.Minute, DownloadExpiry: 5 * time.Minute, PendingRetention: 24 * time.Hour, FileRetention: 180 * 24 * time.Hour, CleanupInterval: time.Hour}
}

func LoadFromFile(path string) *Config {
	cfg := defaults()
	if data, err := os.ReadFile(path); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			slog.Warn("[file] invalid config", "error", err)
		}
	}
	set := func(key string, dst *string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	set("SERVER_ADDR", &cfg.ServerAddr)
	set("SERVICE_ADDR", &cfg.ServiceAddr)
	set("DB_HOST", &cfg.DBHost)
	set("DB_USER", &cfg.DBUser)
	set("DB_PASSWORD", &cfg.DBPassword)
	set("DB_NAME", &cfg.DBName)
	set("JWT_SECRET", &cfg.JWTSecret)
	set("MINIO_ENDPOINT", &cfg.MinioEndpoint)
	set("MINIO_PUBLIC_ENDPOINT", &cfg.MinioPublicEndpoint)
	set("MINIO_ACCESS_KEY", &cfg.MinioAccessKey)
	set("MINIO_SECRET_KEY", &cfg.MinioSecretKey)
	set("MINIO_BUCKET", &cfg.MinioBucket)
	if v := os.Getenv("ETCD_ENDPOINTS"); v != "" {
		cfg.EtcdEndpoints = strings.Split(v, ",")
	}
	if v, err := strconv.Atoi(os.Getenv("DB_PORT")); err == nil && v > 0 {
		cfg.DBPort = v
	}
	if v, err := strconv.ParseInt(os.Getenv("FILE_MAX_SIZE"), 10, 64); err == nil && v > 0 {
		cfg.MaxFileSize = v
	}
	if v, err := strconv.ParseInt(os.Getenv("FILE_USER_QUOTA"), 10, 64); err == nil && v > 0 {
		cfg.UserQuota = v
	}
	if v := os.Getenv("MINIO_USE_SSL"); v != "" {
		cfg.MinioUseSSL = v == "true" || v == "1"
	}
	parseDuration("FILE_UPLOAD_EXPIRY", &cfg.UploadExpiry)
	parseDuration("FILE_DOWNLOAD_EXPIRY", &cfg.DownloadExpiry)
	parseDuration("FILE_PENDING_RETENTION", &cfg.PendingRetention)
	parseDuration("FILE_RETENTION", &cfg.FileRetention)
	parseDuration("FILE_CLEANUP_INTERVAL", &cfg.CleanupInterval)
	return cfg
}

func parseDuration(key string, dst *time.Duration) {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			*dst = d
		}
	}
}
func (c *Config) DSN() string {
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + strconv.Itoa(c.DBPort) + ")/" + c.DBName + "?charset=utf8mb4&parseTime=True&loc=Local"
}
