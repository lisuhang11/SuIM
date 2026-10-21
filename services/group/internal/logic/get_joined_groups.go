package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/proto/grouppb"
)

// GetJoinedGroups 分页返回用户已加入的群。
func (s *Server) GetJoinedGroups(ctx context.Context, req *grouppb.GetJoinedGroupsReq) (*grouppb.GetJoinedGroupsResp, error) {
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	total, ids, err := s.store.JoinedGroupIDs(ctx, userID, offset, limit)
	if err != nil {
		return nil, err
	}
	groups, err := s.store.FindGroups(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*grouppb.GroupInfo, 0, len(groups))
	for _, group := range groups {
		out = append(out, groupPB(group))
	}
	return &grouppb.GetJoinedGroupsResp{Groups: out, Total: int32(total)}, nil
}
