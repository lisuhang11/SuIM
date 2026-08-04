// Package repository 提供好友关系数据操作的 GORM 实现。
package repository

import (
	"context"

	"relation/internal/types"

	"gorm.io/gorm"
)

// CreateFriend 持久化一条单向好友记录。
func (r *relationRepository) CreateFriend(ctx context.Context, f *types.Friend) error {
	return r.db.WithContext(ctx).Create(f).Error
}

// DeleteFriendPair 删除两个用户之间的双向好友关系，并为双方 bump Delete version。
func (r *relationRepository) DeleteFriendPair(ctx context.Context, userA, userB string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where(
				"(owner_user_id = ? AND friend_user_id = ?) OR (owner_user_id = ? AND friend_user_id = ?)",
				userA, userB, userB, userA,
			).
			Delete(&types.Friend{}).Error; err != nil {
			return err
		}
		if err := incrVersionTx(tx, userA, []string{userB}, types.VersionStateDelete, false); err != nil {
			return err
		}
		return incrVersionTx(tx, userB, []string{userA}, types.VersionStateDelete, false)
	})
}

// FriendExists 判断 (owner, friend) 好友记录是否存在。
func (r *relationRepository) FriendExists(ctx context.Context, ownerUserID, friendUserID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&types.Friend{}).
		Where("owner_user_id = ? AND friend_user_id = ?", ownerUserID, friendUserID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListFriends 分页查询用户的好友列表，按置顶优先、创建时间倒序排列。
func (r *relationRepository) ListFriends(ctx context.Context, ownerUserID string, offset, limit int) ([]*types.Friend, int64, error) {
	var (
		friends []*types.Friend
		total   int64
	)
	base := r.db.WithContext(ctx).Model(&types.Friend{}).Where("owner_user_id = ?", ownerUserID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base.Order("is_pinned DESC, create_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&friends).Error; err != nil {
		return nil, 0, err
	}
	return friends, total, nil
}

// UpdateFriend 按 fields 更新单向好友记录。
func (r *relationRepository) UpdateFriend(ctx context.Context, ownerUserID, friendUserID string, fields map[string]any) error {
	if len(fields) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&types.Friend{}).
		Where("owner_user_id = ? AND friend_user_id = ?", ownerUserID, friendUserID).
		Updates(fields).Error
}
