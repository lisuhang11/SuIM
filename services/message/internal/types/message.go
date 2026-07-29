// Package types 定义消息服务的领域模型。
// 存储模型对应 OpenIM 的 MongoDB 设计（msg / seq / seq_user），
// 在 MySQL 中每个消息是 msg_info 的一行，会话 seq 在 seq_conversation，
// 用户级 seq 在 seq_user。模型仅本服务内部使用，
// 跨层仅通过此包共享（不应被其他服务直接引用）。
package types

// 消息状态常量。
const (
	MsgStatusNormal = 0 // 正常
	MsgStatusRevoke = 1 // 已撤回
)

// MsgDataModel 单条聊天消息的核心内容。
type MsgDataModel struct {
	SendID           string `gorm:"column:send_id;size:64;not null;uniqueIndex:uk_sender_client_msg"`                   // 发送者 ID
	RecvID           string `gorm:"column:recv_id;size:64;not null;default:''"`                                         // 接收者 ID
	GroupID          string `gorm:"column:group_id;size:64;not null;default:''"`                                        // 群组 ID
	ClientMsgID      string `gorm:"column:client_msg_id;size:128;not null;default:'';uniqueIndex:uk_sender_client_msg"` // 客户端消息 ID（发送者范围内幂等）
	ServerMsgID      string `gorm:"column:server_msg_id;size:128;not null;default:''"`                                  // 服务端消息 ID
	SenderPlatformID int    `gorm:"column:sender_platform_id;not null;default:0"`                                       // 发送平台
	SenderNickname   string `gorm:"column:sender_nickname;size:255;not null;default:''"`                                // 发送者昵称
	SenderFaceURL    string `gorm:"column:sender_face_url;size:512;not null;default:''"`                                // 发送者头像
	SessionType      int    `gorm:"column:session_type;not null;default:0"`                                             // 会话类型
	MsgFrom          int    `gorm:"column:msg_from;not null;default:0"`                                                 // 消息来源
	ContentType      int    `gorm:"column:content_type;not null;default:0"`                                             // 内容类型
	Content          string `gorm:"column:content;type:text"`                                                           // 消息内容
	Seq              int64  `gorm:"column:seq;not null;default:0;index:idx_seq"`                                        // 会话内序号
	SendTime         int64  `gorm:"column:send_time;not null;default:0;index:idx_send_time"`                            // 发送时间
	CreateTime       int64  `gorm:"column:create_time;not null;default:0"`                                              // 创建时间
	Status           int    `gorm:"column:status;not null;default:0"`                                                   // 消息状态
	Options          string `gorm:"column:options;type:text"`                                                           // 选项
	AtUserIDList     string `gorm:"column:at_user_id_list;type:text"`                                                   // @用户列表（JSON）
	AttachedInfo     string `gorm:"column:attached_info;type:text"`                                                     // 附加信息
	Ex               string `gorm:"column:ex;type:text"`                                                                // 扩展字段
}

// OfflinePushModel 消息附带的离线推送信息。
type OfflinePushModel struct {
	OfflinePushTitle         string `gorm:"column:offline_push_title;size:255"`                     // 推送标题
	OfflinePushDesc          string `gorm:"column:offline_push_desc;size:512"`                      // 推送描述
	OfflinePushEx            string `gorm:"column:offline_push_ex;type:text"`                       // 推送扩展
	OfflinePushIOSound       string `gorm:"column:offline_push_ios_sound;size:255"`                 // iOS 推送声音
	OfflinePushIOSBadgeCount int    `gorm:"column:offline_push_ios_badge_count;not null;default:0"` // iOS 角标数
}

// RevokeModel 记录消息撤回信息。
type RevokeModel struct {
	RevokeRole     int    `gorm:"column:revoke_role"`              // 撤回角色
	RevokeUserID   string `gorm:"column:revoke_user_id;size:64"`   // 撤回用户 ID
	RevokeNickname string `gorm:"column:revoke_nickname;size:255"` // 撤回者昵称
	RevokeTime     int64  `gorm:"column:revoke_time"`              // 撤回时间
}

// MsgInfoModel msg_info 表中的一行，即一条存储的消息。
// 对应 OpenIM MongoDB 文档（doc_id = conversationID:docIndex，每文档最多 100 条）。
// MySQL 下每条消息独立存储以利于查询，同时保留 doc_id/msg_index 保持批次概念。
type MsgInfoModel struct {
	ID             int64  `gorm:"primaryKey;autoIncrement;column:id"`                                 // 自增主键
	DocID          string `gorm:"column:doc_id;size:255;not null;index:idx_doc_id"`                   // 批次文档 ID
	MsgIndex       int    `gorm:"column:msg_index;not null;default:0"`                                // 文档内消息索引
	DelList        string `gorm:"column:del_list;type:text"`                                          // 删除列表
	IsRead         bool   `gorm:"column:is_read;not null;default:0"`                                  // 是否已读
	ConversationID string `gorm:"column:conversation_id;size:255;not null;index:idx_conversation_id"` // 会话 ID

	// RecvUserIDs 仅发送时使用，不持久化（gorm:"-"）。
	RecvUserIDs []string `gorm:"-"`

	MsgDataModel     // 嵌入消息核心数据
	OfflinePushModel // 嵌入离线推送信息
	RevokeModel      // 嵌入撤回信息
}

// TableName 覆盖 GORM 默认复数命名。
func (MsgInfoModel) TableName() string { return TableMsgInfo }

// SeqConversation 会话级序列号，对应 MongoDB seq 集合（每个会话一条记录）。
type SeqConversation struct {
	ID             int64  `gorm:"primaryKey;autoIncrement;column:id"`
	ConversationID string `gorm:"column:conversation_id;size:255;not null;uniqueIndex:idx_conversation_id"` // 会话 ID
	MaxSeq         int64  `gorm:"column:max_seq;not null;default:0"`                                        // 最大 seq
	MinSeq         int64  `gorm:"column:min_seq;not null;default:0"`                                        // 最小 seq
}

// TableName 覆盖 GORM 默认复数命名。
func (SeqConversation) TableName() string { return TableSeqConversation }

// SeqUser 用户级会话序列号，对应 MongoDB seq_user 集合。
//   - MaxSeq：用户在该会话中可见的最大消息 seq
//   - MinSeq：用户在该会话中可见的最小消息 seq（清理后）
//   - ReadSeq：用户已读的最大消息 seq（未读数 = MaxSeq - ReadSeq）
type SeqUser struct {
	ID             int64  `gorm:"primaryKey;autoIncrement;column:id"`
	UserID         string `gorm:"column:user_id;size:64;not null;uniqueIndex:idx_user_conversation"`          // 用户 ID
	ConversationID string `gorm:"column:conversation_id;size:255;not null;uniqueIndex:idx_user_conversation"` // 会话 ID
	MinSeq         int64  `gorm:"column:min_seq;not null;default:0"`                                          // 最小可见 seq
	MaxSeq         int64  `gorm:"column:max_seq;not null;default:0"`                                          // 最大可见 seq
	ReadSeq        int64  `gorm:"column:read_seq;not null;default:0"`                                         // 已读 seq
}

// MessageDelete 记录用户级消息删除，不影响同一会话中的其他用户。
type MessageDelete struct {
	ID        int64  `gorm:"primaryKey;autoIncrement;column:id"`
	MessageID int64  `gorm:"column:message_id;not null;uniqueIndex:uk_user_message"`
	UserID    string `gorm:"column:user_id;size:64;not null;uniqueIndex:uk_user_message;index:idx_message_delete_user"`
	CreatedAt int64  `gorm:"column:created_at;not null"`
}

func (MessageDelete) TableName() string { return "msg_delete" }

// TableName 覆盖 GORM 默认复数命名。
func (SeqUser) TableName() string { return TableSeqUser }

// Message 是领域层使用的存储消息模型别名。仓库层以上通过此名称引用消息。
type Message = MsgInfoModel
