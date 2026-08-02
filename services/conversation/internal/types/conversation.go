// Package types 定义会话服务的 GORM 领域模型及更新值对象。
package types

import "time"

// Conversation 会话领域模型，表示用户视角下的聊天线程（单聊或群聊）。
// 直接映射到 migrations/conversation.sql 中的 conversation 表，
// 主键为 (owner_user_id, conversation_id)。
type Conversation struct {
	OwnerUserID           string    `gorm:"primaryKey;column:owner_user_id;size:64;not null"`      // 会话所有者用户 ID
	ConversationID        string    `gorm:"primaryKey;column:conversation_id;size:128;not null"`    // 会话 ID
	ConversationType      int       `gorm:"column:conversation_type;not null;default:0"`            // 会话类型（1 单聊 2 群聊）
	UserID                string    `gorm:"column:user_id;size:64;not null;default:''"`             // 对方用户 ID（单聊）
	GroupID               string    `gorm:"column:group_id;size:64;not null;default:''"`            // 群组 ID（群聊）
	RecvMsgOpt            int       `gorm:"column:recv_msg_opt;not null;default:0"`                 // 消息接收选项（0 正常 1 不接收 2 不通知）
	IsPinned              bool      `gorm:"column:is_pinned;not null;default:0"`                    // 是否置顶
	IsPrivateChat         bool      `gorm:"column:is_private_chat;not null;default:0"`              // 是否为私聊
	BurnDuration          int       `gorm:"column:burn_duration;not null;default:0"`                // 阅后即焚时长（秒）
	GroupAtType           int       `gorm:"column:group_at_type;not null;default:0"`                // @类型
	AttachedInfo          string    `gorm:"column:attached_info;size:1024;not null;default:''"`     // 附加信息
	Ex                    string    `gorm:"column:ex;size:1024;not null;default:''"`                // 扩展字段
	MinSeq                int64     `gorm:"column:min_seq;not null;default:0"`                      // 最小序列号
	MaxSeq                int64     `gorm:"column:max_seq;not null;default:0"`                      // 最大（已读）序列号
	IsMsgDestruct         bool      `gorm:"column:is_msg_destruct;not null;default:0"`              // 是否阅后即焚
	MsgDestructTime       int64     `gorm:"column:msg_destruct_time;not null;default:0"`            // 消息销毁时间
	LatestMsgDestructTime int64     `gorm:"column:latest_msg_destruct_time;not null;default:0"`     // 最新消息销毁时间
	CreateTime            time.Time `gorm:"column:create_time;not null"`                            // 创建时间
}

// TableName 覆盖 GORM 默认复数表名。
func (Conversation) TableName() string { return "conversation" }

// LatestMsg 会话最后一条消息预览（由 message.GetLastMessage 填充，不入 conversation 表）。
type LatestMsg struct {
	ConversationID string `gorm:"column:conversation_id"`
	ServerMsgID    string `gorm:"column:server_msg_id"`
	ClientMsgID    string `gorm:"column:client_msg_id"`
	SessionType    int    `gorm:"column:session_type"`
	SendID         string `gorm:"column:send_id"`
	RecvID         string `gorm:"column:recv_id"`
	SenderNickname string `gorm:"column:sender_nickname"`
	SenderFaceURL  string `gorm:"column:sender_face_url"`
	GroupID        string `gorm:"column:group_id"`
	MsgFrom        int    `gorm:"column:msg_from"`
	ContentType    int    `gorm:"column:content_type"`
	Content        string `gorm:"column:content"`
	Ex             string `gorm:"column:ex"`
	SendTime       int64  `gorm:"column:send_time"`
}

// ConversationUpdate 携带 UpdateConversation 可选更新字段。
// nil 指针表示"不修改该字段"。
type ConversationUpdate struct {
	ConversationType      *int    // 会话类型
	UserID                *string // 对方用户 ID
	GroupID               *string // 群组 ID
	RecvMsgOpt            *int    // 消息接收选项
	IsPinned              *bool   // 是否置顶
	AttachedInfo          *string // 附加信息
	IsPrivateChat         *bool   // 是否私聊
	Ex                    *string // 扩展字段
	BurnDuration          *int    // 阅后即焚时长
	MinSeq                *int64  // 最小序列号
	MaxSeq                *int64  // 最大序列号
	GroupAtType           *int    // @类型
	MsgDestructTime       *int64  // 消息销毁时间
	IsMsgDestruct         *bool   // 是否阅后即焚
	LatestMsgDestructTime *int64  // 最新消息销毁时间
}
