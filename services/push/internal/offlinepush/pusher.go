package offlinepush

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/services/push/internal/config"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/dummy"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/fcm"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/getui"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/jpush"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

// Pusher 把一条通知送到一家离线推送厂商。
type Pusher interface {
	Push(ctx context.Context, userIDs []string, title, content string, opts *options.Opts) error
}

// NewPusher 按 enable 配置选出一家厂商，未识别的取值返回空实现。
func NewPusher(cfg *config.Push, tokens options.Tokens) (Pusher, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Enable)) {
	case "getui":
		return getui.NewClient(cfg, tokens), nil
	case "fcm":
		client, err := fcm.NewClient(cfg, tokens)
		if err != nil {
			return nil, err
		}
		return client, nil
	case "jpush":
		return jpush.NewClient(cfg), nil
	default:
		return dummy.NewClient(), nil
	}
}
