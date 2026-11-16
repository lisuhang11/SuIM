package options

import (
	"context"
	"time"
)

// Signal 携带一次推送对应的消息标识。
type Signal struct {
	ClientMsgID string
}

// Opts 是厂商无关的通知选项。
type Opts struct {
	Signal        *Signal
	Ex            string
	IOSPushSound  string
	IOSBadgeCount bool
}

// Tokens 提供各厂商需要的设备 token 与角标。
type Tokens interface {
	FCMTokens(ctx context.Context, userID string) ([]string, error)
	Badge(ctx context.Context, userID string) (int64, error)
	IncrBadge(ctx context.Context, userID string, delta int64) (int64, error)
	GetuiToken(ctx context.Context) (string, error)
	SetGetuiToken(ctx context.Context, token string, ttl time.Duration) error
}
