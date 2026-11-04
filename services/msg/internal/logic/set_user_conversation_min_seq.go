package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// SetUserConversationMinSeq writes each user's history floor.
// The conversation service calls this, then writes conversation.min_seq itself.
func (s *Server) SetUserConversationMinSeq(ctx context.Context, req *msgpb.SetUserConversationMinSeqReq) (*msgpb.SetUserConversationMinSeqResp, error) {
	conversationID := strings.TrimSpace(req.GetConversationId())
	if conversationID == "" || req.GetMinSeq() < 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("conversation seq is invalid")
	}
	if s.store == nil {
		return nil, errorcode.ErrInternal.WithMsg("message store is not configured")
	}
	seen := map[string]struct{}{}
	for _, raw := range req.GetUserIds() {
		userID := strings.TrimSpace(raw)
		if userID == "" {
			return nil, errorcode.ErrMsgInvalidArgs.WithMsg("user id is invalid")
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		if err := s.store.SetUserMinSeq(ctx, conversationID, userID, req.GetMinSeq()); err != nil {
			return nil, errorcode.ErrInternal.WithMsg("set user seq")
		}
	}
	return &msgpb.SetUserConversationMinSeqResp{}, nil
}
