package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversation"
)

// 返回调用者自己的一行会话。没有这一行是未找到，别人的行不能读。
func (s *Server) GetConversation(ctx context.Context, req *conversationpb.GetConversationReq) (*conversationpb.GetConversationResp, error) {
	owner, err := actorID(ctx, req.GetOwnerUserId())
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(req.GetConversationId())
	if id == "" {
		return nil, errorcode.ErrConversationInvalidArgs.WithMsg("conversation id is required")
	}
	if s.store == nil {
		return nil, errorcode.ErrConversationNotFound
	}
	rows, err := s.store.ConversationsByID(ctx, owner, []string{id})
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversation")
	}
	if len(rows) == 0 || rows[0] == nil {
		return nil, errorcode.ErrConversationNotFound
	}
	return &conversationpb.GetConversationResp{Conversation: conversationPB(rows[0])}, nil
}
