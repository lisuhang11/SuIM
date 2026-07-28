// Package types 定义 push 服务的领域模型。
package types

// 平台 ID 常量。
const (
	PlatformIOS     = 1 // iOS (APNs)
	PlatformAndroid = 2 // Android (FCM)
	PlatformWindows = 3 // Windows
	PlatformMacOS   = 4 // macOS
	PlatformWeb     = 5 // Web
)

// PushToken 用户设备推送令牌存储模型。
type PushToken struct {
	ID         int64  `gorm:"primaryKey;autoIncrement;column:id"`
	UserID     string `gorm:"column:user_id;size:64;not null;uniqueIndex:idx_user_platform"`
	PlatformID int    `gorm:"column:platform_id;not null;default:0;uniqueIndex:idx_user_platform"`
	Token      string `gorm:"column:token;size:512;not null"`
	CreatedAt  int64  `gorm:"column:created_at;not null;default:0"`
	UpdatedAt  int64  `gorm:"column:updated_at;not null;default:0"`
}

// TableName 覆盖 GORM 默认表名。
func (PushToken) TableName() string { return TablePushToken }
