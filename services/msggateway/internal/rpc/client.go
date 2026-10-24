package rpc

import (
	"context"
	"fmt"

	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/proto/userpb"
)

const pageLimit = 200

// UserClient checks a login token with the user service.
type UserClient struct {
	client userpb.UserServiceClient
}

func NewUserClient(client userpb.UserServiceClient) *UserClient {
	return &UserClient{client: client}
}

func (c *UserClient) Validate(ctx context.Context, token string) (string, int32, error) {
	resp, err := c.client.ValidateToken(ctx, &userpb.ValidateTokenReq{Token: token})
	if err != nil {
		return "", 0, err
	}
	if resp == nil || !resp.GetValid() || resp.GetUser() == nil || resp.GetUser().GetUserId() == "" {
		return "", 0, fmt.Errorf("invalid token")
	}
	return resp.GetUser().GetUserId(), resp.GetPlatformId(), nil
}

// GroupClient reads membership from the group service.
type GroupClient struct {
	client grouppb.GroupServiceClient
}

func NewGroupClient(client grouppb.GroupServiceClient) *GroupClient {
	return &GroupClient{client: client}
}

type JoinedGroup struct {
	ID    string
	Super bool
}

func (c *GroupClient) MemberUserIDs(ctx context.Context, groupID string) ([]string, error) {
	resp, err := c.client.GetGroupMemberUserIDs(ctx, &grouppb.GetGroupMemberUserIDsReq{GroupId: groupID})
	if err != nil {
		return nil, err
	}
	return resp.GetUserIds(), nil
}

func (c *GroupClient) Joined(ctx context.Context, userID string) ([]JoinedGroup, error) {
	groups, err := c.ListJoined(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]JoinedGroup, 0, len(groups))
	for _, group := range groups {
		out = append(out, JoinedGroup{ID: group.GetGroupId(), Super: group.GetGroupType() == 2})
	}
	return out, nil
}

// IncrementalJoined reads one page of the user's joined-group version log.
func (c *GroupClient) IncrementalJoined(ctx context.Context, userID, versionID string, version uint64) (*grouppb.GetIncrementalJoinGroupsResp, error) {
	return c.client.GetIncrementalJoinGroups(ctx, &grouppb.GetIncrementalJoinGroupsReq{
		UserId:    userID,
		VersionId: versionID,
		Version:   version,
	})
}

// ListJoined pages every group the user still belongs to.
func (c *GroupClient) ListJoined(ctx context.Context, userID string) ([]*grouppb.GroupInfo, error) {
	var out []*grouppb.GroupInfo
	for offset := int32(0); ; offset += pageLimit {
		resp, err := c.client.GetJoinedGroups(ctx, &grouppb.GetJoinedGroupsReq{
			UserId: userID,
			Offset: offset,
			Limit:  pageLimit,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, resp.GetGroups()...)
		if int(offset)+len(resp.GetGroups()) >= int(resp.GetTotal()) || len(resp.GetGroups()) == 0 {
			break
		}
	}
	return out, nil
}
