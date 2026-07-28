// Package repository 提供会话的 GORM 持久化实现。
package repository

import (
	"context"

	"conversation/internal/types"
	"conversation/internal/types/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// conversationRepository 是 interfaces.ConversationRepository 的 GORM 实现。
type conversationRepository struct {
	db *gorm.DB
}

// NewConversationRepository 创建基于 GORM 的 ConversationRepository 实现。
func NewConversationRepository(db *gorm.DB) interfaces.ConversationRepository {
	return &conversationRepository{db: db}
}

// Upsert 插入或更新会话记录，主键冲突时更新全部字段。
func (r *conversationRepository) Upsert(ctx context.Context, conv *types.Conversation) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "owner_user_id"}, {Name: "conversation_id"}},
		UpdateAll: true,
	}).Create(conv).Error
}

// Get 根据所有者和会话 ID 查询会话记录。
func (r *conversationRepository) Get(ctx context.Context, ownerUserID, conversationID string) (*types.Conversation, error) {
	var conv types.Conversation
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND conversation_id = ?", ownerUserID, conversationID).
		First(&conv).Error
	if err != nil {
		return nil, err
	}
	return &conv, nil
}

// ListByOwner 查询用户的所有会话。
func (r *conversationRepository) ListByOwner(ctx context.Context, ownerUserID string) ([]types.Conversation, error) {
	var convs []types.Conversation
	err := r.db.WithContext(ctx).Where("owner_user_id = ?", ownerUserID).Find(&convs).Error
	return convs, err
}

// ListByOwnerIDs 查询用户指定的多个会话。
func (r *conversationRepository) ListByOwnerIDs(ctx context.Context, ownerUserID string, ids []string) ([]types.Conversation, error) {
	var convs []types.Conversation
	if len(ids) == 0 {
		return convs, nil
	}
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND conversation_id IN ?", ownerUserID, ids).
		Find(&convs).Error
	return convs, err
}

// ListByOwnerPaginated 分页查询用户会话，按置顶优先、最新消息销毁时间及创建时间排序。
func (r *conversationRepository) ListByOwnerPaginated(ctx context.Context, ownerUserID string, offset, limit int) ([]types.Conversation, int64, error) {
	var convs []types.Conversation
	var total int64
	if err := r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("owner_user_id = ?", ownerUserID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ?", ownerUserID).
		Order("is_pinned DESC, latest_msg_destruct_time DESC, create_time DESC").
		Offset(offset).Limit(limit).Find(&convs).Error
	return convs, total, err
}

// ListByOwnerSorted 分页查询用户会话，可按 ID 筛选，按置顶和最新消息销毁时间排序。
func (r *conversationRepository) ListByOwnerSorted(ctx context.Context, ownerUserID string, ids []string, offset, limit int) ([]types.Conversation, int64, error) {
	q := r.db.WithContext(ctx).Model(&types.Conversation{}).Where("owner_user_id = ?", ownerUserID)
	if len(ids) > 0 {
		q = q.Where("conversation_id IN ?", ids)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var convs []types.Conversation
	err := q.Order("is_pinned DESC, latest_msg_destruct_time DESC").
		Offset(offset).Limit(limit).Find(&convs).Error
	return convs, total, err
}

// ListByConversationID 根据会话 ID 查询所有所有者的会话记录。
func (r *conversationRepository) ListByConversationID(ctx context.Context, conversationID string) ([]types.Conversation, error) {
	var convs []types.Conversation
	err := r.db.WithContext(ctx).Where("conversation_id = ?", conversationID).Find(&convs).Error
	return convs, err
}

// ListByConversationIDs 根据会话 ID 列表批量查询。
func (r *conversationRepository) ListByConversationIDs(ctx context.Context, ids []string) ([]types.Conversation, error) {
	var convs []types.Conversation
	if len(ids) == 0 {
		return convs, nil
	}
	err := r.db.WithContext(ctx).Where("conversation_id IN ?", ids).Find(&convs).Error
	return convs, err
}

// ListByConversationIDAndOwners 根据会话 ID 和所有者列表查询。
func (r *conversationRepository) ListByConversationIDAndOwners(ctx context.Context, conversationID string, ownerIDs []string) ([]types.Conversation, error) {
	var convs []types.Conversation
	if len(ownerIDs) == 0 {
		return convs, nil
	}
	err := r.db.WithContext(ctx).
		Where("conversation_id = ? AND owner_user_id IN ?", conversationID, ownerIDs).
		Find(&convs).Error
	return convs, err
}

// ListByGroupID 根据群组 ID 查询所有成员的会话记录。
func (r *conversationRepository) ListByGroupID(ctx context.Context, groupID string) ([]types.Conversation, error) {
	var convs []types.Conversation
	err := r.db.WithContext(ctx).Where("group_id = ?", groupID).Find(&convs).Error
	return convs, err
}

// UpdateFields 更新单条会话的可选字段。
func (r *conversationRepository) UpdateFields(ctx context.Context, ownerUserID, conversationID string, patch map[string]any) error {
	if len(patch) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("owner_user_id = ? AND conversation_id = ?", ownerUserID, conversationID).
		Updates(patch).Error
}

// BulkUpdateFields 批量更新多条会话的可选字段。
func (r *conversationRepository) BulkUpdateFields(ctx context.Context, conversationID string, ownerIDs []string, patch map[string]any) error {
	if len(patch) == 0 || len(ownerIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("conversation_id = ? AND owner_user_id IN ?", conversationID, ownerIDs).
		Updates(patch).Error
}

// SetSeq 设置会话的序列号字段（max_seq 或 min_seq）。
func (r *conversationRepository) SetSeq(ctx context.Context, conversationID string, ownerIDs []string, field string, value int64) error {
	if len(ownerIDs) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("conversation_id = ? AND owner_user_id IN ?", conversationID, ownerIDs).
		Update(field, value).Error
}

// ListConversationIDsByOwner 获取用户的所有会话 ID 列表。
func (r *conversationRepository) ListConversationIDsByOwner(ctx context.Context, ownerUserID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("owner_user_id = ?", ownerUserID).
		Pluck("conversation_id", &ids).Error
	return ids, err
}

// ListPinnedIDsByOwner 获取用户已置顶的会话 ID 列表。
func (r *conversationRepository) ListPinnedIDsByOwner(ctx context.Context, ownerUserID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("owner_user_id = ? AND is_pinned = ?", ownerUserID, true).
		Pluck("conversation_id", &ids).Error
	return ids, err
}

// ListNotNotifyIDsByOwner 获取用户静音（不通知）的会话 ID 列表。
func (r *conversationRepository) ListNotNotifyIDsByOwner(ctx context.Context, ownerUserID string) ([]string, error) {
	var ids []string
	err := r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("owner_user_id = ? AND recv_msg_opt = ?", ownerUserID, 2).
		Pluck("conversation_id", &ids).Error
	return ids, err
}

// Delete 删除用户的指定会话记录。
func (r *conversationRepository) Delete(ctx context.Context, ownerUserID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Where("owner_user_id = ? AND conversation_id IN ?", ownerUserID, ids).
		Delete(&types.Conversation{}).Error
}

// UpdateExByOwner 更新用户所有会话的扩展字段。
func (r *conversationRepository) UpdateExByOwner(ctx context.Context, ownerUserID, ex string) error {
	return r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("owner_user_id = ?", ownerUserID).
		Update("ex", ex).Error
}

// ListNeedClearMsg 查询需要清理阅后即焚消息的会话列表。
func (r *conversationRepository) ListNeedClearMsg(ctx context.Context, now int64) ([]types.Conversation, error) {
	var convs []types.Conversation
	err := r.db.WithContext(ctx).
		Where("is_msg_destruct = ? AND latest_msg_destruct_time > ? AND latest_msg_destruct_time <= ?", true, 0, now).
		Find(&convs).Error
	return convs, err
}

// ClearMsgSeqs 清理指定会话的序列号（将 max_seq 设为 min_seq）。
func (r *conversationRepository) ClearMsgSeqs(ctx context.Context, conversationIDs []string) (int64, error) {
	if len(conversationIDs) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&types.Conversation{}).
		Where("conversation_id IN ?", conversationIDs).
		Updates(map[string]any{"max_seq": gorm.Expr("min_seq")})
	return res.RowsAffected, res.Error
}
