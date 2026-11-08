package rpc

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// GroupClient reads group membership for call admission.
type GroupClient struct {
	client grouppb.GroupServiceClient
}

func NewGroupClient(client grouppb.GroupServiceClient) *GroupClient {
	return &GroupClient{client: client}
}

// InGroup reports whether userID is currently a member. A missing member is false, not an error.
func (c *GroupClient) InGroup(ctx context.Context, groupID, userID string) (bool, error) {
	if c == nil || c.client == nil {
		return false, nil
	}
	resp, err := c.client.GetGroupMember(ctx, &grouppb.GetGroupMemberReq{GroupId: groupID, UserId: userID})
	if err != nil {
		return false, err
	}
	return resp.GetMember().GetUserId() != "", nil
}
