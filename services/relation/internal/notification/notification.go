// Package notification 提供 relation 服务的好友通知发送器。
// 通过嵌入 pkg/notification.NotificationSender 内核来复用通用异步发送管道，
// 此处只负责拼装业务 tips 结构并调用基类 Notification()。
package notification

import (
	"context"
	"time"

	"SuIM/pkg/notification"
	"SuIM/pkg/notification/common_user"
	messagepb "SuIM/proto/messagepb"
)

// FriendNotificationSender 好友类通知发送器。
// 嵌入 notification.NotificationSender 内核，添加好友业务相关的领域方法。
type FriendNotificationSender struct {
	*notification.NotificationSender

	getUsersInfo func(ctx context.Context, userID string) (common_user.CommonUser, error)
}

// friendNotifOption 函数选项（不导出）。
type friendNotifOption func(*FriendNotificationSender)

// WithRpcFunc 注入用户资料获取函数，用于填充 MsgData 发送者昵称/头像。
func WithRpcFunc(fn func(ctx context.Context, userID string) (common_user.CommonUser, error)) friendNotifOption {
	return func(f *FriendNotificationSender) { f.getUsersInfo = fn }
}

// NewFriendNotificationSender 创建好友类通知发送器。
// pushMsg 是发送出口，通常封装了 msggateway.OnlinePushMsg。
func NewFriendNotificationSender(pushMsg func(ctx context.Context, recvID string, msg *messagepb.MsgData) error, opts ...friendNotifOption) *FriendNotificationSender {
	f := &FriendNotificationSender{
		NotificationSender: notification.NewNotificationSender(
			notification.WithPushMsg(pushMsg),
		),
	}
	for _, o := range opts {
		o(f)
	}
	if f.getUsersInfo != nil {
		f.NotificationSender.SetUserInfoFunc(f.getUsersInfo)
	}
	return f
}

// ---------- 领域方法：拼装业务 tips，调用基类 Notification ----------

// FriendApplicationNotification 好友申请通知，发给被申请方（toUserID）。
func (f *FriendNotificationSender) FriendApplicationNotification(ctx context.Context, fromUserID, toUserID, applyMsg string) {
	tips := notification.FriendApplicationTips{
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		ApplyMsg:   applyMsg,
		ApplyTime:  time.Now().UnixMilli(),
	}
	f.Notification(ctx, fromUserID, toUserID, notification.FriendApplicationNotification, tips)
}

// FriendApplicationAcceptedNotification 申请被接受通知，发给申请方。
func (f *FriendNotificationSender) FriendApplicationAcceptedNotification(ctx context.Context, fromUserID, toUserID string) {
	tips := notification.FriendApplicationAcceptedTips{
		FromUserID: toUserID,   // 接受方
		ToUserID:   fromUserID, // 申请方
		HandleTime: time.Now().UnixMilli(),
	}
	f.Notification(ctx, toUserID, fromUserID, notification.FriendApplicationAcceptedNotification, tips)
}

// FriendApplicationRejectedNotification 申请被拒绝通知，发给申请方。
func (f *FriendNotificationSender) FriendApplicationRejectedNotification(ctx context.Context, fromUserID, toUserID, handleMsg string) {
	tips := notification.FriendApplicationRejectedTips{
		FromUserID: toUserID,   // 拒绝方
		ToUserID:   fromUserID, // 申请方
		HandleMsg:  handleMsg,
		HandleTime: time.Now().UnixMilli(),
	}
	f.Notification(ctx, toUserID, fromUserID, notification.FriendApplicationRejectedNotification, tips)
}

// FriendDeletedNotification 好友删除通知，发给 recvUserID（需同步好友列表的一方）。
func (f *FriendNotificationSender) FriendDeletedNotification(ctx context.Context, operatorUserID, recvUserID string) {
	tips := notification.FriendDeletedTips{
		FromUserID: operatorUserID,
		ToUserID:   recvUserID,
		HandleTime: time.Now().UnixMilli(),
	}
	f.Notification(ctx, operatorUserID, recvUserID, notification.FriendDeletedNotification, tips)
}

// FriendInfoChangedNotification 备注/置顶变更，发给 owner（多端同步）。
func (f *FriendNotificationSender) FriendInfoChangedNotification(ctx context.Context, ownerUserID, friendUserID string) {
	tips := notification.FriendInfoChangedTips{
		OwnerUserID:  ownerUserID,
		FriendUserID: friendUserID,
		HandleTime:   time.Now().UnixMilli(),
	}
	f.Notification(ctx, ownerUserID, ownerUserID, notification.FriendInfoChangedNotification, tips)
}

// FriendInfoUpdatedNotification 好友资料变更，发给 ownerUserID。
func (f *FriendNotificationSender) FriendInfoUpdatedNotification(ctx context.Context, changedUserID, ownerUserID string) {
	tips := notification.FriendInfoUpdatedTips{
		ChangedUserID: changedUserID,
		OwnerUserID:   ownerUserID,
		HandleTime:    time.Now().UnixMilli(),
	}
	f.Notification(ctx, changedUserID, ownerUserID, notification.FriendInfoUpdatedNotification, tips)
}
