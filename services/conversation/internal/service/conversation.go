// Package service 实现会话业务逻辑，包括会话 CRUD、单聊/群聊会话创建、序列号管理、离线推送等。
package service

import (
	"context"
	"errors"
	"hash/fnv"
	"sort"
	"time"

	apperrors "conversation/internal/errors"
	"conversation/internal/types"
	"conversation/internal/types/interfaces"

	"gorm.io/gorm"
)

// conversationService 实现 ConversationService 接口。
type conversationService struct {
	repo interfaces.ConversationRepository
}

// NewConversationService 创建会话服务实例。
func NewConversationService(repo interfaces.ConversationRepository) interfaces.ConversationService {
	return &conversationService{repo: repo}
}

// --------------- 辅助函数 ---------------

// singleChatID 生成单聊会话 ID，格式为 "si_<较小ID>_<较大ID>"，保证双向一致。
func singleChatID(sendID, recvID string) string {
	a, b := sendID, recvID
	if a > b {
		a, b = b, a
	}
	return "si_" + a + "_" + b
}

// groupChatID 生成群聊会话 ID，格式为 "gid_<群ID>"。
func groupChatID(groupID string) string {
	return "gid_" + groupID
}

// buildPatch 将 ConversationUpdate 转换为字段更新 map，nil 字段表示不修改。
func buildPatch(u *types.ConversationUpdate) map[string]any {
	patch := map[string]any{}
	if u == nil {
		return patch
	}
	if u.ConversationType != nil {
		patch["conversation_type"] = *u.ConversationType
	}
	if u.UserID != nil {
		patch["user_id"] = *u.UserID
	}
	if u.GroupID != nil {
		patch["group_id"] = *u.GroupID
	}
	if u.RecvMsgOpt != nil {
		patch["recv_msg_opt"] = *u.RecvMsgOpt
	}
	if u.IsPinned != nil {
		patch["is_pinned"] = *u.IsPinned
	}
	if u.AttachedInfo != nil {
		patch["attached_info"] = *u.AttachedInfo
	}
	if u.IsPrivateChat != nil {
		patch["is_private_chat"] = *u.IsPrivateChat
	}
	if u.Ex != nil {
		patch["ex"] = *u.Ex
	}
	if u.BurnDuration != nil {
		patch["burn_duration"] = *u.BurnDuration
	}
	if u.MinSeq != nil {
		patch["min_seq"] = *u.MinSeq
	}
	if u.MaxSeq != nil {
		patch["max_seq"] = *u.MaxSeq
	}
	if u.GroupAtType != nil {
		patch["group_at_type"] = *u.GroupAtType
	}
	if u.MsgDestructTime != nil {
		patch["msg_destruct_time"] = *u.MsgDestructTime
	}
	if u.IsMsgDestruct != nil {
		patch["is_msg_destruct"] = *u.IsMsgDestruct
	}
	if u.LatestMsgDestructTime != nil {
		patch["latest_msg_destruct_time"] = *u.LatestMsgDestructTime
	}
	return patch
}

// conversationIDsHash 计算会话 ID 列表的 FNV 哈希值，先排序以保证确定性。
func conversationIDsHash(ids []string) uint64 {
	sorted := make([]string, len(ids))
	copy(sorted, ids)
	sort.Strings(sorted)
	h := fnv.New64a()
	for _, id := range sorted {
		_, _ = h.Write([]byte(id))
		_, _ = h.Write([]byte{0})
	}
	return h.Sum64()
}

// --------------- 会话 CRUD ---------------

// SetConversation 设置或更新单个会话（upsert）。
func (s *conversationService) SetConversation(ctx context.Context, conv *types.Conversation) error {
	if conv.OwnerUserID == "" || conv.ConversationID == "" {
		return apperrors.NewValidationError("owner_user_id and conversation_id are required")
	}
	if conv.CreateTime.IsZero() {
		conv.CreateTime = time.Now()
	}
	if err := s.repo.Upsert(ctx, conv); err != nil {
		return apperrors.NewInternalError("failed to set conversation").WithDetails(err)
	}
	return nil
}

// GetConversation 获取指定用户的指定会话。
func (s *conversationService) GetConversation(ctx context.Context, ownerUserID, conversationID string) (*types.Conversation, error) {
	conv, err := s.repo.Get(ctx, ownerUserID, conversationID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.NewConversationNotFoundError()
		}
		return nil, apperrors.NewInternalError("failed to get conversation").WithDetails(err)
	}
	return conv, nil
}

// GetConversations 批量获取用户会话。
func (s *conversationService) GetConversations(ctx context.Context, ownerUserID string, ids []string) ([]types.Conversation, error) {
	convs, err := s.repo.ListByOwnerIDs(ctx, ownerUserID, ids)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	return convs, nil
}

// GetAllConversations 获取用户所有会话。
func (s *conversationService) GetAllConversations(ctx context.Context, ownerUserID string) ([]types.Conversation, error) {
	convs, err := s.repo.ListByOwner(ctx, ownerUserID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	return convs, nil
}

// GetSortedConversationList 分页获取排序后的会话列表。
func (s *conversationService) GetSortedConversationList(ctx context.Context, userID string, ids []string, offset, limit int) ([]types.Conversation, int64, error) {
	convs, total, err := s.repo.ListByOwnerSorted(ctx, userID, ids, offset, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	return convs, total, nil
}

// --------------- 会话创建 ---------------

// CreateSingleChatConversations 为两个用户创建双向单聊会话。
func (s *conversationService) CreateSingleChatConversations(ctx context.Context, sendID, recvID, conversationID string, conversationType int) error {
	if sendID == "" || recvID == "" {
		return apperrors.NewValidationError("send_id and recv_id are required")
	}
	if conversationID == "" {
		conversationID = singleChatID(sendID, recvID)
	}
	if conversationType == 0 {
		conversationType = 1
	}
	now := time.Now()
	rows := []*types.Conversation{
		{OwnerUserID: sendID, ConversationID: conversationID, ConversationType: conversationType, UserID: recvID, CreateTime: now},
		{OwnerUserID: recvID, ConversationID: conversationID, ConversationType: conversationType, UserID: sendID, CreateTime: now},
	}
	for _, c := range rows {
		if err := s.repo.Upsert(ctx, c); err != nil {
			return apperrors.NewInternalError("failed to create single chat conversation").WithDetails(err)
		}
	}
	return nil
}

// CreateGroupChatConversations 为群组成员批量创建群聊会话。
func (s *conversationService) CreateGroupChatConversations(ctx context.Context, groupID string, userIDs []string) error {
	if groupID == "" {
		return apperrors.NewValidationError("group_id is required")
	}
	if len(userIDs) == 0 {
		return apperrors.NewValidationError("user_ids is required")
	}
	conversationID := groupChatID(groupID)
	now := time.Now()
	for _, uid := range userIDs {
		conv := &types.Conversation{
			OwnerUserID:      uid,
			ConversationID:   conversationID,
			ConversationType: 2, // 群聊类型
			GroupID:          groupID,
			CreateTime:       now,
		}
		if err := s.repo.Upsert(ctx, conv); err != nil {
			return apperrors.NewInternalError("failed to create group chat conversation").WithDetails(err)
		}
	}
	return nil
}

// --------------- 序列号管理 ---------------

// SetConversationMaxSeq 设置会话最大序列号（标记已读位置）。
func (s *conversationService) SetConversationMaxSeq(ctx context.Context, conversationID string, ownerIDs []string, maxSeq int64) error {
	if err := s.repo.SetSeq(ctx, conversationID, ownerIDs, "max_seq", maxSeq); err != nil {
		return apperrors.NewInternalError("failed to set max seq").WithDetails(err)
	}
	return nil
}

// SetConversationMinSeq 设置会话最小序列号。
func (s *conversationService) SetConversationMinSeq(ctx context.Context, conversationID string, ownerIDs []string, minSeq int64) error {
	if err := s.repo.SetSeq(ctx, conversationID, ownerIDs, "min_seq", minSeq); err != nil {
		return apperrors.NewInternalError("failed to set min seq").WithDetails(err)
	}
	return nil
}

// --------------- 查询类 ---------------

// GetConversationIDs 获取用户的所有会话 ID。
func (s *conversationService) GetConversationIDs(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.repo.ListConversationIDsByOwner(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversation ids").WithDetails(err)
	}
	return ids, nil
}

// GetConversationsByConversationID 根据会话 ID 查询所有用户的会话记录。
func (s *conversationService) GetConversationsByConversationID(ctx context.Context, ids []string) ([]types.Conversation, error) {
	convs, err := s.repo.ListByConversationIDs(ctx, ids)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	return convs, nil
}

// GetOwnerConversation 分页获取用户所属会话。
func (s *conversationService) GetOwnerConversation(ctx context.Context, userID string, offset, limit int) ([]types.Conversation, int64, error) {
	convs, total, err := s.repo.ListByOwnerPaginated(ctx, userID, offset, limit)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	return convs, total, nil
}

// GetFullOwnerConversationIDs 获取用户完整会话 ID 列表。
func (s *conversationService) GetFullOwnerConversationIDs(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.repo.ListConversationIDsByOwner(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversation ids").WithDetails(err)
	}
	return ids, nil
}

// GetIncrementalConversation 获取用户增量会话（当前返回全量）。
func (s *conversationService) GetIncrementalConversation(ctx context.Context, userID string) ([]types.Conversation, error) {
	convs, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	return convs, nil
}

// --------------- 批量操作 ---------------

// SetConversations 为多个用户批量设置同一会话。
func (s *conversationService) SetConversations(ctx context.Context, userIDs []string, conv *types.Conversation) error {
	if conv.ConversationID == "" {
		return apperrors.NewValidationError("conversation_id is required")
	}
	if conv.CreateTime.IsZero() {
		conv.CreateTime = time.Now()
	}
	for _, uid := range userIDs {
		row := *conv
		row.OwnerUserID = uid
		if err := s.repo.Upsert(ctx, &row); err != nil {
			return apperrors.NewInternalError("failed to set conversations").WithDetails(err)
		}
	}
	return nil
}

// UpdateConversation 更新多个用户的指定会话的可选字段。
func (s *conversationService) UpdateConversation(ctx context.Context, conversationID string, userIDs []string, patch *types.ConversationUpdate) error {
	if conversationID == "" {
		return apperrors.NewValidationError("conversation_id is required")
	}
	m := buildPatch(patch)
	if err := s.repo.BulkUpdateFields(ctx, conversationID, userIDs, m); err != nil {
		return apperrors.NewInternalError("failed to update conversation").WithDetails(err)
	}
	return nil
}

// UpdateConversationsByUser 更新用户全部会话的扩展字段。
func (s *conversationService) UpdateConversationsByUser(ctx context.Context, userID, ex string) error {
	if err := s.repo.UpdateExByOwner(ctx, userID, ex); err != nil {
		return apperrors.NewInternalError("failed to update conversations").WithDetails(err)
	}
	return nil
}

// DeleteConversations 删除用户的指定会话。
func (s *conversationService) DeleteConversations(ctx context.Context, ownerUserID string, ids []string) error {
	if err := s.repo.Delete(ctx, ownerUserID, ids); err != nil {
		return apperrors.NewInternalError("failed to delete conversations").WithDetails(err)
	}
	return nil
}

// --------------- 通知/推送相关 ---------------

// GetRecvMsgNotNotifyUserIDs 获取群内不接受通知的用户 ID 列表（recv_msg_opt == 2）。
func (s *conversationService) GetRecvMsgNotNotifyUserIDs(ctx context.Context, groupID string) ([]string, error) {
	convs, err := s.repo.ListByGroupID(ctx, groupID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	var ids []string
	for _, c := range convs {
		if c.RecvMsgOpt == 2 {
			ids = append(ids, c.OwnerUserID)
		}
	}
	return ids, nil
}

// GetConversationOfflinePushUserIDs 获取需要离线推送的用户 ID（排除 recv_msg_opt == 1 静音用户）。
func (s *conversationService) GetConversationOfflinePushUserIDs(ctx context.Context, conversationID string, userIDs []string) ([]string, error) {
	convs, err := s.repo.ListByConversationIDAndOwners(ctx, conversationID, userIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	var ids []string
	for _, c := range convs {
		if c.RecvMsgOpt != 1 { // 1 = 静音/不接收 → 不推送
			ids = append(ids, c.OwnerUserID)
		}
	}
	return ids, nil
}

// GetConversationNotReceiveMessageUserIDs 获取会话中设置不接收消息的用户 ID（recv_msg_opt == 1）。
func (s *conversationService) GetConversationNotReceiveMessageUserIDs(ctx context.Context, conversationID string) ([]string, error) {
	convs, err := s.repo.ListByConversationID(ctx, conversationID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations").WithDetails(err)
	}
	var ids []string
	for _, c := range convs {
		if c.RecvMsgOpt == 1 {
			ids = append(ids, c.OwnerUserID)
		}
	}
	return ids, nil
}

// --------------- 置顶/静音 ---------------

// GetPinnedConversationIDs 获取用户已置顶的会话 ID 列表。
func (s *conversationService) GetPinnedConversationIDs(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.repo.ListPinnedIDsByOwner(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list pinned conversations").WithDetails(err)
	}
	return ids, nil
}

// GetNotNotifyConversationIDs 获取用户静音的会话 ID 列表。
func (s *conversationService) GetNotNotifyConversationIDs(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.repo.ListNotNotifyIDsByOwner(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list not-notify conversations").WithDetails(err)
	}
	return ids, nil
}

// --------------- 消息清理 ---------------

// ClearUserConversationMsg 清理到期的阅后即焚消息，受 limit 参数限制。
func (s *conversationService) ClearUserConversationMsg(ctx context.Context, timestamp int64, limit int) (int, error) {
	now := timestamp
	if now == 0 {
		now = time.Now().Unix()
	}
	convs, err := s.repo.ListNeedClearMsg(ctx, now)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to list conversations needing clear").WithDetails(err)
	}
	if limit > 0 && len(convs) > limit {
		convs = convs[:limit]
	}
	ids := make([]string, 0, len(convs))
	for _, c := range convs {
		ids = append(ids, c.ConversationID)
	}
	n, err := s.repo.ClearMsgSeqs(ctx, ids)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to clear conversation messages").WithDetails(err)
	}
	return int(n), nil
}

// GetConversationsNeedClearMsg 获取需要清理阅后即焚消息的会话列表。
func (s *conversationService) GetConversationsNeedClearMsg(ctx context.Context) ([]types.Conversation, error) {
	convs, err := s.repo.ListNeedClearMsg(ctx, time.Now().Unix())
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list conversations needing clear").WithDetails(err)
	}
	return convs, nil
}

// --------------- 哈希 ---------------

// GetUserConversationIDsHash 计算用户会话 ID 列表的 FNV 哈希值，用于客户端同步校验。
func (s *conversationService) GetUserConversationIDsHash(ctx context.Context, ownerUserID string) (uint64, error) {
	ids, err := s.repo.ListConversationIDsByOwner(ctx, ownerUserID)
	if err != nil {
		return 0, apperrors.NewInternalError("failed to list conversation ids").WithDetails(err)
	}
	return conversationIDsHash(ids), nil
}
