// Package notification 提供 rtc 服务的通话 tip 发送器。
package notification

import (
	"context"

	"SuIM/pkg/notification"
	messagepb "SuIM/proto/messagepb"
)

// CallNotificationSender 通话 tip 发送器。
type CallNotificationSender struct {
	*notification.NotificationSender
}

// NewCallNotificationSender 创建通话 tip 发送器。
func NewCallNotificationSender(
	pushMsg func(ctx context.Context, recvID string, msg *messagepb.MsgData) error,
) *CallNotificationSender {
	return &CallNotificationSender{
		NotificationSender: notification.NewNotificationSender(
			notification.WithPushMsg(pushMsg),
		),
	}
}

func (c *CallNotificationSender) CallInvite(ctx context.Context, callerID, calleeID string, tips notification.CallTips) {
	c.Notification(ctx, callerID, calleeID, notification.CallInviteNotification, tips)
}

func (c *CallNotificationSender) CallAccepted(ctx context.Context, calleeID, callerID string, tips notification.CallTips) {
	c.Notification(ctx, calleeID, callerID, notification.CallAcceptedNotification, tips)
	// 被叫其他端停铃。
	c.Notification(ctx, calleeID, calleeID, notification.CallAcceptedNotification, tips)
}

func (c *CallNotificationSender) CallRejected(ctx context.Context, calleeID, callerID string, tips notification.CallTips) {
	c.Notification(ctx, calleeID, callerID, notification.CallRejectedNotification, tips)
}

func (c *CallNotificationSender) CallCancelled(ctx context.Context, callerID, calleeID string, tips notification.CallTips) {
	c.Notification(ctx, callerID, calleeID, notification.CallCancelledNotification, tips)
}

func (c *CallNotificationSender) CallTimeout(ctx context.Context, sendID, recvID string, tips notification.CallTips) {
	c.Notification(ctx, sendID, recvID, notification.CallTimeoutNotification, tips)
}

func (c *CallNotificationSender) CallBusy(ctx context.Context, calleeID, callerID string, tips notification.CallTips) {
	c.Notification(ctx, calleeID, callerID, notification.CallBusyNotification, tips)
}

func (c *CallNotificationSender) CallEnded(ctx context.Context, sendID, recvID string, tips notification.CallTips) {
	c.Notification(ctx, sendID, recvID, notification.CallEndedNotification, tips)
}
