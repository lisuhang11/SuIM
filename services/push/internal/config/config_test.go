package config

import "testing"

func TestValidateDefaults(t *testing.T) {
	var c Config
	c.Redis.Address = []string{"127.0.0.1:6379"}
	c.Registry.Type = "etcd"
	if err := c.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if c.Port != 10180 || c.Queue.StreamPrefix != "mq" || c.Queue.ConsumerGroup != "push" {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.Push.Enable != "dummy" || c.Push.MaxConcurrentWorkers != 8 {
		t.Fatalf("unexpected push defaults: %+v", c.Push)
	}
}

func TestValidateRequiresRedisAndRegistry(t *testing.T) {
	var c Config
	if err := c.Validate(); err == nil {
		t.Fatal("expected redis error")
	}
	c.Redis.Address = []string{"127.0.0.1:6379"}
	if err := c.Validate(); err == nil {
		t.Fatal("expected registry error")
	}
}

func TestValidateWebhookURL(t *testing.T) {
	var c Config
	c.Redis.Address = []string{"127.0.0.1:6379"}
	c.Registry.Type = "etcd"
	c.Webhooks.BeforeOfflinePush.Enable = true
	if err := c.Validate(); err == nil {
		t.Fatal("expected webhook url error")
	}
}
