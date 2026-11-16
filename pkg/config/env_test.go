package config

import (
	"os"
	"path/filepath"
	"testing"
)

type envTestConfig struct {
	MySQL struct {
		Address  []string `yaml:"address"`
		Password string   `yaml:"password"`
	} `yaml:"mysql"`
	Redis struct {
		Password string `yaml:"password"`
	} `yaml:"redis"`
}

func writeEnvTestFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "sample.yaml")
	body := "mysql:\n  address:\n    - 127.0.0.1:3306\n  password: \"\"\nredis:\n  password: \"\"\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAppliesEnvOverrides(t *testing.T) {
	path := writeEnvTestFile(t)
	t.Setenv("SUIMENV_MYSQL_PASSWORD", "mysql-secret")
	t.Setenv("SUIMENV_MYSQL_ADDRESS", "mysql:3306,mysql-replica:3306")
	t.Setenv("SUIMENV_REDIS_PASSWORD", "redis-secret")
	var cfg envTestConfig
	if err := Load(path, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.Password != "mysql-secret" || cfg.Redis.Password != "redis-secret" {
		t.Fatalf("passwords = %q/%q", cfg.MySQL.Password, cfg.Redis.Password)
	}
	if len(cfg.MySQL.Address) != 2 || cfg.MySQL.Address[0] != "mysql:3306" {
		t.Fatalf("addresses = %v", cfg.MySQL.Address)
	}
}

func TestLoadConfigPathRedirectsDirectory(t *testing.T) {
	original := writeEnvTestFile(t)
	dir := t.TempDir()
	mounted := filepath.Join(dir, "sample.yaml")
	body := "mysql:\n  password: mounted-secret\n"
	if err := os.WriteFile(mounted, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(ConfigPathEnv, dir)
	var cfg envTestConfig
	if err := Load(original, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.Password != "mounted-secret" {
		t.Fatalf("password = %q, want mounted-secret", cfg.MySQL.Password)
	}
}

func TestLoadIgnoresUnknownSection(t *testing.T) {
	path := writeEnvTestFile(t)
	t.Setenv("SUIMENV_LIVEKIT_SECRET", "unused")
	var cfg envTestConfig
	if err := Load(path, &cfg); err != nil {
		t.Fatal(err)
	}
}
