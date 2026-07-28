// Package client 提供对其他服务的 gRPC 客户端，通过 types/interfaces 中定义的接口封装，保持领域层解耦。
package client

import (
	"context"
	"fmt"

	"group/internal/types/interfaces"

	pb "SuIM/proto/userpb"

	"google.golang.org/grpc"
)

// userGRPCClient 实现 interfaces.UserVerifier，通过 user 服务进行用户校验。
type userGRPCClient struct {
	cli pb.UserServiceClient
}

// NewUserVerifier 创建 UserVerifier 实例，连接通过 grpc.NewClient（非阻塞）建立，
// user 服务短暂不可用时不影响 group 服务启动，校验调用仅在请求时失败。
func NewUserVerifier(conn *grpc.ClientConn) interfaces.UserVerifier {
	return &userGRPCClient{cli: pb.NewUserServiceClient(conn)}
}

// UserExists 通过批量查询判断单个用户是否存在。
func (c *userGRPCClient) UserExists(ctx context.Context, userID string) (bool, error) {
	res, err := c.cli.GetUsersByIDs(ctx, &pb.GetUsersByIDsReq{UserIds: []string{userID}})
	if err != nil {
		return false, err
	}
	_, ok := res.Users[userID]
	return ok, nil
}

// UsersExist 批量检查一组用户 ID 是否存在，返回每个 ID 的存在状态。
func (c *userGRPCClient) UsersExist(ctx context.Context, userIDs []string) (map[string]bool, error) {
	res, err := c.cli.GetUsersByIDs(ctx, &pb.GetUsersByIDsReq{UserIds: userIDs})
	if err != nil {
		return nil, err
	}
	out := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		_, ok := res.Users[id]
		out[id] = ok
	}
	return out, nil
}

// Authenticate 通过 user 服务验证访问令牌，避免 group 服务信任客户端声明的用户 ID。
func (c *userGRPCClient) Authenticate(ctx context.Context, token string) (string, error) {
	res, err := c.cli.ValidateToken(ctx, &pb.ValidateTokenReq{Token: token})
	if err != nil {
		return "", err
	}
	if !res.Valid || res.User == nil || res.User.UserId == "" {
		return "", fmt.Errorf("invalid access token")
	}
	return res.User.UserId, nil
}
