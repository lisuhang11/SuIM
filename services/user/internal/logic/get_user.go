package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) GetUserByID(ctx context.Context, req *userpb.GetUserByIDReq) (*userpb.GetUserByIDResp, error) {
	// 1. 校验用户 ID。
	id, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 查询用户。
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errorcode.ErrUserNotFound
		}
		return nil, errorcode.ErrInternal.WithMsg("query user")
	}
	return &userpb.GetUserByIDResp{User: toPB(user)}, nil
}

func (s *Server) GetUsersByIDs(ctx context.Context, req *userpb.GetUsersByIDsReq) (*userpb.GetUsersByIDsResp, error) {
	// 1. 校验用户 ID，并去掉重复项。
	seen := make(map[string]struct{}, len(req.GetUserIds()))
	ids := make([]string, 0, len(req.GetUserIds()))
	for _, raw := range req.GetUserIds() {
		id, err := normalizeUserID(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	// 2. 批量查询。不存在的 ID 不出现在结果里。
	users, err := s.store.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query users")
	}
	resp := &userpb.GetUsersByIDsResp{Users: make(map[string]*userpb.UserInfo, len(users))}
	for _, user := range users {
		resp.Users[user.UserID] = toPB(user)
	}
	return resp, nil
}
