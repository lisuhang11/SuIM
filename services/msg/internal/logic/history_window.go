package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

// groupIDFromConversation reports the group id carried by a group or group-notification conversation.
func groupIDFromConversation(conversationID string) (string, bool) {
	switch {
	case strings.HasPrefix(conversationID, "sg_"):
		id := strings.TrimPrefix(conversationID, "sg_")
		return id, id != ""
	case strings.HasPrefix(conversationID, "g_"):
		id := strings.TrimPrefix(conversationID, "g_")
		return id, id != ""
	case strings.HasPrefix(conversationID, "n_"):
		id := strings.TrimPrefix(conversationID, "n_")
		return id, id != "" && !strings.Contains(id, "_")
	default:
		return "", false
	}
}

// visibleSeqs is the sequence window this user may read.
// A current member follows the conversation, tightened by a personal minimum or ceiling.
// Someone who has left can read only up to the ceiling stored when they left.
// Zero is not a ceiling, so a stranger with no row cannot read the group.
func (s *Server) visibleSeqs(ctx context.Context, userID, conversationID string) (int64, int64, error) {
	groupID, group := groupIDFromConversation(conversationID)
	if !group {
		if !canRead(conversationID, userID) {
			return 0, 0, errorcode.ErrNoPermission
		}
		if s.store == nil {
			return 0, 0, nil
		}
		return s.store.SeqBounds(ctx, userID, conversationID)
	}
	inGroup, err := s.stillInGroup(ctx, groupID, userID)
	if err != nil {
		return 0, 0, err
	}
	var userMax int64
	if s.store != nil {
		userMax, err = s.store.UserMaxSeq(ctx, userID, conversationID)
		if err != nil {
			return 0, 0, errorcode.ErrInternal.WithMsg("query user seq")
		}
	}
	if !inGroup && userMax <= 0 {
		return 0, 0, errorcode.ErrNoPermission
	}
	if s.store == nil {
		return 0, 0, nil
	}
	return s.store.SeqBounds(ctx, userID, conversationID)
}

func (s *Server) stillInGroup(ctx context.Context, groupID, userID string) (bool, error) {
	if s.groups == nil {
		return false, nil
	}
	speaker, err := s.groups.Speaker(ctx, groupID, userID)
	if err != nil {
		return false, errorcode.ErrInternal.WithMsg("check group")
	}
	return speaker.InGroup, nil
}
