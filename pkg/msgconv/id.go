package msgconv

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const optNotNotification = "isNotNotification"

// ChatConversationID is the storage id of a normal message.
func ChatConversationID(msg *msgpb.MsgData) string {
	if msg == nil {
		return ""
	}
	switch msg.SessionType {
	case 1:
		return "si_" + joinSorted(msg.SendId, msg.RecvId)
	case 2:
		return "g_" + msg.GroupId
	case 3:
		return "sg_" + msg.GroupId
	case 4:
		return "sn_" + joinSorted(msg.SendId, msg.RecvId)
	default:
		return ""
	}
}

// NotificationConversationID is the storage id of a notification.
func NotificationConversationID(msg *msgpb.MsgData) string {
	if msg == nil {
		return ""
	}
	switch msg.SessionType {
	case 1, 4:
		return "n_" + joinSorted(msg.SendId, msg.RecvId)
	case 2, 3:
		return "n_" + msg.GroupId
	default:
		return ""
	}
}

// QueueKey is the toRedis key. Notifications and chat messages stay on separate keys.
func QueueKey(msg *msgpb.MsgData) string {
	if IsNotification(msg) {
		return NotificationConversationID(msg)
	}
	return ChatConversationID(msg)
}

// IsNotification reports a message whose isNotNotification option is explicitly false.
func IsNotification(msg *msgpb.MsgData) bool {
	if msg == nil {
		return false
	}
	v, ok := msg.Options[optNotNotification]
	return ok && !v
}

// IsNotificationConversation reports ids produced by NotificationConversationID.
func IsNotificationConversation(conversationID string) bool {
	return strings.HasPrefix(conversationID, "n_")
}

// ServerMsgID is stable for a client message id, so a retry keeps the same id.
func ServerMsgID(conversationID, clientMsgID string) string {
	sum := sha256.Sum256([]byte(conversationID + "\n" + clientMsgID))
	return hex.EncodeToString(sum[:16])
}

// GroupConversationID maps a SuIM group type onto the conversation id prefix.
// Normal groups use g_, super groups use sg_.
func GroupConversationID(groupID string, super bool) string {
	if super {
		return "sg_" + groupID
	}
	return "g_" + groupID
}

func joinSorted(a, b string) string {
	if a > b {
		a, b = b, a
	}
	return a + "_" + b
}
