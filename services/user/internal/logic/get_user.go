package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) GetUserByID(ctx context.Context, req *userpb.GetUserByIDReq) (*userpb.GetUserByIDResp, error) {
	// 1. 校验用户 ID。
	id, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 查询用户。没有令牌或令牌不是本人时只返回公开名片。
	viewer, err := s.viewerID(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}
	user, err := s.userByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	return &userpb.GetUserByIDResp{User: projectUser(user, viewer)}, nil
}
