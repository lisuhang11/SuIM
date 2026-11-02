package logic

import (
	"context"

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
