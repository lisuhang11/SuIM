package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

func (s *Server) GetIncrementalGroupMembers(ctx context.Context, req *grouppb.GetIncrementalGroupMembersReq) (*grouppb.GetIncrementalGroupMembersResp, error) {
	// 1. 调用方必须是群成员，已解散的群不能增量同步。
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	userID, err := bindActor(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	if _, err := s.memberOf(ctx, groupID, userID); err != nil {
		return nil, err
	}
	group, err := s.activeGroup(ctx, groupID)
	if err != nil {
		return nil, err
	}
	// 2. 版本对不上，或差额达到限额，返回 full 且不带成员资料。
	head, err := s.store.MemberVersion(ctx, groupID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query member version")
	}
	resp := &grouppb.GetIncrementalGroupMembersResp{Version: head.Version, VersionId: head.ID}
	var changed bool
	if needFull(head, req.GetVersionId(), req.GetVersion()) {
		resp.Full = true
		changed = true
	} else if head.Version > req.GetVersion() {
		logs, err := s.store.MemberChanges(ctx, groupID, req.GetVersion(), syncLimit)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query member changes")
		}
		if len(logs) >= syncLimit {
			resp.Full = true
			changed = true
		} else {
			insertIDs, updateIDs, deleteIDs, groupChanged, sortVersion := splitMemberLogs(logs)
			insert, err := s.membersByID(ctx, groupID, insertIDs)
			if err != nil {
				return nil, err
			}
			update, err := s.membersByID(ctx, groupID, updateIDs)
			if err != nil {
				return nil, err
			}
			resp.Delete = deleteIDs
			resp.Insert = insert
			resp.Update = update
			resp.SortVersion = sortVersion
			changed = groupChanged
		}
	}
	// 3. 整量同步或成员有变化时附带群资料。
	if resp.Full || changed {
		resp.Group = groupPB(group)
	}
	return resp, nil
}
