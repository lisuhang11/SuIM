package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/proto/grouppb"
)

// GetGroupMemberUserIDs 返回群内全部成员的用户 ID。
func (s *Server) GetGroupMemberUserIDs(ctx context.Context, req *grouppb.GetGroupMemberUserIDsReq) (*grouppb.GetGroupMemberUserIDsResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
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
