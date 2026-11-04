package logic

import (
	"context"
	"log/slog"

	"github.com/lisuhang11/SuIM/pkg/msgconv"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

// SeqClient stores the sequence ceiling a user can still read after leaving a group.
type SeqClient interface {
	ConversationMaxSeq(ctx context.Context, conversationID string) (int64, error)
	SetConversationMaxSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error
}

func (s *Server) UseSeq(seq SeqClient) {
	if s != nil {
		s.seq = seq
	}
}

// GroupConversations creates one conversation row per member.
type GroupConversations interface {
	CreateGroup(ctx context.Context, groupID string, userIDs []string, conversationType int32) error
}

func (s *Server) UseConversations(convs GroupConversations) {
	if s != nil {
		s.convs = convs
	}
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

// freezeHistory records the conversation's current max seq as the ceiling for people who just left.
// A later rejoin clears it. Dismiss does not call this, so remaining members keep the full history.
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

// reopenHistory clears a personal ceiling so a member follows the conversation again.
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
