package types

const (
	CallStatusRinging  = "ringing"
	CallStatusAccepted = "accepted"
	CallStatusActive   = "active"
	CallStatusEnded    = "ended"
)

const (
	EndReasonCompleted   = "completed"
	EndReasonRejected    = "rejected"
	EndReasonCancelled   = "cancelled"
	EndReasonTimeout     = "timeout"
	EndReasonBusy        = "busy"
	EndReasonUnavailable = "unavailable"
)

const (
	MediaTypeAudio = "audio"
	MediaTypeVideo = "video"
)

// Call 通话记录（rtc_calls 表）。
type Call struct {
	CallID         string `gorm:"primaryKey;column:call_id;size:64"`
	ConversationID string `gorm:"column:conversation_id;size:128;not null;index"`
	CallerID       string `gorm:"column:caller_id;size:64;not null;index:idx_caller_status,priority:1"`
	CalleeID       string `gorm:"column:callee_id;size:64;not null;index:idx_callee_status,priority:1"`
	MediaType      string `gorm:"column:media_type;size:16;not null"`
	Status         string `gorm:"column:status;size:16;not null;index:idx_caller_status,priority:2;index:idx_callee_status,priority:2"`
	EndReason      string `gorm:"column:end_reason;size:32"`
	RoomName       string `gorm:"column:room_name;size:128"`
	StartedAt      int64  `gorm:"column:started_at;not null;default:0"`
	AnsweredAt     int64  `gorm:"column:answered_at;not null;default:0"`
	EndedAt        int64  `gorm:"column:ended_at;not null;default:0"`
	DurationSec    int32  `gorm:"column:duration_sec;not null;default:0"`
	CreatedAt      int64  `gorm:"column:created_at;not null;default:0"`
	UpdatedAt      int64  `gorm:"column:updated_at;not null;default:0"`
}

// TableName 覆盖 GORM 默认表名。
func (Call) TableName() string { return TableRtcCalls }
