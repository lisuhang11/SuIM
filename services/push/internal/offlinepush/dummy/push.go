package dummy

import (
	"context"
	"log/slog"

	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

// Client 是未配置厂商时的空推送实现。
type Client struct{}

func NewClient() *Client { return &Client{} }

// Push 只记录一条告警并返回成功，不产生任何通知。
func (c *Client) Push(ctx context.Context, userIDs []string, title, content string, opts *options.Opts) error {
	slog.Warn("offline push skipped", "reason", "no provider configured", "users", len(userIDs))
	return nil
}
