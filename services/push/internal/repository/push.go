// Package repository 提供 push 持久化的 GORM 实现。
package repository

import (
	"context"
	"time"

	"push/internal/types"
	"push/internal/types/interfaces"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// pushRepository GORM 实现的推送仓库。
type pushRepository struct {
	db *gorm.DB
}

// NewPushRepository 创建 GORM 支持的推送仓库。
func NewPushRepository(db *gorm.DB) interfaces.PushRepository {
	return &pushRepository{db: db}
}

// GetTokensByUserIDs 批量查询用户的推送令牌。
func (r *pushRepository) GetTokensByUserIDs(ctx context.Context, userIDs []string) ([]types.PushToken, error) {
	if len(userIDs) == 0 {
		return nil, nil
	}
	var tokens []types.PushToken
	err := r.db.WithContext(ctx).Where("user_id IN ?", userIDs).Find(&tokens).Error
	return tokens, err
}

// UpsertToken 创建或更新用户的推送令牌（按 user_id + platform_id 唯一）。
func (r *pushRepository) UpsertToken(ctx context.Context, token *types.PushToken) error {
	now := time.Now().UnixMilli()
	token.CreatedAt = now
	token.UpdatedAt = now
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "platform_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"token", "updated_at"}),
	}).Create(token).Error
}

// DeleteToken 删除用户指定平台的推送令牌。
func (r *pushRepository) DeleteToken(ctx context.Context, userID string, platformID int) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND platform_id = ?", userID, platformID).
		Delete(&types.PushToken{}).Error
}
