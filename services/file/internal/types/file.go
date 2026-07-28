package types

import "time"

const (
	StatusPending   = "pending"
	StatusAvailable = "available"
	StatusDeleted   = "deleted"
)

type File struct {
	FileID          string     `gorm:"column:file_id;primaryKey;size:36"`
	OwnerID         string     `gorm:"column:owner_id;size:64;not null;index:idx_file_owner_status"`
	ObjectKey       string     `gorm:"column:object_key;size:512;not null;uniqueIndex"`
	OriginalName    string     `gorm:"column:original_name;size:255;not null"`
	ContentType     string     `gorm:"column:content_type;size:255;not null"`
	Size            int64      `gorm:"column:size;not null"`
	SHA256          string     `gorm:"column:sha256;size:64;index:idx_file_dedup"`
	Category        string     `gorm:"column:category;size:32;not null"`
	Status          string     `gorm:"column:status;size:16;not null;index:idx_file_owner_status"`
	UploadExpiresAt time.Time  `gorm:"column:upload_expires_at;index"`
	ExpiresAt       time.Time  `gorm:"column:expires_at;index"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
	DeletedAt       *time.Time `gorm:"column:deleted_at"`
}

func (File) TableName() string { return "file_object" }

type Binding struct {
	ID             uint64    `gorm:"column:id;primaryKey;autoIncrement"`
	FileID         string    `gorm:"column:file_id;size:36;not null;uniqueIndex:idx_file_conversation"`
	ConversationID string    `gorm:"column:conversation_id;size:128;not null;uniqueIndex:idx_file_conversation;index"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	ExpiresAt      time.Time `gorm:"column:expires_at;index"`
}

func (Binding) TableName() string { return "file_binding" }
