package notification

// FriendApplicationTips 好友申请通知 payload，发给被申请方。
type FriendApplicationTips struct {
	FromUserID string `json:"from_user_id"`
	ToUserID   string `json:"to_user_id"`
	ApplyMsg   string `json:"apply_msg"`
	ApplyTime  int64  `json:"apply_time"` // unix 毫秒
}

// FriendApplicationAcceptedTips 好友申请被接受通知 payload，发给申请方。
type FriendApplicationAcceptedTips struct {
	FromUserID string `json:"from_user_id"` // 接受方
	ToUserID   string `json:"to_user_id"`   // 申请方
	HandleTime int64  `json:"handle_time"`  // unix 毫秒
}

// FriendApplicationRejectedTips 好友申请被拒绝通知 payload，发给申请方。
type FriendApplicationRejectedTips struct {
	FromUserID string `json:"from_user_id"` // 拒绝方
	ToUserID   string `json:"to_user_id"`   // 申请方
	HandleMsg  string `json:"handle_msg"`
	HandleTime int64  `json:"handle_time"` // unix 毫秒
}

// FriendDeletedTips 好友删除通知。
type FriendDeletedTips struct {
	FromUserID string `json:"from_user_id"` // 操作方
	ToUserID   string `json:"to_user_id"`   // 被删方 / 需要同步的一方
	HandleTime int64  `json:"handle_time"`
}

// FriendInfoChangedTips 备注/置顶变更通知（发给操作方自己，驱动多端同步）。
type FriendInfoChangedTips struct {
	OwnerUserID  string `json:"owner_user_id"`
	FriendUserID string `json:"friend_user_id"`
	HandleTime   int64  `json:"handle_time"`
}

// FriendInfoUpdatedTips 好友资料变更通知（发给需要刷新列表的 owner）。
type FriendInfoUpdatedTips struct {
	ChangedUserID string `json:"changed_user_id"`
	OwnerUserID   string `json:"owner_user_id"`
	HandleTime    int64  `json:"handle_time"`
}

// UserOnlineStatusTips 用户在线状态 tip（发给订阅了该用户的连接）。
type UserOnlineStatusTips struct {
	UserID      string  `json:"user_id"`
	Status      int32   `json:"status"` // 1=online 0=offline
	PlatformIDs []int32 `json:"platform_ids,omitempty"`
}

// RevokeMsgTips 消息撤回 tip（对齐 OpenIM RevokeMsgTips）。
type RevokeMsgTips struct {
	RevokerUserID  string `json:"revoker_user_id"`
	ClientMsgID    string `json:"client_msg_id"`
	RevokeTime     int64  `json:"revoke_time"`
	SessionType    int32  `json:"session_type"`
	Seq            int64  `json:"seq"`
	ConversationID string `json:"conversation_id"`
	IsAdminRevoke  bool   `json:"is_admin_revoke,omitempty"`
}

// MarkAsReadTips 已读回执 tip（对齐 OpenIM MarkAsReadTips）。
type MarkAsReadTips struct {
	MarkAsReadUserID string  `json:"mark_as_read_user_id"`
	ConversationID   string  `json:"conversation_id"`
	Seqs             []int64 `json:"seqs,omitempty"`
	HasReadSeq       int64   `json:"has_read_seq"`
}

// CallTips 通话信令 tip / 时间线摘要共用 payload。
type CallTips struct {
	CallID         string `json:"call_id"`
	CallerID       string `json:"caller_id,omitempty"`
	CalleeID       string `json:"callee_id,omitempty"`
	MediaType      string `json:"media_type,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	Reason         string `json:"reason,omitempty"`
	DurationSec    int32  `json:"duration_sec,omitempty"`
}
