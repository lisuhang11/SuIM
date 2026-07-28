package types

// 消息存储模型的数据库表名。
const (
	TableMsgInfo         = "msg_info"          // 消息主表
	TableSeqConversation = "seq_conversation"  // 会话级 seq 表
	TableSeqUser         = "seq_user"          // 用户级 seq 表
)
