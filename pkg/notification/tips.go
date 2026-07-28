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
