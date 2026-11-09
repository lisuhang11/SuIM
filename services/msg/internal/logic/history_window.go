package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

// 从群会话或群通知会话的 ID 里取出群 ID。单聊和带下划线的通知 ID 不算群。
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

// 这个用户能读的序号窗口。在群成员跟随会话，并受个人上下限收紧。已退群的人只能读到离群时记下的上限。0 不是上限，没有记录的外人读不到。
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
