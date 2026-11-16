package config

import (
	"testing"
	"time"
)

func TestRateLimiterValidateDefaults(t *testing.T) {
	var cfg RateLimiter
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Window != 20*time.Second || cfg.Bucket != 500 || cfg.CPUThreshold != 850 {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestRateLimiterValidateRejectsInvalid(t *testing.T) {
	tests := []RateLimiter{
		{Window: time.Microsecond, Bucket: 500, CPUThreshold: 850},
		{Window: time.Second, Bucket: -1, CPUThreshold: 850},
		{Window: time.Second, Bucket: 10, CPUThreshold: 1001},
	}
	for i, cfg := range tests {
		if err := cfg.Validate(); err == nil {
			t.Fatalf("case %d: expected error", i)
		}
	}
}
