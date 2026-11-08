package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// GetConversation returns one conversation owned by the caller.
// A missing row is not found. Another user's row is not readable.
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
