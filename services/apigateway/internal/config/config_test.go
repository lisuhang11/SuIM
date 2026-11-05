package config

import "testing"

func TestValidateCompressionLevel(t *testing.T) {
	cfg := Config{Port: 1, ListenIP: "127.0.0.1"}
	cfg.Registry.Type = "etcd"
	for _, level := range []int{-1, 0, 1, 2} {
		cfg.CompressionLevel = level
		if err := cfg.Validate(); err != nil {
			t.Fatalf("level %d: %v", level, err)
		}
	}
	cfg.CompressionLevel = 3
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected invalid compression level")
	}
}
