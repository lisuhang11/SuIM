// Package interfaces 定义消息领域的服务契约和仓库契约。
package interfaces

import (
	"context"

	"message/internal/types"
)

// MessageService 业务层契约，由 service 层实现，由 gRPC handler 消费。
type MessageService interface {
	// SendMsg 持久化消息，分配会话级 seq。基于 client_msg_id 幂等。
	// msg.RecvUserIDs（不持久化）用于推进每个接收者的 seq_user.max_seq。
	SendMsg(ctx context.Context, msg *types.Message) (*types.Message, error)
	// GetHistoryMessages 加载锚点 seq 附近的游标分页消息。
	// 返回匹配的消息列表、分页前匹配行数（供 handler 推导 is_end）及错误。
	GetHistoryMessages(ctx context.Context, userID, conversationID string, anchorSeq int64, limit, order int) ([]types.Message, int64, error)
	// GetMessagesBySeq 按 seq 列表获取消息。
	GetMessagesBySeq(ctx context.Context, userID, conversationID string, seqs []int64) ([]types.Message, error)
	// GetMessagesByClientMsgIDs 按客户端消息 ID 列表获取消息。
	GetMessagesByClientMsgIDs(ctx context.Context, userID string, clientMsgIDs []string) ([]types.Message, error)
	// RevokeMsg 撤回消息（仅发送者可撤回）。
	RevokeMsg(ctx context.Context, conversationID, clientMsgID, sendID string, revokeRole int32, revokeNickname string) error
	// MarkMsgsAsRead 标记消息为已读。
	MarkMsgsAsRead(ctx context.Context, conversationID, userID string, seq int64) error
	// DeleteMsgs 删除消息。
	DeleteMsgs(ctx context.Context, userID, conversationID string, seqs []int64) error
	// GetMaxSeq 返回用户在指定会话中的 max/min seq；conversationIDs 为空则返回全部。
	GetMaxSeq(ctx context.Context, userID string, conversationIDs []string) (map[string]types.SeqBounds, error)
	// GetConversationsHasReadAndMaxSeq 返回用户在指定会话中的 max_seq、已读 seq 与 max_seq_time。
	GetConversationsHasReadAndMaxSeq(ctx context.Context, userID string, conversationIDs []string) (map[string]types.SeqPair, error)
	// GetActiveConversation 返回指定会话的 max_seq + last_time；limit>0 时按 last_time 降序截断。
	GetActiveConversation(ctx context.Context, userID string, conversationIDs []string, limit int64) ([]types.ActiveConversation, error)
	// GetLastMessage 返回每个会话对用户可见的最后一条消息。
	GetLastMessage(ctx context.Context, userID string, conversationIDs []string) (map[string]types.Message, error)
}

// MessageRepository 持久层契约。
type MessageRepository interface {
	// SendMessage 事务内执行：原子分配会话下一个 seq，
	// 插入消息，推进发送者和接收者的 seq_user.max_seq，
	// 并尽最大努力同步 conversation.max_seq 保持会话服务未读模型一致。
	SendMessage(ctx context.Context, msg *types.Message, recvUserIDs []string) error
	// GetByClientMsgIDs 按客户端消息 ID 批量查询。
	GetByClientMsgIDs(ctx context.Context, userID string, clientMsgIDs []string) ([]types.Message, error)
	// GetBySenderClientMsgIDs 用于发送幂等检查，不受用户删除标记影响。
	GetBySenderClientMsgIDs(ctx context.Context, senderID string, clientMsgIDs []string) ([]types.Message, error)
	// GetBySeqs 按 seq 列表查询。
	GetBySeqs(ctx context.Context, userID, conversationID string, seqs []int64) ([]types.Message, error)
	// GetHistory 返回分页消息和截断前匹配行数（供调用方推导 is_end）。
	GetHistory(ctx context.Context, userID, conversationID string, anchorSeq int64, limit, order int) ([]types.Message, int64, error)
	// Revoke 撤回消息（事务内执行），返回撤回后的消息快照供 tip 推送。
	Revoke(ctx context.Context, conversationID, clientMsgID, sendID string, revokeRole int32, revokeNickname string) (*types.Message, error)
	// SetReadSeq 推进指定用户的已读游标。
	SetReadSeq(ctx context.Context, userID, conversationID string, seq int64) error
	// DeleteForUser 仅对指定用户隐藏消息。
	DeleteForUser(ctx context.Context, userID, conversationID string, seqs []int64) error
	// ListSeqUser 返回 user 的 seq_user 行。conversationIDs 为空则返回该用户全部行。
	ListSeqUser(ctx context.Context, userID string, conversationIDs []string) ([]types.SeqUser, error)
	// MapSendTimeByConvSeq 按 (conversation_id, seq) 批量查 send_time；key 为 conversation_id。
	MapSendTimeByConvSeq(ctx context.Context, conversationSeqs map[string]int64) (map[string]int64, error)
	// GetLastMessage 批量取每个会话对用户可见的最后一条消息。
	GetLastMessage(ctx context.Context, userID string, conversationIDs []string) (map[string]types.Message, error)
}
