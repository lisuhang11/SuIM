// Package repository 提供拉黑相关数据操作的 GORM 实现。
package repository

import (
	"context"
	"errors"

	"relation/internal/types"

	"gorm.io/gorm"
)

// CreateBlock 持久化一条单向拉黑记录。
func (r *relationRepository) CreateBlock(ctx context.Context, b *types.Black) error {
	return r.db.WithContext(ctx).Create(b).Error
}

// DeleteBlock 删除指定用户对目标用户的拉黑记录。
func (r *relationRepository) DeleteBlock(ctx context.Context, ownerUserID, blockUserID string) error {
	return r.db.WithContext(ctx).
		Where("owner_user_id = ? AND block_user_id = ?", ownerUserID, blockUserID).
		Delete(&types.Black{}).Error
}

// BlockExists 判断 (owner, blocked) 拉黑记录是否存在。
func (r *relationRepository) BlockExists(ctx context.Context, ownerUserID, blockUserID string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&types.Black{}).
		Where("owner_user_id = ? AND block_user_id = ?", ownerUserID, blockUserID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListBlocks 分页查询用户的拉黑列表，返回记录和总数。
func (r *relationRepository) ListBlocks(ctx context.Context, ownerUserID string, offset, limit int) ([]*types.Black, int64, error) {
	var (
		blocks []*types.Black
		total  int64
	)
	base := r.db.WithContext(ctx).Model(&types.Black{}).Where("owner_user_id = ?", ownerUserID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base.Order("create_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&blocks).Error; err != nil {
		return nil, 0, err
	}
	return blocks, total, nil
}

// FindBlock 查询指定用户对目标用户的拉黑记录，不存在则返回 ErrBlackNotFound。
func (r *relationRepository) FindBlock(ctx context.Context, ownerUserID, targetUserID string) (*types.Black, error) {
	var b types.Black
	if err := r.db.WithContext(ctx).
		Where("owner_user_id = ? AND block_user_id = ?", ownerUserID, targetUserID).
		First(&b).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBlackNotFound
		}
		return nil, err
	}
	return &b, nil
}
