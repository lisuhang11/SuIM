package config

import (
	"testing"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
)

func validConfig() Config {
	return Config{
		Port: 8083,
		MySQL: config.MySQL{
			Address:  []string{"127.0.0.1:3306"},
			Username: "suim",
			Database: "suim",
		},
		Redis: config.Redis{Address: []string{"127.0.0.1:6379"}},
		LocalCache: LocalCache{
			Topic:         "suim:group:cache:delete",
			SlotNum:       100,
			SlotSize:      2000,
			SuccessExpire: 300,
			FailedExpire:  5,
		},
		Registry: register.Config{Type: "etcd"},
	}
}

func TestMemberLimitDefaultsAndRejects(t *testing.T) {
	cfg := validConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.MemberLimit.Normal != 200 || cfg.MemberLimit.Super != 1000 {
		t.Fatalf("defaults %+v", cfg.MemberLimit)
	}

	negative := validConfig()
	negative.MemberLimit = MemberLimit{Normal: -1, Super: 1000}
	if err := negative.Validate(); err == nil {
		t.Fatal("negative normal was accepted")
	}

	inverted := validConfig()
	inverted.MemberLimit = MemberLimit{Normal: 500, Super: 200}
	if err := inverted.Validate(); err == nil {
		t.Fatal("super below normal was accepted")
	}
}
