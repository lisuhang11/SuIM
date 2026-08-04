package service

import (
	"context"
	"strings"

	apperrors "group/internal/errors"
	"group/internal/types"
	"group/internal/types/interfaces"
)

// bumpMemberVersion increments one group's member-list version for the given entity IDs.
func bumpMemberVersion(ctx context.Context, repo interfaces.GroupRepository, groupID string, entityIDs []string, state int8) error {
	if groupID == "" || len(entityIDs) == 0 {
		return nil
	}
	return repo.IncrMemberVersion(ctx, groupID, entityIDs, state)
}

// GetIncrementalGroupMember returns OpenIM-style incremental member changes for one group.
func (s *groupService) GetIncrementalGroupMember(ctx context.Context, groupID, opUserID, versionID string, version uint64) (*types.IncrementalGroupMembersResult, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, apperrors.NewValidationError("group_id is required")
	}
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleNormal); err != nil {
		return nil, err
	}

	ver, err := s.repo.EnsureGroupMemberVersion(ctx, groupID)
	if err != nil {
		return nil, apperrors.NewInternalError("ensure group member version failed").WithDetails(err)
	}

	out := &types.IncrementalGroupMembersResult{
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

	logs, err := s.repo.ListGroupMemberVersionLogs(ctx, groupID, version, ver.Version)
	if err != nil {
		return nil, apperrors.NewInternalError("list group member version logs failed").WithDetails(err)
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

	deleteSet := map[string]struct{}{}
	insertSet := map[string]struct{}{}
	updateSet := map[string]struct{}{}
	groupChanged := false
	sortChanged := false

	for _, lg := range logs {
		switch lg.EntityID {
		case types.VersionGroupChangeID:
			groupChanged = true
			continue
		case types.VersionSortChangeID:
			sortChanged = true
			continue
		}

		switch lg.State {
		case types.VersionStateInsert:
			delete(deleteSet, lg.EntityID)
			delete(updateSet, lg.EntityID)
			insertSet[lg.EntityID] = struct{}{}
		case types.VersionStateDelete:
			delete(insertSet, lg.EntityID)
			delete(updateSet, lg.EntityID)
			deleteSet[lg.EntityID] = struct{}{}
		case types.VersionStateUpdate:
			if _, ok := insertSet[lg.EntityID]; ok {
				continue
			}
			if _, ok := deleteSet[lg.EntityID]; ok {
				continue
			}
			updateSet[lg.EntityID] = struct{}{}
		}
	}

	out.Delete = setKeys(deleteSet)

	loadIDs := make([]string, 0, len(insertSet)+len(updateSet))
	for id := range insertSet {
		loadIDs = append(loadIDs, id)
	}
	for id := range updateSet {
		loadIDs = append(loadIDs, id)
	}
	members, err := s.repo.ListMembersByIDs(ctx, groupID, loadIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("list members by ids failed").WithDetails(err)
	}
	byID := make(map[string]*types.GroupMember, len(members))
	for _, m := range members {
		byID[m.UserID] = m
	}
	for id := range insertSet {
		if m, ok := byID[id]; ok {
			out.Insert = append(out.Insert, m)
		} else {
			out.Delete = append(out.Delete, id)
		}
	}
	for id := range updateSet {
		if m, ok := byID[id]; ok {
			out.Update = append(out.Update, m)
		}
	}

	if groupChanged {
		g, err := s.repo.GetGroup(ctx, groupID)
		if err == nil {
			_ = s.fillMemberCounts(ctx, []*types.Group{g})
			out.Group = g
		}
	}
	if sortChanged {
		out.SortVersion = 1
	}
	return out, nil
}

// GetFullGroupMemberUserIDs returns ordered member IDs plus current version watermark.
func (s *groupService) GetFullGroupMemberUserIDs(ctx context.Context, groupID, opUserID string) ([]string, uint64, string, error) {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return nil, 0, "", apperrors.NewValidationError("group_id is required")
	}
	if _, _, err := s.authMember(ctx, groupID, opUserID, types.GroupMemberRoleNormal); err != nil {
		return nil, 0, "", err
	}
	ids, err := s.repo.ListOrderedMemberUserIDs(ctx, groupID)
	if err != nil {
		return nil, 0, "", apperrors.NewInternalError("list ordered member user ids failed").WithDetails(err)
	}
	ver, err := s.repo.EnsureGroupMemberVersion(ctx, groupID)
	if err != nil {
		return nil, 0, "", apperrors.NewInternalError("ensure group member version failed").WithDetails(err)
	}
	return ids, ver.Version, ver.VersionID, nil
}

func setKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
