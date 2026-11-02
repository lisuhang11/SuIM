package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// GetFullGroupMemberUserIDs 返回成员 ID 和版本。调用者必须已经在群里。
func (s *Server) GetFullGroupMemberUserIDs(ctx context.Context, req *grouppb.GetFullGroupMemberUserIDsReq) (*grouppb.GetFullGroupMemberUserIDsResp, error) {
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
	head, err := s.store.MemberVersion(ctx, groupID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query member version")
	}
	ids, err := s.store.MemberUserIDs(ctx, groupID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list member ids")
	}
	sum := idhash.Hash(ids)
	resp := &grouppb.GetFullGroupMemberUserIDsResp{
		Version:   head.Version,
		VersionId: head.ID,
		Equal:     req.GetIdHash() == sum,
		UserIds:   ids,
	}
	if resp.Equal {
		resp.UserIds = nil
	}
	return resp, nil
}
