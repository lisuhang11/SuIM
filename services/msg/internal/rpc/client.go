package rpc

import (
	"context"

	"github.com/lisuhang11/SuIM/proto/grouppb"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/msg/internal/logic"
)

const pageLimit = 200

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

// GroupClient reads joined groups so GetMaxSeq can include them before the first message.
type GroupClient struct {
	client grouppb.GroupServiceClient
}

func NewGroupClient(client grouppb.GroupServiceClient) *GroupClient {
	return &GroupClient{client: client}
}

func (c *GroupClient) Joined(ctx context.Context, userID string) ([]logic.JoinedGroup, error) {
	var out []logic.JoinedGroup
	for offset := int32(0); ; offset += pageLimit {
		resp, err := c.client.GetJoinedGroups(ctx, &grouppb.GetJoinedGroupsReq{
			UserId: userID,
			Offset: offset,
			Limit:  pageLimit,
		})
		if err != nil {
			return nil, err
		}
		for _, group := range resp.GetGroups() {
			if group.GetGroupId() == "" {
				continue
			}
			out = append(out, logic.JoinedGroup{ID: group.GetGroupId(), Super: group.GetGroupType() == 2})
		}
		if int(offset)+len(resp.GetGroups()) >= int(resp.GetTotal()) || len(resp.GetGroups()) == 0 {
			break
		}
	}
	return out, nil
}
