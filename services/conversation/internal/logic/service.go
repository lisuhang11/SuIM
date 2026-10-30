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

type Server struct {
	conversationpb.UnimplementedConversationServiceServer
	store  *store.Store
	sender MsgSender
	now    func() time.Time
}

func New(st *store.Store, sender MsgSender) *Server {
	return &Server{store: st, sender: sender, now: time.Now}
}
