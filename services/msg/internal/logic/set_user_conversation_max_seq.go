package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// SetUserConversationMaxSeq writes each user's history ceiling.
// user_ids are the people being capped. The operator is the caller in the context,
// such as the member who kicked them, and is not one of those ids.
func (s *Server) SetUserConversationMaxSeq(ctx context.Context, req *msgpb.SetUserConversationMaxSeqReq) (*msgpb.SetUserConversationMaxSeqResp, error) {
	conversationID := strings.TrimSpace(req.GetConversationId())
	if conversationID == "" || req.GetMaxSeq() < 0 {
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
		if err := s.store.SetUserMaxSeq(ctx, conversationID, userID, req.GetMaxSeq()); err != nil {
			return nil, errorcode.ErrInternal.WithMsg("set user seq")
		}
	}
	return &msgpb.SetUserConversationMaxSeqResp{}, nil
}
