package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// MemberFriendAllowed 判断两人能否互加好友。
// 只要同在一个未解散、且 apply_member_friend 为 0 的群里，就不允许。
func (s *Server) MemberFriendAllowed(ctx context.Context, req *grouppb.MemberFriendAllowedReq) (*grouppb.MemberFriendAllowedResp, error) {
	user1, err := bindActor(ctx, req.GetUserId_1())
	if err != nil {
		return nil, err
	}
	user2, err := normalizeUserID(req.GetUserId_2())
	if err != nil {
		return nil, err
	}
	if user1 == user2 {
		return &grouppb.MemberFriendAllowedResp{Allowed: true}, nil
	}
	forbidden, err := s.store.MemberFriendForbidden(ctx, user1, user2)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check group friend")
	}
	return &grouppb.MemberFriendAllowedResp{Allowed: !forbidden}, nil
}
