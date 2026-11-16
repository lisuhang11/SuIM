package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/webhook"
	pushpb "github.com/lisuhang11/SuIM/proto/push"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush/options"
)

// Members 读取群成员 userID，userID 是发起这次读取的操作者。
type Members interface {
	UserIDs(ctx context.Context, groupID, userID string) ([]string, error)
}

// OnlineStatus 查询一个用户当前在线的平台，空列表表示离线。
type OnlineStatus interface {
	Get(ctx context.Context, userID string) ([]int32, error)
}

// Tokens 管理推送 token 与角标。
type Tokens interface {
	options.Tokens
	SetFCMToken(ctx context.Context, userID string, platformID int32, token string) error
	DelFCMToken(ctx context.Context, userID string, platformID int32) error
	SetBadge(ctx context.Context, userID string, count int64) error
}

// Hooks 控制推送前的同步回调。
type Hooks struct {
	Client            *webhook.Client
	BeforeOfflinePush webhook.BeforeConfig
}

// Server 管理设备推送状态，同时消费 toPush 分流在线与离线通知。
type Server struct {
	pushpb.UnimplementedPushServiceServer
	members Members
	online  OnlineStatus
	pusher  offlinepush.Pusher
	tokens  Tokens
	hooks   Hooks
}

// New 用群成员、在线状态、离线厂商和 token 存储构造推送服务。
func New(members Members, online OnlineStatus, pusher offlinepush.Pusher, tokens Tokens) *Server {
	return &Server{members: members, online: online, pusher: pusher, tokens: tokens}
}

// UseHooks 接入 webhook 开关，客户端为 nil 时跳过全部回调。
func (s *Server) UseHooks(hooks Hooks) {
	if s != nil {
		s.hooks = hooks
	}
}
