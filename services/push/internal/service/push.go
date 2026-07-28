// Package service 实现 push 领域的业务逻辑。
package service

import (
	"context"

	pb "SuIM/proto/pushpb"

	apperrors "push/internal/errors"
	"push/internal/logger"
	"push/internal/types"
	"push/internal/types/interfaces"
)

// pushService push 服务实现。
type pushService struct {
	repo interfaces.PushRepository
}

// NewPushService 创建 push 服务实例。
func NewPushService(repo interfaces.PushRepository) interfaces.PushService {
	return &pushService{repo: repo}
}

// PushMsg 向指定用户列表推送消息通知。
func (s *pushService) PushMsg(ctx context.Context, req *pb.PushMsgReq) error {
	if req.ConversationId == "" {
		return apperrors.NewValidationError("conversation_id is required")
	}
	if len(req.UserIds) == 0 {
		return apperrors.NewValidationError("user_ids is required")
	}

	tokens, err := s.repo.GetTokensByUserIDs(ctx, req.UserIds)
	if err != nil {
		return apperrors.NewInternalError("failed to query push tokens").WithDetails(err)
	}

	if len(tokens) == 0 {
		logger.Warn(ctx, "no push tokens found for users", "user_count", len(req.UserIds))
		return nil
	}

	// TODO: 对接真实的推送通道（FCM / APNs / 厂商推送）。
	// 当前实现仅记录日志，作为占位。
	msg := req.MsgData
	title := "New Message"
	if msg != nil {
		if msg.SenderNickname != "" {
			title = msg.SenderNickname
		}
	}
	logger.Info(ctx, "push message dispatched (placeholder)",
		"user_count", len(req.UserIds),
		"token_count", len(tokens),
		"conversation_id", req.ConversationId,
		"title", title,
	)

	return nil
}

// SetUserPushToken 为用户注册或更新指定平台的设备推送令牌。
func (s *pushService) SetUserPushToken(ctx context.Context, userID string, platformID int32, token string) error {
	if userID == "" {
		return apperrors.NewValidationError("user_id is required")
	}
	if token == "" {
		return apperrors.NewValidationError("token is required")
	}
	t := &types.PushToken{
		UserID:     userID,
		PlatformID: int(platformID),
		Token:      token,
	}
	if err := s.repo.UpsertToken(ctx, t); err != nil {
		return apperrors.NewInternalError("failed to store push token").WithDetails(err)
	}
	logger.Info(ctx, "push token stored",
		"user_id", userID,
		"platform_id", platformID,
	)
	return nil
}

// DelUserPushToken 删除用户指定平台的设备推送令牌。
func (s *pushService) DelUserPushToken(ctx context.Context, userID string, platformID int32) error {
	if userID == "" {
		return apperrors.NewValidationError("user_id is required")
	}
	if err := s.repo.DeleteToken(ctx, userID, int(platformID)); err != nil {
		return apperrors.NewInternalError("failed to delete push token").WithDetails(err)
	}
	logger.Info(ctx, "push token deleted",
		"user_id", userID,
		"platform_id", platformID,
	)
	return nil
}
