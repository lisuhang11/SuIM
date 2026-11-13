package logic

import (
	"context"
	"time"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/conversation/internal/store"
)

// 投递一条通知。发送端为 nil 时不发通知。
type MsgSender interface {
	SendNotification(ctx context.Context, msg *msgpb.MsgData) error
}

// 写消息服务里按人保存的序号下限和上限。
type UserSeq interface {
	SetUserMaxSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error
	SetUserMinSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error
}

// 按发送时间查消息序号，供定时清理使用。
type HistorySeq interface {
	LastSeqByTime(ctx context.Context, conversationID string, timeMs int64) (int64, error)
}

// 会话 gRPC 服务。一行会话只属于它的 owner。
type Server struct {
	conversationpb.UnimplementedConversationServiceServer
	store   *store.Store
	sender  MsgSender
	seq     UserSeq
	history HistorySeq
	now     func() time.Time
}

// 用存储构造会话服务。序号写入和通知稍后注入。
func New(st *store.Store, sender MsgSender) *Server {
	return &Server{store: st, sender: sender, now: time.Now}
}

// 接入消息服务的序号写入。
func (s *Server) UseSeq(seq UserSeq) {
	if s != nil {
		s.seq = seq
	}
}

// 接入消息服务的按时间序号查询。
func (s *Server) UseHistory(history HistorySeq) {
	if s != nil {
		s.history = history
	}
}
