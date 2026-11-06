package livekit

import (
	"strings"
	"testing"
	"time"
)

func TestSignMicrophoneToken(t *testing.T) {
	cfg := Config{URL: "ws://127.0.0.1:7880", Key: "suim", Secret: strings.Repeat("k", 32)}
	now := time.Unix(1_700_000_000, 0)
	url, token, err := cfg.Sign("user-a", "room-1", now)
	if err != nil || url != cfg.URL || strings.Count(token, ".") != 2 {
		t.Fatalf("url %s token %s err %v", url, token, err)
	}
	identity, room, err := Parse(cfg.Secret, token)
	if err != nil || identity != "user-a" || room != "room-1" {
		t.Fatalf("parsed %s %s err %v", identity, room, err)
	}
	if _, _, err := Parse(strings.Repeat("x", 32), token); err == nil {
		t.Fatal("wrong secret was accepted")
	}
	if _, _, err := cfg.Sign("", "room-1", now); err == nil {
		t.Fatal("empty identity was accepted")
	}
}

func TestValidateRequiresAllFields(t *testing.T) {
	if err := (Config{}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Config{URL: "ws://127.0.0.1:7880"}).Validate(); err == nil {
		t.Fatal("partial config was accepted")
	}
	if err := (Config{URL: "ws://127.0.0.1:7880", Key: "suim", Secret: "short"}).Validate(); err == nil {
		t.Fatal("short secret was accepted")
	}
}
