package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
)

// CreateGroupChatConversations writes one row per member.
// A repeated call leaves existing rows in place. Each member's visible ceiling is cleared to zero.
func (s *Server) CreateGroupChatConversations(ctx context.Context, req *conversationpb.CreateGroupChatConversationsReq) (*conversationpb.CreateGroupChatConversationsResp, error) {
	groupID := strings.TrimSpace(req.GetGroupId())
	if groupID == "" || len(groupID) > 64 || strings.Contains(groupID, ":") {
		return nil, errorcode.ErrConversationInvalidArgs.WithMsg("group_id is invalid")
	}
	owners, err := normalizeUserIDs(req.GetUserIds())
	if err != nil {
		return nil, err
	}
	kind := req.GetConversationType()
	if kind == 0 {
		kind = model.ConversationGroup
	}
	if kind != model.ConversationGroup && kind != model.ConversationSuperGroup {
		return nil, errorcode.ErrConversationInvalidArgs.WithMsg("conversation_type is invalid")
	}
	conversationID := msgconv.GroupConversationID(groupID, kind == model.ConversationSuperGroup)
	rows := make([]*model.Conversation, 0, len(owners))
	for _, owner := range owners {
		rows = append(rows, &model.Conversation{
			OwnerUserID:      owner,
			ConversationID:   conversationID,
			ConversationType: kind,
			GroupID:          groupID,
		})
	}
	if err := s.store.CreateMissing(ctx, rows, s.now()); err != nil {
		return nil, err
	}
	if s.seq == nil {
		return nil, errorcode.ErrInternal.WithMsg("message seq is not configured")
	}
	if err := s.seq.SetUserMaxSeq(ctx, conversationID, owners, 0); err != nil {
		return nil, err
	}
	return &conversationpb.CreateGroupChatConversationsResp{}, nil
}
