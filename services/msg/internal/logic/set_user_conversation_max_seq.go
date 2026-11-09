package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// 写下这些人还能看见的历史上限。user_ids 是被限制的人，上下文里的操作者不在其中。
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
