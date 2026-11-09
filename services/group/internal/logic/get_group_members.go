package logic

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// 分页返回群成员资料。调用者必须已经在群里。
func (s *Server) GetGroupMembers(ctx context.Context, req *grouppb.GetGroupMembersReq) (*grouppb.GetGroupMembersResp, error) {
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
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	if _, err := s.store.TakeGroup(ctx, groupID); err != nil {
		return nil, mapStore(err)
	}
	total, members, err := s.store.PageMembers(ctx, groupID, offset, limit)
	if err != nil {
		return nil, err
	}
	if err := s.fillMembers(ctx, members); err != nil {
		return nil, err
	}
	out := make([]*grouppb.GroupMemberFullInfo, 0, len(members))
	for _, member := range members {
		out = append(out, memberPB(member))
	}
	return &grouppb.GetGroupMembersResp{Members: out, Total: int32(total)}, nil
}
