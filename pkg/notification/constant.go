// Package notification 提供与业务无关的异步消息通知内核。
// 各业务服务通过嵌入 NotificationSender 并添加领域方法来扩展。
package notification

const (
	// MsgFromSystem 系统消息来源标识。
	MsgFromSystem = 200

	// SessionTypeSingle 单聊会话类型。
	SessionTypeSingle = 1

	// SystemSenderID 系统消息发送者标识。
	SystemSenderID = "sys_msg"

	// -------- 好友类 contentType（1000~1099）--------

	// FriendApplicationNotification 好友申请通知 → 发给被申请方。
	FriendApplicationNotification = 1000

	// FriendApplicationAcceptedNotification 好友申请被接受通知 → 发给申请方。
	FriendApplicationAcceptedNotification = 1001

	// FriendApplicationRejectedNotification 好友申请被拒绝通知 → 发给申请方。
	FriendApplicationRejectedNotification = 1002
)

// defaultSessionTypeConf 返回 contentType → sessionType 硬编码映射表。
// 所有好友类通知均为单聊（SessionTypeSingle）。
func defaultSessionTypeConf() map[int32]int32 {
	return map[int32]int32{
		FriendApplicationNotification:         SessionTypeSingle,
		FriendApplicationAcceptedNotification: SessionTypeSingle,
		FriendApplicationRejectedNotification: SessionTypeSingle,
	}
}
