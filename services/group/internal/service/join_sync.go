package service

import (
	"context"

	apperrors "group/internal/errors"
	"group/internal/types"
	"group/internal/types/interfaces"
)

func bumpJoinVersions(ctx context.Context, repo interfaces.GroupRepository, userIDs []string, groupID string, state int8) error {
	for _, uid := range userIDs {
		if uid == "" {
			continue
		}
		if err := repo.IncrJoinVersion(ctx, uid, []string{groupID}, state); err != nil {
			return err
		}
	}
	return nil
}

// GetIncrementalJoinGroup 对齐 OpenIM：按水位返回 Full 或 delete/update/insert。
func (s *groupService) GetIncrementalJoinGroup(ctx context.Context, userID, versionID string, version uint64) (*types.IncrementalJoinGroupsResult, error) {
	ver, err := s.repo.EnsureJoinGroupVersion(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("ensure join group version failed").WithDetails(err)
	}
	out := &types.IncrementalJoinGroupsResult{
		Version:   ver.Version,
		VersionID: ver.VersionID,
	}
	if version == 0 || versionID == "" || versionID != ver.VersionID {
		out.Full = true
		return out, nil
	}
	if version >= ver.Version {
		return out, nil
	}

	logs, err := s.repo.ListJoinGroupVersionLogs(ctx, userID, version, ver.Version)
	if err != nil {
		return nil, apperrors.NewInternalError("list join group version logs failed").WithDetails(err)
	}
	if len(logs) == 0 || logs[0].Version != version+1 {
		out.Full = true
		return out, nil
	}
	for i := 1; i < len(logs); i++ {
		if logs[i].Version > logs[i-1].Version+1 {
			out.Full = true
			return out, nil
		}
	}

	m := make(map[string]int8)
	for _, lg := range logs {
		m[lg.GroupID] = lg.State
	}
	var delIDs, insertIDs, updateIDs, upsertIDs []string
	for gid, state := range m {
		switch state {
		case types.VersionStateDelete:
			delIDs = append(delIDs, gid)
		case types.VersionStateInsert:
			insertIDs = append(insertIDs, gid)
			upsertIDs = append(upsertIDs, gid)
		case types.VersionStateUpdate:
			updateIDs = append(updateIDs, gid)
			upsertIDs = append(upsertIDs, gid)
		}
	}

	groups, err := s.repo.ListGroupsByIDs(ctx, upsertIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("list groups by ids failed").WithDetails(err)
	}
	if err := s.fillMemberCounts(ctx, groups); err != nil {
		return nil, err
	}
	byID := make(map[string]*types.Group, len(groups))
	for _, g := range groups {
		byID[g.GroupID] = g
	}
	for _, id := range insertIDs {
		if g, ok := byID[id]; ok {
			out.Insert = append(out.Insert, g)
		}
	}
	for _, id := range updateIDs {
		if g, ok := byID[id]; ok {
			out.Update = append(out.Update, g)
		}
	}
	out.Delete = delIDs
	return out, nil
}

// GetFullJoinGroupIDs 返回排序后的完整已加入群 ID 列表。
func (s *groupService) GetFullJoinGroupIDs(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.repo.ListJoinGroupIDs(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("list join group ids failed").WithDetails(err)
	}
	return ids, nil
}
