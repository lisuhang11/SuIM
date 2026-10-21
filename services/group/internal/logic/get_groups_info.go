package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/grouppb"
)

// GetGroupsInfo 按群 ID 取群资料。一个或多个 ID 走同一条逻辑，找不到的 ID 略过。
func (s *Server) GetGroupsInfo(ctx context.Context, req *grouppb.GetGroupsInfoReq) (*grouppb.GetGroupsInfoResp, error) {
	if len(req.GetGroupIds()) == 0 {
		return nil, errorcode.ErrGroupInvalidArgs.WithMsg("group ids are empty")
	}
	ids := make([]string, 0, len(req.GetGroupIds()))
	seen := map[string]struct{}{}
	for _, raw := range req.GetGroupIds() {
		id, err := normalizeGroupID(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	groups, err := s.store.FindGroups(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*grouppb.GroupInfo, 0, len(groups))
	for _, group := range groups {
		out = append(out, groupPB(group))
	}
	return &grouppb.GetGroupsInfoResp{Groups: out}, nil
}
