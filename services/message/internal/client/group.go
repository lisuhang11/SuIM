package client

import (
	"context"
	"strings"

	pb "SuIM/proto/grouppb"

	"google.golang.org/grpc"
)

// GroupMemberResolver 通过 group.GetGroupMemberUserIDs 解析群成员（对齐 OpenIM push 侧展开）。
type GroupMemberResolver struct {
	client pb.GroupServiceClient
}

// NewGroupMemberResolver 创建群成员解析器。
func NewGroupMemberResolver(conn grpc.ClientConnInterface) *GroupMemberResolver {
	return &GroupMemberResolver{client: pb.NewGroupServiceClient(conn)}
}

// GetGroupMemberUserIDs 返回群全部成员 userID。
func (c *GroupMemberResolver) GetGroupMemberUserIDs(ctx context.Context, groupID string) ([]string, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, nil
	}
	resp, err := c.client.GetGroupMemberUserIDs(withForwardedMD(ctx), &pb.GetGroupMemberUserIDsReq{GroupId: groupID})
	if err != nil {
		return nil, err
	}
	return resp.GetUserIds(), nil
}
