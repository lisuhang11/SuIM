// Package repository 提供入群请求数据操作的 GORM 实现。
package repository

import (
	"context"
	"errors"

	"group/internal/types"

	"gorm.io/gorm"
)

// CreateRequest 持久化入群请求。
func (r *groupRepository) CreateRequest(ctx context.Context, req *types.GroupRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

// GetRequest 根据群组和用户 ID 查询入群请求，不存在则返回 ErrRequestNotFound。
func (r *groupRepository) GetRequest(ctx context.Context, groupID, userID string) (*types.GroupRequest, error) {
	var req types.GroupRequest
	if err := r.db.WithContext(ctx).Where("group_id = ? AND user_id = ?", groupID, userID).First(&req).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRequestNotFound
		}
		return nil, err
	}
	return &req, nil
}

// UpdateRequest 写入所有请求字段（包括零值）。
func (r *groupRepository) UpdateRequest(ctx context.Context, req *types.GroupRequest) error {
	return r.db.WithContext(ctx).Model(&types.GroupRequest{}).Where("group_id = ? AND user_id = ?", req.GroupID, req.UserID).Select("*").Updates(req).Error
}

// DeleteRequestsByGroup 删除群组的所有入群请求。
func (r *groupRepository) DeleteRequestsByGroup(ctx context.Context, groupID string) error {
	return r.db.WithContext(ctx).Where("group_id = ?", groupID).Delete(&types.GroupRequest{}).Error
}

// ListPendingByGroup 分页查询群组的待处理入群请求。
func (r *groupRepository) ListPendingByGroup(ctx context.Context, groupID string, offset, limit int) ([]*types.GroupRequest, int64, error) {
	var (
		reqs  []*types.GroupRequest
		total int64
	)
	base := r.db.WithContext(ctx).Model(&types.GroupRequest{}).Where("group_id = ? AND handle_result = ?", groupID, 0)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base.Order("req_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&reqs).Error; err != nil {
		return nil, 0, err
	}
	return reqs, total, nil
}

// ListByUser 分页查询用户的入群申请记录。
func (r *groupRepository) ListByUser(ctx context.Context, userID string, offset, limit int) ([]*types.GroupRequest, int64, error) {
	var (
		reqs  []*types.GroupRequest
		total int64
	)
	base := r.db.WithContext(ctx).Model(&types.GroupRequest{}).Where("user_id = ?", userID)
	if err := base.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query := base.Order("req_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&reqs).Error; err != nil {
		return nil, 0, err
	}
	return reqs, total, nil
}

// CountPendingByGroup 统计群组待处理的入群请求数量。
func (r *groupRepository) CountPendingByGroup(ctx context.Context, groupID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&types.GroupRequest{}).
		Where("group_id = ? AND handle_result = ?", groupID, 0).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
