package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// 按版本返回调用者的入群变更。版本对不上或差额达到限额时只返回 full，不带群资料。
func (s *Server) GetIncrementalJoinGroups(ctx context.Context, req *grouppb.GetIncrementalJoinGroupsReq) (*grouppb.GetIncrementalJoinGroupsResp, error) {
	// 1. 校验用户。调用方必须是上下文里的本人。
	userID, err := bindActor(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 版本对不上，或差额达到限额，返回 full 且不带群资料。
	head, err := s.store.JoinVersion(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query join version")
	}
	if needFull(head, req.GetVersionId(), req.GetVersion()) {
		return &grouppb.GetIncrementalJoinGroupsResp{Version: head.Version, VersionId: head.ID, Full: true}, nil
	}
	if head.Version <= req.GetVersion() {
		return &grouppb.GetIncrementalJoinGroupsResp{Version: head.Version, VersionId: head.ID}, nil
	}
	logs, err := s.store.JoinChanges(ctx, userID, req.GetVersion(), syncLimit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query join changes")
	}
	if len(logs) >= syncLimit {
		return &grouppb.GetIncrementalJoinGroupsResp{Version: head.Version, VersionId: head.ID, Full: true}, nil
	}
	// 3. 限额内返回增删改的群资料。
	insertIDs, updateIDs, deleteIDs := splitIDs(logs)
	insert, err := s.groupsByID(ctx, insertIDs)
	if err != nil {
		return nil, err
	}
	update, err := s.groupsByID(ctx, updateIDs)
	if err != nil {
		return nil, err
	}
	return &grouppb.GetIncrementalJoinGroupsResp{
		Version:   head.Version,
		VersionId: head.ID,
		Delete:    deleteIDs,
		Insert:    insert,
		Update:    update,
	}, nil
}
