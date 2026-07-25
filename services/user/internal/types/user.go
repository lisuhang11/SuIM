package types

import "time"

type AuthToken struct {
	// Unique identifier of the token
	ID string `json:"id"         gorm:"type:varchar(36);primaryKey"`
	// User ID that owns this token
	UserID string `json:"user_id"    gorm:"type:varchar(64);index;not null"`
	// Token value (JWT or other format)
	Token string `json:"token"      gorm:"type:text;not null"`
	// Token type (access_token, refresh_token)
	TokenType string `json:"token_type" gorm:"type:varchar(50);not null"`
	// Token expiration time
	ExpiresAt time.Time `json:"expires_at"`
	// Whether the token is revoked
	IsRevoked bool `json:"is_revoked" gorm:"default:false"`
	// Creation time of the token
	CreatedAt time.Time `json:"created_at"`
	// Last updated time of the token
	UpdatedAt time.Time `json:"updated_at"`
}

// User is the unified user domain model, covering both OpenIM-style
// profile fields and authentication fields.
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

type RegisterRequest struct {
	Username string `json:"username" binding:"required,min=2,max=50"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type RegisterResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	User    *User  `json:"user,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message,omitempty"`
	User         *User  `json:"user,omitempty"`
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
}
