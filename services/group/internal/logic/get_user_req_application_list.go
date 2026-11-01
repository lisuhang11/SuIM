package logic

import (
	"context"

	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// GetUserReqApplicationList 返回用户自己发出的入群申请。
func (s *Server) GetUserReqApplicationList(ctx context.Context, req *grouppb.GetUserReqApplicationListReq) (*grouppb.GetUserReqApplicationListResp, error) {
	userID, err := normalizeUserID(req.GetUserId())
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
	if _, err := s.requireUsers(ctx, []string{userID}); err != nil {
		return nil, err
	}
	total, requests, err := s.store.PageUserRequests(ctx, userID, groupIDs, results, offset, limit)
	if err != nil {
		return nil, err
	}
	return &grouppb.GetUserReqApplicationListResp{Total: int32(total), Requests: requestListPB(requests)}, nil
}
