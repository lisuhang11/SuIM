// Package repository 提供好友请求数据操作的 GORM 实现。
package repository

import (
	"context"
	"errors"

	"relation/internal/types"

	"gorm.io/gorm"
)

// CreateFriendRequest 持久化一条新的好友请求。
func (r *relationRepository) CreateFriendRequest(ctx context.Context, req *types.FriendRequest) error {
	return r.db.WithContext(ctx).Create(req).Error
}

// GetFriendRequest 根据复合主键查询好友请求。
func (r *relationRepository) GetFriendRequest(ctx context.Context, fromUserID, toUserID string) (*types.FriendRequest, error) {
	var req types.FriendRequest
	if err := r.db.WithContext(ctx).
		Where("from_user_id = ? AND to_user_id = ?", fromUserID, toUserID).
		First(&req).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFriendRequestNotFound
		}
		return nil, err
	}
	return &req, nil
}

// GetPendingBetween 查询两个用户之间任意方向的待处理好友请求。
func (r *relationRepository) GetPendingBetween(ctx context.Context, userA, userB string) (*types.FriendRequest, error) {
	var req types.FriendRequest
	err := r.db.WithContext(ctx).
		Where(
			"((from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)) AND handle_result = ?",
			userA, userB, userB, userA, types.FriendRequestPending,
		).
		First(&req).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFriendRequestNotFound
		}
		return nil, err
	}
	return &req, nil
}

// UpdateFriendRequestStatus 更新好友请求的处理状态。
func (r *relationRepository) UpdateFriendRequestStatus(ctx context.Context, fromUserID, toUserID string, status types.FriendRequestHandleResult) error {
	return r.db.WithContext(ctx).
		Model(&types.FriendRequest{}).
		Where("from_user_id = ? AND to_user_id = ?", fromUserID, toUserID).
		Updates(map[string]any{"handle_result": status}).Error
}

// ListIncomingRequests 分页查询发给指定用户的好友请求，按 handleResults 筛选状态。
func (r *relationRepository) ListIncomingRequests(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error) {
	return listFriendRequests(ctx, r.db, "to_user_id = ?", handleResults, offset, limit, userID)
}

// ListOutgoingRequests 分页查询指定用户发出的好友请求，按 handleResults 筛选状态。
func (r *relationRepository) ListOutgoingRequests(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error) {
	return listFriendRequests(ctx, r.db, "from_user_id = ?", handleResults, offset, limit, userID)
}

// CountUnhandledRequests 统计发给指定用户的未处理好友请求数量。
func (r *relationRepository) CountUnhandledRequests(ctx context.Context, toUserID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&types.FriendRequest{}).
		Where("to_user_id = ? AND handle_result = ?", toUserID, types.FriendRequestPending).
		Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// listFriendRequests 分页查询好友请求的通用辅助函数。
// cond/handleResults 为筛选条件，handleResults 非空时追加 IN 子句。
func listFriendRequests(ctx context.Context, db *gorm.DB, cond string, handleResults []int32, offset, limit int, args ...any) ([]*types.FriendRequest, int64, error) {
	var (
		reqs  []*types.FriendRequest
		total int64
	)
	base := db.WithContext(ctx).Model(&types.FriendRequest{}).Where(cond, args...)
	if len(handleResults) > 0 {
		base = base.Where("handle_result IN ?", handleResults)
	}
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
	if err := query.Find(&reqs).Error; err != nil {
		return nil, 0, err
	}
	return reqs, total, nil
}
