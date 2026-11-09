package logic

import (
	"context"
	"log/slog"

	"github.com/lisuhang11/SuIM/pkg/msgconv"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// 保存用户退群后仍能读到的序号上限。
type SeqClient interface {
	ConversationMaxSeq(ctx context.Context, conversationID string) (int64, error)
	SetConversationMaxSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error
}

// 为每名成员写一行群会话。
type GroupConversations interface {
	CreateGroup(ctx context.Context, groupID string, userIDs []string, conversationType int32) error
}

func (s *Server) createGroupConversations(ctx context.Context, group *model.Group, userIDs []string) {
	if s == nil || s.convs == nil || group == nil || len(userIDs) == 0 {
		return
	}
	kind := int32(2)
	if group.GroupType == model.GroupTypeSuper {
		kind = 3
	}
	if err := s.convs.CreateGroup(ctx, group.GroupID, userIDs, kind); err != nil {
		slog.Warn("create group conversations", "err", err, "groupID", group.GroupID)
	}
}

func memberUserIDs(members []*model.Member) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		if member != nil && member.UserID != "" {
			ids = append(ids, member.UserID)
		}
	}
	return ids
}

func groupConversationID(group *model.Group) string {
	if group == nil {
		return ""
	}
	return msgconv.GroupConversationID(group.GroupID, group.GroupType == model.GroupTypeSuper)
}

// 把当前最大序号记成刚离群的人的可见上限。再次入群会清掉。解散不走这里，留下的成员仍能看全部历史。
func (s *Server) freezeHistory(ctx context.Context, group *model.Group, userIDs []string) error {
	if s == nil || s.seq == nil || len(userIDs) == 0 {
		return nil
	}
	conversationID := groupConversationID(group)
	if conversationID == "" {
		return nil
	}
	maxSeq, err := s.seq.ConversationMaxSeq(ctx, conversationID)
	if err != nil {
		return err
	}
	return s.seq.SetConversationMaxSeq(ctx, conversationID, userIDs, maxSeq)
}

// 清掉个人可见上限，这名成员重新跟随会话。
func (s *Server) reopenHistory(ctx context.Context, group *model.Group, userIDs []string) error {
	if s == nil || s.seq == nil || len(userIDs) == 0 {
		return nil
	}
	conversationID := groupConversationID(group)
	if conversationID == "" {
		return nil
	}
	return s.seq.SetConversationMaxSeq(ctx, conversationID, userIDs, 0)
}
