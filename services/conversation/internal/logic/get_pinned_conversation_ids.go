package logic

import (
	"context"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// GetPinnedConversationIDs lists the caller's pinned conversations.
func (s *Server) GetPinnedConversationIDs(ctx context.Context, req *conversationpb.GetPinnedConversationIDsReq) (*conversationpb.GetPinnedConversationIDsResp, error) {
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return &conversationpb.GetPinnedConversationIDsResp{}, nil
	}
	ids, err := s.store.PinnedIDs(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &conversationpb.GetPinnedConversationIDsResp{ConversationIds: ids}, nil
}
