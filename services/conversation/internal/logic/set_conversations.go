package logic

import (
	"context"

	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// 给每个 owner 应用同一份部分更新。没有会话行会先创建。min_seq 和 max_seq 被忽略。单聊的 is_private_chat 同时写到对方那一行。
func (s *Server) SetConversations(ctx context.Context, req *conversationpb.SetConversationsReq) (*conversationpb.SetConversationsResp, error) {
	owners, err := normalizeUserIDs(req.GetUserIds())
	if err != nil {
		return nil, err
	}
	for _, owner := range owners {
		if _, err := actorID(ctx, owner); err != nil {
			return nil, err
		}
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
