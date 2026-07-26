package types

import "time"

// Friend 好友关系表（单方视角）
type Friend struct {
	OwnerUserID    string     `json:"owner_user_id"  gorm:"column:owner_user_id;type:varchar(64);primaryKey;not null"`
	FriendUserID   string     `json:"friend_user_id" gorm:"column:friend_user_id;type:varchar(64);primaryKey;not null"`
	Remark         string     `json:"remark"         gorm:"column:remark;type:varchar(255);default:''"`
	CreateTime     *time.Time `json:"create_time"    gorm:"column:create_time;default null"`
	AddSource      int        `json:"add_source"     gorm:"column:add_source;default:0"`
	OperatorUserID string     `json:"operator_user_id" gorm:"column:operator_user_id;type:varchar(64);default:''"`
	Ex             string     `json:"ex"             gorm:"column:ex;type:varchar(1024);default:''"`
	IsPinned       bool       `json:"is_pinned"      gorm:"column:is_pinned;default:0"`
}

// FriendRequest 好友申请表
type FriendRequest struct {
	FromUserID    string     `json:"from_user_id"   gorm:"column:from_user_id;type:varchar(64);primaryKey;not null"`
	ToUserID      string     `json:"to_user_id"     gorm:"column:to_user_id;type:varchar(64);primaryKey;not null"`
	HandleResult  int        `json:"handle_result"  gorm:"column:handle_result;default:0"`
	ReqMsg        string     `json:"req_msg"        gorm:"column:req_msg;type:varchar(512);default:''"`
	CreateTime    *time.Time `json:"create_time"    gorm:"column:create_time;default null"`
	HandlerUserID string     `json:"handler_user_id" gorm:"column:handler_user_id;type:varchar(64);default:''"`
	HandleMsg     string     `json:"handle_msg"     gorm:"column:handle_msg;type:varchar(512);default:''"`
	HandleTime    *time.Time `json:"handle_time"    gorm:"column:handle_time;default null"`
	Ex            string     `json:"ex"             gorm:"column:ex;type:varchar(1024);default:''"`
}

// Black 黑名单表
type Black struct {
	OwnerUserID    string     `json:"owner_user_id"  gorm:"column:owner_user_id;type:varchar(64);primaryKey;not null"`
	BlockUserID    string     `json:"block_user_id"  gorm:"column:block_user_id;type:varchar(64);primaryKey;not null"`
	CreateTime     *time.Time `json:"create_time"    gorm:"column:create_time;default null"`
	AddSource      int        `json:"add_source"     gorm:"column:add_source;default:0"`
	OperatorUserID string     `json:"operator_user_id" gorm:"column:operator_user_id;type:varchar(64);default:''"`
	Ex             string     `json:"ex"             gorm:"column:ex;type:varchar(1024);default:''"`
}
