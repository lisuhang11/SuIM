// Package types 定义群组服务的领域模型，包括群组、群成员和入群请求，以及各类输入值对象。
package types

import "time"

// 成员角色等级，数字越大权限越高。
const (
	GroupMemberRoleNormal int = 0 // 普通成员
	GroupMemberRoleAdmin  int = 1 // 管理员
	GroupMemberRoleOwner  int = 2 // 群主
)

// 群组状态位掩码（存储在 Group.Status 字段中）。
const (
	// GroupStatusMutedBit 在 Group.Status 中表示全员禁言。
	GroupStatusMutedBit int = 1 << 0
)

// IsMuted 判断群组是否处于全员禁言状态。
func (g *Group) IsMuted() bool { return g.Status&GroupStatusMutedBit != 0 }

// SetMuted 设置或取消群组全员禁言。
func (g *Group) SetMuted(muted bool) {
	if muted {
		g.Status |= GroupStatusMutedBit
	} else {
		g.Status &^= GroupStatusMutedBit
	}
}

// Group 群组领域模型，映射到 group 表。
type Group struct {
	GroupID                string    `json:"group_id"                gorm:"column:group_id;primaryKey;not null;comment:群组ID"`
	GroupName              string    `json:"group_name"              gorm:"column:group_name;not null;default:'';comment:群名称"`
	Notification           string    `json:"notification"            gorm:"column:notification;not null;default:'';comment:群公告"`
	Introduction           string    `json:"introduction"            gorm:"column:introduction;not null;default:'';comment:群简介"`
	FaceURL                string    `json:"face_url"                gorm:"column:face_url;not null;default:'';comment:群头像"`
	CreateTime             time.Time `json:"create_time"             gorm:"column:create_time;not null;comment:创建时间"`
	Ex                     string    `json:"ex"                      gorm:"column:ex;not null;default:'';comment:扩展字段(json)"`
	Status                 int       `json:"status"                  gorm:"column:status;not null;default:0;comment:群状态(bit0全员禁言)"`
	CreatorUserID          string    `json:"creator_user_id"         gorm:"column:creator_user_id;not null;default:'';index;comment:创建者用户ID"`
	GroupType              int       `json:"group_type"              gorm:"column:group_type;not null;default:0;comment:群类型"`
	NeedVerification       int       `json:"need_verification"       gorm:"column:need_verification;not null;default:0;comment:加群是否需要验证"`
	LookMemberInfo         int       `json:"look_member_info"        gorm:"column:look_member_info;not null;default:0;comment:是否允许查看成员信息"`
	ApplyMemberFriend      int       `json:"apply_member_friend"     gorm:"column:apply_member_friend;not null;default:0;comment:是否允许成员互加好友"`
	NotificationUpdateTime time.Time `json:"notification_update_time" gorm:"column:notification_update_time;not null;comment:公告更新时间"`
	NotificationUserID     string    `json:"notification_user_id"    gorm:"column:notification_user_id;not null;default:'';comment:公告更新者用户ID"`
}

// TableName 返回 group 表名。MySQL 保留字 `group` 由 GORM mysql 方言自动加引号处理。
func (Group) TableName() string {
	return "group"
}

// GroupMember 群成员关系领域模型，映射到 group_member 表。
type GroupMember struct {
	GroupID        string     `json:"group_id"        gorm:"column:group_id;primaryKey;not null;comment:群组ID"`
	UserID         string     `json:"user_id"         gorm:"column:user_id;primaryKey;not null;comment:用户ID"`
	Nickname       string     `json:"nickname"        gorm:"column:nickname;not null;default:'';comment:群内昵称"`
	FaceURL        string     `json:"face_url"        gorm:"column:face_url;not null;default:'';comment:群内头像"`
	RoleLevel      int        `json:"role_level"      gorm:"column:role_level;not null;default:0;comment:角色(0普通1管理员2群主)"`
	JoinTime       time.Time  `json:"join_time"       gorm:"column:join_time;not null;comment:加入时间"`
	JoinSource     int        `json:"join_source"     gorm:"column:join_source;not null;default:0;comment:加入来源"`
	InviterUserID  string     `json:"inviter_user_id" gorm:"column:inviter_user_id;not null;default:'';comment:邀请者用户ID"`
	OperatorUserID string     `json:"operator_user_id" gorm:"column:operator_user_id;not null;default:'';comment:操作者用户ID"`
	MuteEndTime    *time.Time `json:"mute_end_time"  gorm:"column:mute_end_time;comment:禁言结束时间"`
	Ex             string     `json:"ex"              gorm:"column:ex;not null;default:'';comment:扩展字段(json)"`
}

// TableName 返回 group_member 表名。
func (GroupMember) TableName() string {
	return "group_member"
}

// GroupRequest 入群请求领域模型，映射到 group_request 表。
type GroupRequest struct {
	UserID        string     `json:"user_id"        gorm:"column:user_id;primaryKey;not null;comment:申请用户ID"`
	GroupID       string     `json:"group_id"       gorm:"column:group_id;primaryKey;not null;comment:群组ID"`
	HandleResult  int        `json:"handle_result"  gorm:"column:handle_result;not null;default:0;comment:处理状态(0未处理1同意-1拒绝)"`
	ReqMsg        string     `json:"req_msg"        gorm:"column:req_msg;not null;default:'';comment:申请留言"`
	HandledMsg    string     `json:"handled_msg"    gorm:"column:handled_msg;not null;default:'';comment:处理留言"`
	ReqTime       time.Time  `json:"req_time"       gorm:"column:req_time;not null;comment:申请时间"`
	HandleUserID  string     `json:"handle_user_id" gorm:"column:handle_user_id;not null;default:'';comment:处理者用户ID"`
	HandledTime   *time.Time `json:"handled_time"  gorm:"column:handled_time;comment:处理时间"`
	JoinSource    int        `json:"join_source"    gorm:"column:join_source;not null;default:0;comment:加入来源"`
	InviterUserID string     `json:"inviter_user_id" gorm:"column:inviter_user_id;not null;default:'';comment:邀请者用户ID"`
	Ex            string     `json:"ex"             gorm:"column:ex;not null;default:'';comment:扩展字段(json)"`
}

// TableName 返回 group_request 表名。
func (GroupRequest) TableName() string {
	return "group_request"
}

// --------------- 请求/结果值对象（服务层使用） ---------------

// CreateGroupInput 创建群组的输入参数。
type CreateGroupInput struct {
	CreatorUserID     string
	GroupName         string
	Notification      string
	Introduction      string
	FaceURL           string
	GroupType         int
	NeedVerification  int
	LookMemberInfo    int
	ApplyMemberFriend int
	MemberIDs         []string
	Ex                string
}

// UpdateGroupInfoInput 更新群组信息的输入参数。
// 三个标记指针区分"未提供"（nil）和明确的 0/1 值。
type UpdateGroupInfoInput struct {
	GroupID           string
	OpUserID          string
	GroupName         string
	Notification      string
	Introduction      string
	FaceURL           string
	NeedVerification  *int
	LookMemberInfo    *int
	ApplyMemberFriend *int
	Ex                string
}

// InviteInput 邀请用户入群的输入参数。
type InviteInput struct {
	GroupID  string
	OpUserID string
	UserIDs  []string
	Reason   string
}

// ApplyInput 申请入群的输入参数。
type ApplyInput struct {
	GroupID       string
	UserID        string
	ReqMsg        string
	JoinSource    int
	InviterUserID string
}

// HandleInput 处理入群申请的输入参数。
type HandleInput struct {
	GroupID      string
	UserID       string
	OpUserID     string
	HandleResult int
	HandledMsg   string
}
