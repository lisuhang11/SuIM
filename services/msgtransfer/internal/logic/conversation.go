package logic

import (
	"crypto/sha256"
	"encoding/hex"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/model"
)

// ChatConversationID is the storage id, and the toRedis key, for a normal message.
func ChatConversationID(msg *msgpb.MsgData) string {
	if msg == nil {
		return ""
	}
	switch msg.SessionType {
	case model.SessionSingle:
		return "si_" + joinSorted(msg.SendId, msg.RecvId)
	case model.SessionWriteGroup:
		return "g_" + msg.GroupId
	case model.SessionReadGroup:
		return "sg_" + msg.GroupId
	case model.SessionNotification:
		return "sn_" + joinSorted(msg.SendId, msg.RecvId)
	default:
		return ""
	}
}

// NotificationConversationID is the storage id for a notification message.
func NotificationConversationID(msg *msgpb.MsgData) string {
	if msg == nil {
		return ""
	}
	switch msg.SessionType {
	case model.SessionSingle, model.SessionNotification:
		return "n_" + joinSorted(msg.SendId, msg.RecvId)
	case model.SessionWriteGroup, model.SessionReadGroup:
		return "n_" + msg.GroupId
	default:
		return ""
	}
}

func joinSorted(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "_" + b
}

func serverMsgID(conversationID, clientMsgID string) (string, error) {
	if clientMsgID == "" {
		return newID()
	}
	sum := sha256.Sum256([]byte(conversationID + "\n" + clientMsgID))
	return hex.EncodeToString(sum[:16]), nil
}
