// Package repository 提供好友请求数据操作的 GORM 实现。
package repository

import (
	"context"
	"errors"
	"time"

	"relation/internal/types"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// ResetFriendRequestPending 将历史申请（已同意/已拒绝）覆盖为待处理，并刷新留言与时间。
func (r *relationRepository) ResetFriendRequestPending(ctx context.Context, fromUserID, toUserID, reqMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&types.FriendRequest{}).
		Where("from_user_id = ? AND to_user_id = ?", fromUserID, toUserID).
		Updates(map[string]any{
			"handle_result":   types.FriendRequestPending,
			"req_msg":         reqMsg,
			"create_time":     now,
			"handler_user_id": "",
			"handle_msg":      "",
			"handle_time":     nil,
		}).Error
}

// UpdateFriendRequestStatus 更新好友请求的处理状态。
func (r *relationRepository) UpdateFriendRequestStatus(ctx context.Context, fromUserID, toUserID, handlerUserID string, status types.FriendRequestHandleResult, handleMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&types.FriendRequest{}).
		Where("from_user_id = ? AND to_user_id = ?", fromUserID, toUserID).
		Updates(map[string]any{
			"handle_result":   status,
			"handler_user_id": handlerUserID,
			"handle_msg":      handleMsg,
			"handle_time":     now,
		}).Error
}

// AcceptFriendRequest 接受好友请求，并在同一事务中写处理状态和双向好友关系。
func (r *relationRepository) AcceptFriendRequest(ctx context.Context, fromUserID, toUserID, handlerUserID, handleMsg string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&types.FriendRequest{}).
			Where("from_user_id = ? AND to_user_id = ?", fromUserID, toUserID).
			Updates(map[string]any{
				"handle_result":   types.FriendRequestAccepted,
				"handler_user_id": handlerUserID,
				"handle_msg":      handleMsg,
				"handle_time":     now,
			}).Error; err != nil {
			return err
		}

		friends := []*types.Friend{
			{OwnerUserID: fromUserID, FriendUserID: toUserID, CreateTime: now, AddSource: 0, OperatorUserID: handlerUserID},
			{OwnerUserID: toUserID, FriendUserID: fromUserID, CreateTime: now, AddSource: 0, OperatorUserID: handlerUserID},
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&friends).Error; err != nil {
			return err
		}
		// 双方好友列表各记一条 Insert changelog。
		if err := incrVersionTx(tx, fromUserID, []string{toUserID}, types.VersionStateInsert, false); err != nil {
			return err
		}
		return incrVersionTx(tx, toUserID, []string{fromUserID}, types.VersionStateInsert, false)
	})
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
