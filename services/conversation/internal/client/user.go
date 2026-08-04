package client

import (
	"context"
	"fmt"

	pb "SuIM/proto/userpb"

	"google.golang.org/grpc"
)

type UserAuthenticator struct {
	client pb.UserServiceClient
}

func NewUserAuthenticator(conn grpc.ClientConnInterface) *UserAuthenticator {
	return &UserAuthenticator{client: pb.NewUserServiceClient(conn)}
}

func (c *UserAuthenticator) Authenticate(ctx context.Context, token string) (string, error) {
	resp, err := c.client.ValidateToken(ctx, &pb.ValidateTokenReq{Token: token})
	if err != nil {
		return "", err
	}
	if !resp.Valid || resp.User == nil || resp.User.UserId == "" {
		return "", fmt.Errorf("invalid access token")
	}
	return resp.User.UserId, nil
}
