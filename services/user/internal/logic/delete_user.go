package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) DeleteUser(ctx context.Context, req *userpb.DeleteUserReq) (*userpb.DeleteUserResp, error) {
	// 1. 校验用户 ID。
	id, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 删除用户及其全部 token。
	revoked, err := s.store.DeleteUser(ctx, id)
	s.invalidate(ctx, revoked)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("delete user")
	}
	// 3. 跨服务清理尚未接入。
	//    relation rpc: 删除该用户的好友关系。
	//    group rpc: 将该用户移出所有群。
	//    kafka: 通知 msggateway 断开该用户全部在线连接。
	return &userpb.DeleteUserResp{}, nil
}
