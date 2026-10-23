package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
)

func (s *Server) SearchUsers(ctx context.Context, req *userpb.SearchUsersReq) (*userpb.SearchUsersResp, error) {
	// 1. 只按用户 ID 精确查找。邮箱、账号、手机号不参与匹配。
	keyword := strings.TrimSpace(req.GetKeyword())
	if keyword == "" {
		return nil, errorcode.ErrUserInvalidProfile.WithMsg("keyword is invalid")
	}
	id, err := normalizeUserID(keyword)
	if err != nil {
		return &userpb.SearchUsersResp{}, nil
	}
	// 2. 查不到时返回空列表，页面显示搜索为空。
	user, err := s.store.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &userpb.SearchUsersResp{}, nil
		}
		return nil, errorcode.ErrInternal.WithMsg("search users")
	}
	return &userpb.SearchUsersResp{Users: []*userpb.UserInfo{toPB(user)}}, nil
}
