package logic

import (
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

func seqTarget(conversationID string, userIDs []string, seq int64) (string, []string, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" || len(conversationID) > 255 || seq < 0 {
		return "", nil, errorcode.ErrConversationInvalidArgs.WithMsg("conversation seq is invalid")
	}
	owners, err := normalizeUserIDs(userIDs)
	if err != nil {
		return "", nil, err
	}
	return conversationID, owners, nil
}
