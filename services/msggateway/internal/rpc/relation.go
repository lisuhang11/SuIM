package rpc

import (
	"context"

	"github.com/lisuhang11/SuIM/proto/relationpb"
)

// RelationClient reads friendship and blocks from the relation service.
type RelationClient struct {
	client relationpb.RelationServiceClient
}

func NewRelationClient(client relationpb.RelationServiceClient) *RelationClient {
	return &RelationClient{client: client}
}

func (c *RelationClient) IsFriend(ctx context.Context, user1, user2 string) (bool, bool, error) {
	resp, err := c.client.IsFriend(ctx, &relationpb.IsFriendReq{User1: user1, User2: user2})
	if err != nil {
		return false, false, err
	}
	return resp.GetInUser1Friends(), resp.GetInUser2Friends(), nil
}

func (c *RelationClient) IsBlack(ctx context.Context, user1, user2 string) (bool, bool, error) {
	resp, err := c.client.IsBlack(ctx, &relationpb.IsBlackReq{User1: user1, User2: user2})
	if err != nil {
		return false, false, err
	}
	return resp.GetInUser1Blacklist(), resp.GetInUser2Blacklist(), nil
}
