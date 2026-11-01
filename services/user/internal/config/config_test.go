package config

import (
	"testing"

	shared "github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
)

func TestSMSPepperSeparate(t *testing.T) {
	cfg := Config{
		Port: 8080,
		MySQL: shared.MySQL{
			Address:  []string{"127.0.0.1:3306"},
			Username: "suim",
			Database: "suim",
		},
		Redis:    shared.Redis{Address: []string{"127.0.0.1:6379"}},
		JWT:      shared.JWT{Secret: "jwt-secret", ExpireSeconds: 3600},
		Registry: register.Config{Type: "etcd"},
	}
	cfg.SMS.Pepper = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("empty pepper accepted")
	}
	cfg.SMS.Pepper = "jwt-secret"
	if err := cfg.Validate(); err == nil {
		t.Fatal("pepper equal to jwt secret accepted")
	}
	cfg.SMS.Pepper = "  jwt-secret  "
	if err := cfg.Validate(); err == nil {
		t.Fatal("padded pepper equal to jwt secret accepted")
	}
	cfg.SMS.Pepper = "sms-pepper"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
