package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
)

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
	// 2. 批量查询。不存在的 ID 不出现在结果里。只有调用者自己的那一行是完整资料。
	viewer, err := s.viewerID(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}
	users, err := s.store.GetUsersByIDs(ctx, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query users")
	}
	resp := &userpb.GetUsersByIDsResp{Users: make(map[string]*userpb.UserInfo, len(users))}
	for _, user := range users {
		resp.Users[user.UserID] = projectUser(user, viewer)
	}
	return resp, nil
}
