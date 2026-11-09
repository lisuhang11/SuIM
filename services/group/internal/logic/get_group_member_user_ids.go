package logic

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// 返回群内全部成员的用户 ID。调用者必须已经在群里。
func (s *Server) GetGroupMemberUserIDs(ctx context.Context, req *grouppb.GetGroupMemberUserIDsReq) (*grouppb.GetGroupMemberUserIDsResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	userID, err := bindActor(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	if err := s.requireMember(ctx, groupID, userID); err != nil {
		return nil, err
	}
	if _, err := s.store.TakeGroup(ctx, groupID); err != nil {
		return nil, mapStore(err)
	}
	ids, err := s.store.MemberUserIDs(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return &grouppb.GetGroupMemberUserIDsResp{UserIds: ids}, nil
}
