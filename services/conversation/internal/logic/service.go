package logic

import (
	"context"
	"time"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/conversation/internal/store"
)

// MsgSender publishes one notification. A nil sender skips notices.
type MsgSender interface {
	SendNotification(ctx context.Context, msg *msgpb.MsgData) error
}

// UserSeq writes the per-user sequence floor and ceiling owned by the message service.
type UserSeq interface {
	SetUserMaxSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error
	SetUserMinSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error
}

type Server struct {
	conversationpb.UnimplementedConversationServiceServer
	store  *store.Store
	sender MsgSender
	seq    UserSeq
	now    func() time.Time
}

func New(st *store.Store, sender MsgSender) *Server {
	return &Server{store: st, sender: sender, now: time.Now}
}

// UseSeq attaches the message-service sequence writer.
func (s *Server) UseSeq(seq UserSeq) {
	if s != nil {
		s.seq = seq
	}
}
