// Package notification 提供与业务无关的异步消息通知内核。
// 各业务服务通过嵌入 NotificationSender 并添加领域方法来扩展。
package notification

const (
	// MsgFromSystem 系统消息来源标识。
	MsgFromSystem = 200

	// SessionTypeSingle 单聊会话类型。
	SessionTypeSingle = 1
	// SessionTypeGroup 群聊会话类型。
	SessionTypeGroup = 2

	// SystemSenderID 系统消息发送者标识。
	SystemSenderID = "sys_msg"

	// -------- 好友类 contentType（1000~1099）--------

	// FriendApplicationNotification 好友申请通知 → 发给被申请方。
	FriendApplicationNotification = 1000

	// FriendApplicationAcceptedNotification 好友申请被接受通知 → 发给申请方。
	FriendApplicationAcceptedNotification = 1001

	// FriendApplicationRejectedNotification 好友申请被拒绝通知 → 发给申请方。
	FriendApplicationRejectedNotification = 1002

	// FriendDeletedNotification 好友被删除通知。
	FriendDeletedNotification = 1003

	// FriendInfoChangedNotification 好友备注/置顶等关系字段变更通知。
	FriendInfoChangedNotification = 1004

	// FriendInfoUpdatedNotification 好友本人资料（昵称/头像）变更通知。
	FriendInfoUpdatedNotification = 1005

	// -------- 用户在线类 contentType --------

	// UserOnlineStatusNotification 好友上下线 tip（对齐 OpenIM 订阅推送语义）。
	UserOnlineStatusNotification = 1303

	// -------- 通话类 contentType（1400~1499 tip；1501 时间线）--------

	// CallInviteNotification 来电 tip → 发给被叫。
	CallInviteNotification = 1401
	// CallAcceptedNotification 接听 tip → 发给主叫（及其他端停铃）。
	CallAcceptedNotification = 1402
	// CallRejectedNotification 拒绝 tip。
	CallRejectedNotification = 1403
	// CallCancelledNotification 主叫取消 tip。
	CallCancelledNotification = 1404
	// CallTimeoutNotification 振铃超时 tip。
	CallTimeoutNotification = 1405
	// CallBusyNotification 忙线 tip（可选推给主叫）。
	CallBusyNotification = 1406
	// CallEndedNotification 通话结束 tip。
	CallEndedNotification = 1407
	// CallRecordContentType 会话时间线通话摘要（持久化消息）。
	CallRecordContentType = 1501

	// -------- 消息类 contentType（对齐 OpenIM）--------

	// RevokeNotification 消息撤回 tip（OpenIM MsgRevokeNotification = 2101）。
	RevokeNotification = 2101

	// HasReadReceipt 已读回执 tip（OpenIM HasReadReceipt = 2200）。
	HasReadReceipt = 2200
)

// defaultSessionTypeConf 返回 contentType → sessionType 硬编码映射表。
// 好友类默认单聊；消息 tip 默认单聊，群聊撤回时通过 NotificationWithSessionType 覆盖。
func defaultSessionTypeConf() map[int32]int32 {
	return map[int32]int32{
		FriendApplicationNotification:         SessionTypeSingle,
		FriendApplicationAcceptedNotification: SessionTypeSingle,
		FriendApplicationRejectedNotification: SessionTypeSingle,
		FriendDeletedNotification:             SessionTypeSingle,
		FriendInfoChangedNotification:         SessionTypeSingle,
		FriendInfoUpdatedNotification:         SessionTypeSingle,
		UserOnlineStatusNotification:          SessionTypeSingle,
		CallInviteNotification:                SessionTypeSingle,
		CallAcceptedNotification:              SessionTypeSingle,
		CallRejectedNotification:              SessionTypeSingle,
		CallCancelledNotification:             SessionTypeSingle,
		CallTimeoutNotification:               SessionTypeSingle,
		CallBusyNotification:                  SessionTypeSingle,
		CallEndedNotification:                 SessionTypeSingle,
		CallRecordContentType:                 SessionTypeSingle,
		RevokeNotification:                    SessionTypeSingle,
		HasReadReceipt:                        SessionTypeSingle,
	}
}
