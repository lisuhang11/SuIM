package logic

import (
	"context"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// SetConversations applies one partial update to each owner.
// A missing conversation row is created. min_seq and max_seq are ignored.
// Single-chat is_private_chat is also written on the peer's row.
func (s *Server) SetConversations(ctx context.Context, req *conversationpb.SetConversationsReq) (*conversationpb.SetConversationsResp, error) {
	owners, err := normalizeUserIDs(req.GetUserIds())
	if err != nil {
		return nil, err
	}
	spec, err := conversationSpec(req.GetConversation())
	if err != nil {
		return nil, err
	}
	result, err := s.store.ApplyConversations(ctx, owners, spec, s.now())
	if err != nil {
		return nil, err
	}
	s.notifyChanged(ctx, spec.ConversationID, result.FieldOwners)
	s.notifyPrivate(ctx, spec.ConversationID, result.PrivateNotes)
	return &conversationpb.SetConversationsResp{}, nil
}
