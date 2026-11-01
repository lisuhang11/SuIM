package logic

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// GetGroupApplicationList 返回操作者作为群主或管理员能看到的入群申请。
func (s *Server) GetGroupApplicationList(ctx context.Context, req *grouppb.GetGroupApplicationListReq) (*grouppb.GetGroupApplicationListResp, error) {
	userID, err := normalizeUserID(req.GetFromUserId())
	if err != nil {
		return nil, err
	}
	groupIDs, err := normalizeGroupIDs(req.GetGroupIds())
	if err != nil {
		return nil, err
	}
	results, err := normalizeHandleResults(req.GetHandleResults())
	if err != nil {
		return nil, err
	}
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	if len(groupIDs) == 0 {
		groupIDs, err = s.store.ManagedGroupIDs(ctx, userID)
		if err != nil {
			return nil, err
		}
	} else if err := s.requireManagerGroups(ctx, userID, groupIDs); err != nil {
		return nil, err
	}
	total, requests, err := s.store.PageGroupRequests(ctx, groupIDs, results, offset, limit)
	if err != nil {
		return nil, err
	}
	return &grouppb.GetGroupApplicationListResp{Total: int32(total), Requests: requestListPB(requests)}, nil
}
