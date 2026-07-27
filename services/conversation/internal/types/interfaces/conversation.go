// Package interfaces 定义会话服务的接口契约，解耦业务逻辑与持久化实现。
package interfaces

import (
	"context"

	"conversation/internal/types"
)

// ConversationService 定义会话业务逻辑的接口契约，由服务层实现、gRPC handler 消费。
type ConversationService interface {
	SetConversation(ctx context.Context, conv *types.Conversation) error
	GetConversation(ctx context.Context, ownerUserID, conversationID string) (*types.Conversation, error)
	GetConversations(ctx context.Context, ownerUserID string, ids []string) ([]types.Conversation, error)
	GetAllConversations(ctx context.Context, ownerUserID string) ([]types.Conversation, error)
	GetSortedConversationList(ctx context.Context, userID string, ids []string, offset, limit int) ([]types.Conversation, int64, error)
	CreateSingleChatConversations(ctx context.Context, sendID, recvID, conversationID string, conversationType int) error
	CreateGroupChatConversations(ctx context.Context, groupID string, userIDs []string) error
	SetConversationMaxSeq(ctx context.Context, conversationID string, ownerIDs []string, maxSeq int64) error
	SetConversationMinSeq(ctx context.Context, conversationID string, ownerIDs []string, minSeq int64) error
	GetConversationIDs(ctx context.Context, userID string) ([]string, error)
	SetConversations(ctx context.Context, userIDs []string, conv *types.Conversation) error
	UpdateConversation(ctx context.Context, conversationID string, userIDs []string, patch *types.ConversationUpdate) error
	GetConversationsByConversationID(ctx context.Context, ids []string) ([]types.Conversation, error)
	GetRecvMsgNotNotifyUserIDs(ctx context.Context, groupID string) ([]string, error)
	GetConversationOfflinePushUserIDs(ctx context.Context, conversationID string, userIDs []string) ([]string, error)
	GetConversationNotReceiveMessageUserIDs(ctx context.Context, conversationID string) ([]string, error)
	GetPinnedConversationIDs(ctx context.Context, userID string) ([]string, error)
	GetNotNotifyConversationIDs(ctx context.Context, userID string) ([]string, error)
	DeleteConversations(ctx context.Context, ownerUserID string, ids []string) error
	UpdateConversationsByUser(ctx context.Context, userID, ex string) error
	GetUserConversationIDsHash(ctx context.Context, ownerUserID string) (uint64, error)
	GetOwnerConversation(ctx context.Context, userID string, offset, limit int) ([]types.Conversation, int64, error)
	ClearUserConversationMsg(ctx context.Context, timestamp int64, limit int) (int, error)
	GetConversationsNeedClearMsg(ctx context.Context) ([]types.Conversation, error)
	GetFullOwnerConversationIDs(ctx context.Context, userID string) ([]string, error)
	GetIncrementalConversation(ctx context.Context, userID string) ([]types.Conversation, error)
}

// ConversationRepository 定义会话持久化操作的接口契约。
type ConversationRepository interface {
	Upsert(ctx context.Context, conv *types.Conversation) error
	Get(ctx context.Context, ownerUserID, conversationID string) (*types.Conversation, error)
	ListByOwner(ctx context.Context, ownerUserID string) ([]types.Conversation, error)
	ListByOwnerIDs(ctx context.Context, ownerUserID string, ids []string) ([]types.Conversation, error)
	ListByOwnerPaginated(ctx context.Context, ownerUserID string, offset, limit int) ([]types.Conversation, int64, error)
	ListByOwnerSorted(ctx context.Context, ownerUserID string, ids []string, offset, limit int) ([]types.Conversation, int64, error)
	ListByConversationID(ctx context.Context, conversationID string) ([]types.Conversation, error)
	ListByConversationIDs(ctx context.Context, ids []string) ([]types.Conversation, error)
	ListByConversationIDAndOwners(ctx context.Context, conversationID string, ownerIDs []string) ([]types.Conversation, error)
	ListByGroupID(ctx context.Context, groupID string) ([]types.Conversation, error)
	UpdateFields(ctx context.Context, ownerUserID, conversationID string, patch map[string]any) error
	BulkUpdateFields(ctx context.Context, conversationID string, ownerIDs []string, patch map[string]any) error
	SetSeq(ctx context.Context, conversationID string, ownerIDs []string, field string, value int64) error
	ListConversationIDsByOwner(ctx context.Context, ownerUserID string) ([]string, error)
	ListPinnedIDsByOwner(ctx context.Context, ownerUserID string) ([]string, error)
	ListNotNotifyIDsByOwner(ctx context.Context, ownerUserID string) ([]string, error)
	Delete(ctx context.Context, ownerUserID string, ids []string) error
	UpdateExByOwner(ctx context.Context, ownerUserID, ex string) error
	ListNeedClearMsg(ctx context.Context, now int64) ([]types.Conversation, error)
	ClearMsgSeqs(ctx context.Context, conversationIDs []string) (int64, error)
}
