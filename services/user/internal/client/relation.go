package client

import (
	"context"
	"log/slog"

	pb "SuIM/proto/relationpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// RelationNotifier 通知 relation 服务：用户昵称/头像变更后 bump 好友 version。
type RelationNotifier struct {
	client pb.RelationServiceClient
}

// NewRelationNotifier 创建 relation 通知客户端。
func NewRelationNotifier(conn grpc.ClientConnInterface) *RelationNotifier {
	return &RelationNotifier{client: pb.NewRelationServiceClient(conn)}
}

// NotificationUserInfoUpdate 转发 Bearer metadata 调用 relation。
func (c *RelationNotifier) NotificationUserInfoUpdate(ctx context.Context, userID string) error {
	if c == nil || c.client == nil || userID == "" {
		return nil
	}
	_, err := c.client.NotificationUserInfoUpdate(withForwardedMD(ctx), &pb.NotificationUserInfoUpdateReq{
		UserId: userID,
	})
	if err != nil {
		slog.WarnContext(ctx, "relation NotificationUserInfoUpdate failed", "user_id", userID, "error", err)
	}
	return err
}

func withForwardedMD(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md.Copy())
}
