package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	userpb "github.com/lisuhang11/SuIM/proto/user"
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
	// 2. 查不到时返回空列表，页面显示搜索为空。别人只能看到公开名片。
	viewer, err := s.viewerID(ctx, req.GetToken())
	if err != nil {
		return nil, err
	}
	user, err := s.userByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return &userpb.SearchUsersResp{}, nil
		}
		return nil, errorcode.ErrInternal.WithMsg("search users")
	}
	return &userpb.SearchUsersResp{Users: []*userpb.UserInfo{projectUser(user, viewer)}}, nil
}
