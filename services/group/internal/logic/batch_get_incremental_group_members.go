package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

func (s *Server) BatchGetIncrementalGroupMembers(ctx context.Context, req *grouppb.BatchGetIncrementalGroupMembersReq) (*grouppb.BatchGetIncrementalGroupMembersResp, error) {
	// 1. 一批最多 100 个群，重复的群 ID 只算一次。
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	if len(req.GetGroups()) > batchMemberLimit {
		return nil, errorcode.ErrGroupInvalidArgs.WithMsg("too many groups")
	}
	out := make(map[string]*grouppb.GetIncrementalGroupMembersResp, len(req.GetGroups()))
	for _, item := range req.GetGroups() {
		if item.GetUserId() != "" && item.GetUserId() != userID {
			return nil, errorcode.ErrGroupInvalidArgs.WithMsg("user_id does not match")
		}
		groupID, err := normalizeGroupID(item.GetGroupId())
		if err != nil {
			return nil, err
		}
		if _, ok := out[groupID]; ok {
			continue
		}
		resp, err := s.GetIncrementalGroupMembers(ctx, &grouppb.GetIncrementalGroupMembersReq{
			GroupId:   groupID,
			UserId:    userID,
			VersionId: item.GetVersionId(),
			Version:   item.GetVersion(),
		})
		if err != nil {
			if errors.Is(err, errorcode.ErrGroupDismissed) {
				continue
			}
			return nil, err
		}
		out[groupID] = resp
	}
	return &grouppb.BatchGetIncrementalGroupMembersResp{Groups: out}, nil
}
