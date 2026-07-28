// Package interfaces 定义 push 服务的业务层契约和仓库契约。
package interfaces

import (
	"context"

	pb "SuIM/proto/pushpb"

	"push/internal/types"
)

// PushService 业务层契约，由 service 层实现，由 gRPC handler 消费。
type PushService interface {
	// PushMsg 向指定用户列表推送消息通知。
	PushMsg(ctx context.Context, req *pb.PushMsgReq) error

	// SetUserPushToken 为用户注册或更新指定平台的设备推送令牌。
	SetUserPushToken(ctx context.Context, userID string, platformID int32, token string) error

	// DelUserPushToken 删除用户指定平台的设备推送令牌。
	DelUserPushToken(ctx context.Context, userID string, platformID int32) error
}

// PushRepository 持久层契约。
type PushRepository interface {
	// GetTokensByUserIDs 批量查询用户的推送令牌。
	GetTokensByUserIDs(ctx context.Context, userIDs []string) ([]types.PushToken, error)

	// UpsertToken 创建或更新用户的推送令牌。
	UpsertToken(ctx context.Context, token *types.PushToken) error

	// DeleteToken 删除用户指定平台的推送令牌。
	DeleteToken(ctx context.Context, userID string, platformID int) error
}
