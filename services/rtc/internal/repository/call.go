// Package repository 提供 rtc_call 持久化的 GORM 实现。
package repository

import (
	"context"
	"errors"

	"rtc/internal/types"
	"rtc/internal/types/interfaces"

	"gorm.io/gorm"
)

type callRepository struct {
	db *gorm.DB
}

// NewCallRepository 创建 GORM 支持的通话仓库。
func NewCallRepository(db *gorm.DB) interfaces.CallRepository {
	return &callRepository{db: db}
}

func (r *callRepository) Create(ctx context.Context, call *types.Call) error {
	return r.db.WithContext(ctx).Create(call).Error
}

func (r *callRepository) GetByID(ctx context.Context, callID string) (*types.Call, error) {
	var call types.Call
	err := r.db.WithContext(ctx).Where("call_id = ?", callID).First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &call, nil
}

func (r *callRepository) Update(ctx context.Context, call *types.Call) error {
	return r.db.WithContext(ctx).Save(call).Error
}

func (r *callRepository) FindActiveByUser(ctx context.Context, userID string) (*types.Call, error) {
	activeStatuses := []string{
		types.CallStatusRinging,
		types.CallStatusAccepted,
		types.CallStatusActive,
	}
	var call types.Call
	err := r.db.WithContext(ctx).
		Where("(caller_id = ? OR callee_id = ?) AND status IN ?", userID, userID, activeStatuses).
		Order("created_at DESC").
		First(&call).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &call, nil
}
