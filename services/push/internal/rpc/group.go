package rpc

import (
	"context"
	"time"

	"google.golang.org/grpc"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// GroupClient 通过服务发现的连接读取群成员。
type GroupClient struct {
	conn grpc.ClientConnInterface
}

func NewGroupClient(conn grpc.ClientConnInterface) *GroupClient {
	return &GroupClient{conn: conn}
}

// UserIDs 返回群内成员 userID，userID 是发起这次调用的操作者。
func (c *GroupClient) UserIDs(ctx context.Context, groupID, userID string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := grouppb.NewGroupServiceClient(c.conn).GetGroupMemberUserIDs(
		rpcctx.WithUser(ctx, userID),
		&grouppb.GetGroupMemberUserIDsReq{GroupId: groupID, UserId: userID},
	)
	if err != nil {
		return nil, err
	}
	return resp.GetUserIds(), nil
}
