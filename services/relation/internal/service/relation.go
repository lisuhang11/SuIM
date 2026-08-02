// Package service 实现好友关系业务逻辑，包括好友请求、好友管理和拉黑等核心流程。
package service

import (
	"context"
	"errors"
	"time"
	"unicode/utf8"

	apperrors "relation/internal/errors"
	"relation/internal/logger"
	"relation/internal/notification"
	"relation/internal/repository"
	"relation/internal/types"
	"relation/internal/types/interfaces"
)

// 哨兵错误从仓库层别名导出，使 errors.Is 可跨层使用，避免重复定义消息字符串。
var (
	ErrFriendRequestNotFound = repository.ErrFriendRequestNotFound
	ErrFriendNotFound        = repository.ErrFriendNotFound
	ErrBlackNotFound         = repository.ErrBlackNotFound
)

// relationService 实现 RelationService 接口，封装关系业务逻辑。
type relationService struct {
	repo     interfaces.RelationRepository
	notifier *notification.FriendNotificationSender
}

// NewRelationService 创建关系服务实例。
func NewRelationService(repo interfaces.RelationRepository, notifier *notification.FriendNotificationSender) interfaces.RelationService {
	return &relationService{repo: repo, notifier: notifier}
}

// SendFriendRequest 发送好友请求。
func (s *relationService) SendFriendRequest(ctx context.Context, fromUserID, toUserID, msg string) error {
	logger.Info(ctx, "[relation] send friend request", "from", fromUserID, "to", toUserID)

	if fromUserID == toUserID {
		return apperrors.NewCannotFriendSelfError()
	}

	// 如果双方已是好友，拒绝请求。
	exists, err := s.repo.FriendExists(ctx, fromUserID, toUserID)
	if err != nil {
		return apperrors.NewInternalError("check friend status failed").WithDetails(err)
	}
	if exists {
		return apperrors.NewAlreadyFriendsError()
	}

	// 如果目标用户已将发送者拉黑，拒绝请求。
	if _, err := s.repo.FindBlock(ctx, toUserID, fromUserID); err == nil {
		return apperrors.NewValidationError("you are blocked by this user")
	}

	// 如果任意方向已存在待处理的请求，拒绝重复发送。
	if _, err := s.repo.GetPendingBetween(ctx, fromUserID, toUserID); err == nil {
		return apperrors.NewAlreadyRequestedError()
	}

	// 同方向已有历史申请（已同意/已拒绝）：覆盖为 pending，避免主键冲突。
	if _, err := s.repo.GetFriendRequest(ctx, fromUserID, toUserID); err == nil {
		if err := s.repo.ResetFriendRequestPending(ctx, fromUserID, toUserID, msg); err != nil {
			logger.Error(ctx, "[relation] reset friend request failed", "error", err)
			return apperrors.NewInternalError("failed to send friend request").WithDetails(err)
		}
		if s.notifier != nil {
			s.notifier.FriendApplicationNotification(ctx, fromUserID, toUserID, msg)
		}
		logger.Info(ctx, "[relation] friend request re-sent")
		return nil
	} else if !errors.Is(err, ErrFriendRequestNotFound) {
		return apperrors.NewInternalError("check friend request failed").WithDetails(err)
	}

	now := time.Now()
	req := &types.FriendRequest{
		FromUserID:   fromUserID,
		ToUserID:     toUserID,
		HandleResult: types.FriendRequestPending,
		ReqMsg:       msg,
		CreateTime:   now,
	}
	if err := s.repo.CreateFriendRequest(ctx, req); err != nil {
		logger.Error(ctx, "[relation] create friend request failed", "error", err)
		return apperrors.NewInternalError("failed to send friend request").WithDetails(err)
	}

	// 异步通知接收方（不阻塞主流程）。
	if s.notifier != nil {
		s.notifier.FriendApplicationNotification(ctx, fromUserID, toUserID, msg)
	}

	logger.Info(ctx, "[relation] friend request sent")
	return nil
}

// RespondFriendApply 响应好友请求（接受或拒绝），创建/不创建双向好友记录。
func (s *relationService) RespondFriendApply(ctx context.Context, fromUserID, toUserID, userID string, handleResult types.FriendRequestHandleResult, handleMsg string) error {
	action := "accept"
	if handleResult == types.FriendRequestRejected {
		action = "reject"
	}
	logger.Info(ctx, "[relation] respond friend apply", "from", fromUserID, "to", toUserID, "action", action, "user_id", userID)

	if toUserID != userID {
		return apperrors.NewNotAuthorizedError()
	}
	if handleResult != types.FriendRequestAccepted && handleResult != types.FriendRequestRejected {
		return apperrors.NewValidationError("invalid friend request handle result")
	}

	req, err := s.repo.GetFriendRequest(ctx, fromUserID, toUserID)
	if err != nil {
		return apperrors.NewFriendRequestNotFoundError().WithDetails(err)
	}
	if req.HandleResult != types.FriendRequestPending {
		return apperrors.NewRequestAlreadyProcessedError()
	}

	if handleResult == types.FriendRequestAccepted {
		exists, err := s.repo.FriendExists(ctx, userID, fromUserID)
		if err != nil {
			return apperrors.NewInternalError("check friend status failed").WithDetails(err)
		}
		if exists {
			return apperrors.NewAlreadyFriendsError()
		}
		if err := s.repo.AcceptFriendRequest(ctx, fromUserID, toUserID, userID, handleMsg); err != nil {
			logger.Error(ctx, "[relation] accept friend request failed", "error", err)
			return apperrors.NewInternalError("failed to create friend relation").WithDetails(err)
		}
		if s.notifier != nil {
			s.notifier.FriendApplicationAcceptedNotification(ctx, fromUserID, toUserID)
		}
	} else {
		if err := s.repo.UpdateFriendRequestStatus(ctx, fromUserID, toUserID, userID, handleResult, handleMsg); err != nil {
			logger.Error(ctx, "[relation] update request status failed", "error", err)
			return apperrors.NewInternalError("failed to respond to request").WithDetails(err)
		}
		if s.notifier != nil {
			s.notifier.FriendApplicationRejectedNotification(ctx, fromUserID, toUserID, handleMsg)
		}
	}

	logger.Info(ctx, "[relation] friend apply responded", "action", action, "from", fromUserID, "to", toUserID)
	return nil
}

// DeleteFriend 删除双向好友关系。
func (s *relationService) DeleteFriend(ctx context.Context, userID, friendID string) error {
	logger.Info(ctx, "[relation] delete friend", "user_id", userID, "friend_id", friendID)

	if userID == friendID {
		return apperrors.NewValidationError("cannot delete yourself as friend")
	}

	if err := s.repo.DeleteFriendPair(ctx, userID, friendID); err != nil {
		logger.Error(ctx, "[relation] delete friend failed", "error", err)
		return apperrors.NewInternalError("failed to delete friend").WithDetails(err)
	}
	if s.notifier != nil {
		s.notifier.FriendDeletedNotification(ctx, userID, friendID)
		s.notifier.FriendDeletedNotification(ctx, friendID, userID)
	}

	logger.Info(ctx, "[relation] friend deleted")
	return nil
}

// GetFriends 获取用户的好友列表（分页，含备注/置顶）。
func (s *relationService) GetFriends(ctx context.Context, userID string, offset, limit int) ([]*types.Friend, int, error) {
	friends, total, err := s.repo.ListFriends(ctx, userID, offset, limit)
	if err != nil {
		logger.Error(ctx, "[relation] get friends failed", "error", err)
		return nil, 0, apperrors.NewInternalError("failed to get friends").WithDetails(err)
	}
	return friends, int(total), nil
}

const maxFriendRemarkRunes = 64

// UpdateFriend 局部更新好友备注 / 置顶。
func (s *relationService) UpdateFriend(ctx context.Context, ownerUserID, friendUserID string, remark *string, isPinned *bool) error {
	if ownerUserID == "" || friendUserID == "" {
		return apperrors.NewValidationError("owner_user_id and friend_user_id required")
	}
	if remark == nil && isPinned == nil {
		return apperrors.NewValidationError("at least one of remark or is_pinned required")
	}
	if remark != nil && utf8.RuneCountInString(*remark) > maxFriendRemarkRunes {
		return apperrors.NewValidationError("remark too long")
	}

	ok, err := s.repo.FriendExists(ctx, ownerUserID, friendUserID)
	if err != nil {
		return apperrors.NewInternalError("check friend failed").WithDetails(err)
	}
	if !ok {
		return apperrors.NewRelationNotFoundError()
	}

	fields := make(map[string]any, 2)
	if remark != nil {
		fields["remark"] = *remark
	}
	if isPinned != nil {
		fields["is_pinned"] = *isPinned
	}
	if err := s.repo.UpdateFriend(ctx, ownerUserID, friendUserID, fields); err != nil {
		logger.Error(ctx, "[relation] update friend failed", "error", err)
		return apperrors.NewInternalError("failed to update friend").WithDetails(err)
	}
	isSort := isPinned != nil
	if err := s.repo.IncrVersion(ctx, ownerUserID, []string{friendUserID}, types.VersionStateUpdate, isSort); err != nil {
		logger.Error(ctx, "[relation] incr version after update friend failed", "error", err)
		return apperrors.NewInternalError("failed to update friend version").WithDetails(err)
	}
	if s.notifier != nil {
		s.notifier.FriendInfoChangedNotification(ctx, ownerUserID, friendUserID)
	}
	logger.Info(ctx, "[relation] friend updated", "owner", ownerUserID, "friend", friendUserID)
	return nil
}

// GetIncomingApplyTo 分页获取收到的好友请求，handleResults 为空则不过滤状态。
func (s *relationService) GetIncomingApplyTo(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error) {
	reqs, total, err := s.repo.ListIncomingRequests(ctx, userID, handleResults, offset, limit)
	if err != nil {
		logger.Error(ctx, "[relation] get incoming apply to failed", "error", err)
		return nil, 0, apperrors.NewInternalError("failed to get incoming requests").WithDetails(err)
	}
	return reqs, total, nil
}

// GetOutgoingApplyFrom 分页获取发出的好友请求，handleResults 为空则不过滤状态。
func (s *relationService) GetOutgoingApplyFrom(ctx context.Context, userID string, handleResults []int32, offset, limit int) ([]*types.FriendRequest, int64, error) {
	reqs, total, err := s.repo.ListOutgoingRequests(ctx, userID, handleResults, offset, limit)
	if err != nil {
		logger.Error(ctx, "[relation] get outgoing apply from failed", "error", err)
		return nil, 0, apperrors.NewInternalError("failed to get outgoing requests").WithDetails(err)
	}
	return reqs, total, nil
}

// GetUnhandledApplyCount 获取发给指定用户的未处理好友请求数量。
func (s *relationService) GetUnhandledApplyCount(ctx context.Context, userID string) (int64, error) {
	count, err := s.repo.CountUnhandledRequests(ctx, userID)
	if err != nil {
		logger.Error(ctx, "[relation] count unhandled requests failed", "error", err)
		return 0, apperrors.NewInternalError("failed to count unhandled requests").WithDetails(err)
	}
	return count, nil
}

// BlockUser 拉黑指定用户（保留好友关系，不从好友列表移除）。
func (s *relationService) BlockUser(ctx context.Context, userID, blockedUserID string) error {
	logger.Info(ctx, "[relation] block user", "user_id", userID, "blocked_user_id", blockedUserID)

	if userID == blockedUserID {
		return apperrors.NewValidationError("cannot block yourself")
	}

	// 幂等检查：已拉黑则直接返回错误。
	if _, err := s.repo.FindBlock(ctx, userID, blockedUserID); err == nil {
		return apperrors.NewAlreadyBlockedError()
	}

	now := time.Now()
	b := &types.Black{
		OwnerUserID:    userID,
		BlockUserID:    blockedUserID,
		CreateTime:     now,
		AddSource:      0,
		OperatorUserID: userID,
	}
	if err := s.repo.CreateBlock(ctx, b); err != nil {
		logger.Error(ctx, "[relation] block user failed", "error", err)
		return apperrors.NewInternalError("failed to block user").WithDetails(err)
	}

	logger.Info(ctx, "[relation] user blocked")
	return nil
}

// UnblockUser 取消拉黑指定用户。
func (s *relationService) UnblockUser(ctx context.Context, userID, blockedUserID string) error {
	logger.Info(ctx, "[relation] unblock user", "user_id", userID, "blocked_user_id", blockedUserID)

	if _, err := s.repo.FindBlock(ctx, userID, blockedUserID); err != nil {
		return apperrors.NewNotBlockedError().WithDetails(err)
	}

	if err := s.repo.DeleteBlock(ctx, userID, blockedUserID); err != nil {
		logger.Error(ctx, "[relation] unblock user failed", "error", err)
		return apperrors.NewInternalError("failed to unblock user").WithDetails(err)
	}

	logger.Info(ctx, "[relation] user unblocked")
	return nil
}

// GetBlockedUsers 获取已拉黑列表（分页，含关系字段）。
func (s *relationService) GetBlockedUsers(ctx context.Context, userID string, offset, limit int) ([]*types.Black, int, error) {
	blocks, total, err := s.repo.ListBlocks(ctx, userID, offset, limit)
	if err != nil {
		logger.Error(ctx, "[relation] get blocked users failed", "error", err)
		return nil, 0, apperrors.NewInternalError("failed to get blocked users").WithDetails(err)
	}
	return blocks, int(total), nil
}

// IsFriend 返回 user1 与 user2 之间的双向好友关系详情。
// 每个方向独立查询，即使出现单向好友（数据不一致）也能准确报告。
func (s *relationService) IsFriend(ctx context.Context, user1, user2 string) (inUser1Friends, inUser2Friends bool, err error) {
	logger.Info(ctx, "[relation] is friend", "user1", user1, "user2", user2)

	var e1, e2 error
	var b1, b2 bool
	// in_user1_friends：user2 是否在 user1 的好友列表中。
	b1, e1 = s.repo.FriendExists(ctx, user1, user2)
	// in_user2_friends：user1 是否在 user2 的好友列表中。
	b2, e2 = s.repo.FriendExists(ctx, user2, user1)

	if e1 != nil {
		return false, false, apperrors.NewInternalError("check friend status failed").WithDetails(e1)
	}
	if e2 != nil {
		return false, false, apperrors.NewInternalError("check friend status failed").WithDetails(e2)
	}
	return b1, b2, nil
}

// IsBlack 返回 user1 与 user2 之间的双向拉黑关系详情。
func (s *relationService) IsBlack(ctx context.Context, user1, user2 string) (inUser1Blacklist, inUser2Blacklist bool, err error) {
	logger.Info(ctx, "[relation] is black", "user1", user1, "user2", user2)

	var e1, e2 error
	var b1, b2 bool
	// in_user1_blacklist：user1 是否拉黑了 user2。
	b1, e1 = s.repo.BlockExists(ctx, user1, user2)
	// in_user2_blacklist：user2 是否拉黑了 user1。
	b2, e2 = s.repo.BlockExists(ctx, user2, user1)

	if e1 != nil {
		return false, false, apperrors.NewInternalError("check block status failed").WithDetails(e1)
	}
	if e2 != nil {
		return false, false, apperrors.NewInternalError("check block status failed").WithDetails(e2)
	}
	return b1, b2, nil
}
