// Package types 定义用户服务的领域模型。
package types

import "time"

// AuthToken 认证令牌模型，存储 JWT 令牌的持久化信息。
type AuthToken struct {
	// ID 令牌唯一标识。
	ID string `json:"id"         gorm:"type:varchar(36);primaryKey"`
	// UserID 令牌所属用户 ID。
	UserID string `json:"user_id"    gorm:"type:varchar(64);index;not null"`
	// Token 令牌值（JWT 格式）。
	Token string `json:"token"      gorm:"type:text;not null"`
	// TokenType 令牌类型（access_token / refresh_token）。
	TokenType string `json:"token_type" gorm:"type:varchar(50);not null"`
	// ExpiresAt 令牌过期时间。
	ExpiresAt time.Time `json:"expires_at"`
	// IsRevoked 是否已吊销。
	IsRevoked bool `json:"is_revoked" gorm:"default:false"`
	// CreatedAt 创建时间。
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt 更新时间。
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 覆盖 GORM 默认复数表名，与 migrations/user.sql 对齐。
func (AuthToken) TableName() string { return "auth_tokens" }

// User 统一用户领域模型，覆盖个人资料和认证字段。
type User struct {
	UserID           string    `json:"user_id"             gorm:"type:varchar(64);primaryKey;comment:用户ID"`
	Email            string    `json:"email"               gorm:"type:varchar(255);uniqueIndex;not null;comment:邮箱"`
	PasswordHash     string    `json:"-"                   gorm:"type:varchar(255);not null;comment:密码哈希"`
	Nickname         string    `json:"nickname"            gorm:"type:varchar(255);not null;default:'';comment:昵称"`
	AvatarURL        string    `json:"avatar_url"          gorm:"type:varchar(1024);not null;default:'';comment:头像URL"`
	Ex               string    `json:"ex"                  gorm:"type:varchar(1024);not null;default:'';comment:扩展字段"`
	AppMangerLevel   int       `json:"app_manger_level"    gorm:"type:int;not null;default:0;comment:管理员级别"`
	// GlobalRecvMsgOpt 全局消息接收选项：0 正常 / 1 不接收 / 2 接收但不通知。
	GlobalRecvMsgOpt int       `json:"global_recv_msg_opt" gorm:"type:int;not null;default:0;comment:全局消息接收选项"`
	IsActive         bool      `json:"is_active"           gorm:"not null;default:true;comment:是否激活"`
	CreateTime       time.Time `json:"create_time"         gorm:"autoCreateTime:milli;comment:创建时间"`
	UpdatedAt        time.Time `json:"updated_at"          gorm:"autoUpdateTime:milli;comment:更新时间"`
}

// TableName 覆盖 GORM 默认复数表名（users），与 migrations/user.sql 的 `user` 对齐。
func (User) TableName() string { return "user" }

// 全局消息接收选项，对齐 OpenIM constant.ReceiveMessage 等语义。
const (
	ReceiveMessage           = 0 // 正常接收并通知
	NotReceiveMessage        = 1 // 不接收消息
	ReceiveNotNotifyMessage  = 2 // 接收但不推送离线通知
)

// ValidGlobalRecvMsgOpt 校验选项是否为 0/1/2。
func ValidGlobalRecvMsgOpt(opt int) bool {
	return opt >= ReceiveMessage && opt <= ReceiveNotNotifyMessage
}

// UserProfilePatch 资料局部更新（字段为 nil 表示不改），对齐 OpenIM UserInfoWithEx。
type UserProfilePatch struct {
	Nickname  *string
	AvatarURL *string
	Ex        *string
}
