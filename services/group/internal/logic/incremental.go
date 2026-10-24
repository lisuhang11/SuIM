package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/services/group/internal/model"
)

const (
	syncLimit        = 200
	batchMemberLimit = 100
)

func (s *Server) GetIncrementalJoinGroups(ctx context.Context, req *grouppb.GetIncrementalJoinGroupsReq) (*grouppb.GetIncrementalJoinGroupsResp, error) {
	// 1. 校验用户。调用方必须是本人，操作者身份尚未从上下文接入。
	userID, err := normalizeUserID(req.GetUserId())
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

func (s *Server) GetFullJoinGroupIDs(ctx context.Context, req *grouppb.GetFullJoinGroupIDsReq) (*grouppb.GetFullJoinGroupIDsResp, error) {
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	head, err := s.store.JoinVersion(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query join version")
	}
	ids, err := s.store.AllJoinedGroupIDs(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list joined group ids")
	}
	sum := idhash.Hash(ids)
	resp := &grouppb.GetFullJoinGroupIDsResp{
		Version:   head.Version,
		VersionId: head.ID,
		Equal:     req.GetIdHash() == sum,
		GroupIds:  ids,
	}
	if resp.Equal {
		resp.GroupIds = nil
	}
	return resp, nil
}

func (s *Server) GetIncrementalGroupMembers(ctx context.Context, req *grouppb.GetIncrementalGroupMembersReq) (*grouppb.GetIncrementalGroupMembersResp, error) {
	// 1. 调用方必须是群成员，已解散的群不能增量同步。
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
		return nil, err
	}
	userID, err := normalizeUserID(req.GetUserId())
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

func (s *Server) GetFullGroupMemberUserIDs(ctx context.Context, req *grouppb.GetFullGroupMemberUserIDsReq) (*grouppb.GetFullGroupMemberUserIDsResp, error) {
	groupID, err := normalizeGroupID(req.GetGroupId())
	if err != nil {
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

func (s *Server) groupsByID(ctx context.Context, ids []string) ([]*grouppb.GroupInfo, error) {
	groups, err := s.store.FindGroups(ctx, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query groups")
	}
	out := make([]*grouppb.GroupInfo, 0, len(groups))
	for _, group := range groups {
		out = append(out, groupPB(group))
	}
	return out, nil
}

func (s *Server) membersByID(ctx context.Context, groupID string, ids []string) ([]*grouppb.GroupMemberFullInfo, error) {
	members, err := s.store.FindMembers(ctx, groupID, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query members")
	}
	if err := s.fillMembers(ctx, members); err != nil {
		return nil, err
	}
	out := make([]*grouppb.GroupMemberFullInfo, 0, len(members))
	for _, member := range members {
		out = append(out, memberPB(member))
	}
	return out, nil
}

func needFull(head model.VersionHead, versionID string, version uint64) bool {
	return version == 0 || versionID == "" || head.ID == "" || head.ID != versionID
}

func splitIDs(logs []model.VersionLog) (insertIDs, updateIDs, deleteIDs []string) {
	for _, log := range logs {
		if log.ElemID == "" || log.ElemID == model.SortElemID {
			continue
		}
		switch log.State {
		case model.VersionInsert:
			insertIDs = append(insertIDs, log.ElemID)
		case model.VersionUpdate:
			updateIDs = append(updateIDs, log.ElemID)
		case model.VersionDelete:
			deleteIDs = append(deleteIDs, log.ElemID)
		}
	}
	return insertIDs, updateIDs, deleteIDs
}

func splitMemberLogs(logs []model.VersionLog) (insertIDs, updateIDs, deleteIDs []string, groupChanged bool, sortVersion uint64) {
	for _, log := range logs {
		switch log.ElemID {
		case "":
			groupChanged = true
		case model.SortElemID:
			sortVersion = log.Version
		default:
			groupChanged = true
			switch log.State {
			case model.VersionInsert:
				insertIDs = append(insertIDs, log.ElemID)
			case model.VersionUpdate:
				updateIDs = append(updateIDs, log.ElemID)
			case model.VersionDelete:
				deleteIDs = append(deleteIDs, log.ElemID)
			}
		}
	}
	return insertIDs, updateIDs, deleteIDs, groupChanged, sortVersion
}
