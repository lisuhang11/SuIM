package client

import (
	"context"

	pb "SuIM/proto/relationpb"

	"google.golang.org/grpc"
)

// RelationChecker 通过 relation 服务校验黑名单与好友关系。
type RelationChecker struct {
	client pb.RelationServiceClient
}

// NewBlackChecker 创建关系检查器（保留旧名以兼容装配代码）。
func NewBlackChecker(conn grpc.ClientConnInterface) *RelationChecker {
	return &RelationChecker{client: pb.NewRelationServiceClient(conn)}
}

// IsBlockedByPeer 返回 sendID 是否在 recvID 的黑名单中（对齐 OpenIM FriendLocalCache.IsBlack）。
func (c *RelationChecker) IsBlockedByPeer(ctx context.Context, sendID, recvID string) (bool, error) {
	resp, err := c.client.IsBlack(withForwardedMD(ctx), &pb.IsBlackReq{User1: sendID, User2: recvID})
	if err != nil {
		return false, err
	}
	return resp.GetInUser2Blacklist(), nil
}

// IsMutualFriend 返回双方是否互为好友。
func (c *RelationChecker) IsMutualFriend(ctx context.Context, user1, user2 string) (bool, error) {
	resp, err := c.client.IsFriend(withForwardedMD(ctx), &pb.IsFriendReq{User1: user1, User2: user2})
	if err != nil {
		return false, err
	}
	return resp.GetInUser1Friends() && resp.GetInUser2Friends(), nil
}
