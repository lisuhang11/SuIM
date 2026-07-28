// Package types 定义关系服务的领域模型，包括好友请求、好友和拉黑。
package types

import "time"

// --------------- 好友请求 ---------------

// FriendRequestHandleResult 表示好友请求的处理状态。
type FriendRequestHandleResult int

const (
	FriendRequestPending  FriendRequestHandleResult = 0  // 待处理
	FriendRequestAccepted FriendRequestHandleResult = 1  // 已接受
	FriendRequestRejected FriendRequestHandleResult = -1 // 已拒绝
)

// FriendRequest 好友请求领域模型，映射到 friend_request 表。
type FriendRequest struct {
	FromUserID    string                   `json:"from_user_id"    gorm:"column:from_user_id;primaryKey;not null;comment:申请人 userID"`
	ToUserID      string                   `json:"to_user_id"      gorm:"column:to_user_id;primaryKey;not null;comment:被申请人 userID"`
	HandleResult  FriendRequestHandleResult `json:"handle_result"   gorm:"column:handle_result;default:0;comment:0未处理 1同意 -1拒绝"`
	ReqMsg        string                   `json:"req_msg"         gorm:"column:req_msg;default:'';comment:申请留言"`
	CreateTime    time.Time                `json:"create_time"     gorm:"column:create_time;autoCreateTime;comment:申请时间"`
	HandlerUserID string                   `json:"handler_user_id" gorm:"column:handler_user_id;default:'';comment:处理人 userID"`
	HandleMsg     string                   `json:"handle_msg"      gorm:"column:handle_msg;default:'';comment:处理留言"`
	HandleTime    *time.Time               `json:"handle_time"     gorm:"column:handle_time;comment:处理时间"`
	Ex            string                   `json:"ex"              gorm:"column:ex;default:'';comment:扩展字段(json)"`
}

// TableName 返回 friend_request 表名。
func (FriendRequest) TableName() string {
	return "friend_request"
}

// --------------- 好友 ---------------

// Friend 好友关系领域模型（单向视角），映射到 friend 表。
type Friend struct {
	OwnerUserID    string    `json:"owner_user_id"    gorm:"column:owner_user_id;primaryKey;not null;comment:关系拥有者 userID"`
	FriendUserID   string    `json:"friend_user_id"   gorm:"column:friend_user_id;primaryKey;not null;comment:好友 userID"`
	Remark         string    `json:"remark"           gorm:"column:remark;default:'';comment:好友备注"`
	CreateTime     time.Time `json:"create_time"      gorm:"column:create_time;autoCreateTime;comment:成为好友的时间"`
	AddSource      int       `json:"add_source"       gorm:"column:add_source;default:0;comment:添加来源"`
	OperatorUserID string    `json:"operator_user_id" gorm:"column:operator_user_id;default:'';comment:操作者 userID"`
	Ex             string    `json:"ex"               gorm:"column:ex;default:'';comment:扩展字段(json)"`
	IsPinned       bool      `json:"is_pinned"        gorm:"column:is_pinned;default:0;comment:是否置顶(1是0否)"`
}

// TableName 返回 friend 表名。
func (Friend) TableName() string {
	return "friend"
}

// --------------- 拉黑 ---------------

// Black 拉黑关系领域模型（单向视角），映射到 black 表。
type Black struct {
	OwnerUserID    string    `json:"owner_user_id"    gorm:"column:owner_user_id;primaryKey;not null;comment:拉黑者 userID"`
	BlockUserID    string    `json:"block_user_id"    gorm:"column:block_user_id;primaryKey;not null;comment:被拉黑者 userID"`
	CreateTime     time.Time `json:"create_time"      gorm:"column:create_time;autoCreateTime;comment:拉黑时间"`
	AddSource      int       `json:"add_source"       gorm:"column:add_source;default:0;comment:添加来源"`
	OperatorUserID string    `json:"operator_user_id" gorm:"column:operator_user_id;default:'';comment:操作者 userID"`
	Ex             string    `json:"ex"               gorm:"column:ex;default:'';comment:扩展字段(json)"`
}

// TableName 返回 black 表名。
func (Black) TableName() string {
	return "black"
}
