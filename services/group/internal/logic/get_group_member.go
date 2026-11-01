package logic

import (
	"context"
	"errors"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/store"
)

// GetGroupMember 返回群和一名成员。成员不在群里时 member 为空。
func (s *Server) GetGroupMember(ctx context.Context, req *grouppb.GetGroupMemberReq) (*grouppb.GetGroupMemberResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	group, err := s.store.TakeGroup(ctx, groupID)
	if err != nil {
		return nil, mapStore(err)
	}
	resp := &grouppb.GetGroupMemberResp{Group: groupPB(group)}
	member, err := s.store.TakeMember(ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, store.ErrNotMember) {
			return resp, nil
		}
		return nil, err
	}
	resp.Member = memberPB(member)
	return resp, nil
}
