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

// User 统一用户领域模型，覆盖个人资料和认证字段。
type User struct {
	UserID           string    `json:"user_id"             gorm:"type:varchar(64);primaryKey;comment:用户ID"`
	Email            string    `json:"email"               gorm:"type:varchar(255);uniqueIndex;not null;comment:邮箱"`
	PasswordHash     string    `json:"-"                   gorm:"type:varchar(255);not null;comment:密码哈希"`
	Nickname         string    `json:"nickname"            gorm:"type:varchar(255);not null;default:'';comment:昵称"`
	AvatarURL        string    `json:"avatar_url"          gorm:"type:varchar(1024);not null;default:'';comment:头像URL"`
	Ex               string    `json:"ex"                  gorm:"type:varchar(1024);not null;default:'';comment:扩展字段"`
	AppMangerLevel   int       `json:"app_manger_level"    gorm:"type:int;not null;default:0;comment:管理员级别"`
	GlobalRecvMsgOpt int       `json:"global_recv_msg_opt" gorm:"type:int;not null;default:0;comment:全局消息接收选项"`
	IsActive         bool      `json:"is_active"           gorm:"not null;default:true;comment:是否激活"`
	CreateTime       time.Time `json:"create_time"         gorm:"autoCreateTime:milli;comment:创建时间"`
	UpdatedAt        time.Time `json:"updated_at"          gorm:"autoUpdateTime:milli;comment:更新时间"`
}
