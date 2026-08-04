// Package notification 提供 message 服务的撤回/已读 tip 发送器。
package notification

import (
	"context"

	"SuIM/pkg/notification"
	messagepb "SuIM/proto/messagepb"
)

// MessageNotificationSender 消息 tip 发送器（撤回 2101 / 已读 2200）。
type MessageNotificationSender struct {
	*notification.NotificationSender
}

// NewMessageNotificationSender 创建消息 tip 发送器。
// pushMsg 通常封装 msggateway.OnlinePushMsg。
func NewMessageNotificationSender(
	pushMsg func(ctx context.Context, recvID string, msg *messagepb.MsgData) error,
) *MessageNotificationSender {
	return &MessageNotificationSender{
		NotificationSender: notification.NewNotificationSender(
			notification.WithPushMsg(pushMsg),
		),
	}
}

// RevokeNotification 向 recvIDs 推送撤回 tip。
func (m *MessageNotificationSender) RevokeNotification(
	ctx context.Context,
	revokerUserID string,
	recvIDs []string,
	sessionType int32,
	tips notification.RevokeMsgTips,
) {
	for _, recvID := range recvIDs {
		if recvID == "" {
			continue
		}
		m.NotificationWithSessionType(ctx, revokerUserID, recvID, notification.RevokeNotification, sessionType, tips)
	}
}

// HasReadReceiptNotification 推送已读 tip（单聊→对端；群聊→自己多端）。
func (m *MessageNotificationSender) HasReadReceiptNotification(
	ctx context.Context,
	readerUserID, recvID string,
	sessionType int32,
	tips notification.MarkAsReadTips,
) {
	if recvID == "" {
		return
	}
	m.NotificationWithSessionType(ctx, readerUserID, recvID, notification.HasReadReceipt, sessionType, tips)
}
