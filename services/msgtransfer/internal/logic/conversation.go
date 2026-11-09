package logic

import (
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// 普通消息的存储 ID，也是 toRedis 的键。
func ChatConversationID(msg *msgpb.MsgData) string {
	return msgconv.ChatConversationID(msg)
}

// 通知消息的存储 ID。
func NotificationConversationID(msg *msgpb.MsgData) string {
	return msgconv.NotificationConversationID(msg)
}

func serverMsgID(conversationID, clientMsgID string) (string, error) {
	if clientMsgID == "" {
		return newID()
	}
	return msgconv.ServerMsgID(conversationID, clientMsgID), nil
}
