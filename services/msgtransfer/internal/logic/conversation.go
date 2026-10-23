package logic

import (
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
)

// ChatConversationID is the storage id, and the toRedis key, for a normal message.
func ChatConversationID(msg *msgpb.MsgData) string {
	return msgconv.ChatConversationID(msg)
}

// NotificationConversationID is the storage id for a notification message.
func NotificationConversationID(msg *msgpb.MsgData) string {
	return msgconv.NotificationConversationID(msg)
}

func serverMsgID(conversationID, clientMsgID string) (string, error) {
	if clientMsgID == "" {
		return newID()
	}
	return msgconv.ServerMsgID(conversationID, clientMsgID), nil
}
