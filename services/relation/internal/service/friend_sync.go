package service

import (
	"context"

	apperrors "relation/internal/errors"
	"relation/internal/logger"
	"relation/internal/types"
)

// GetIncrementalFriends 对齐 OpenIM：按水位返回 Full 或 delete/update/insert。
func (s *relationService) GetIncrementalFriends(ctx context.Context, userID, versionID string, version uint64) (*types.IncrementalFriendsResult, error) {
	ver, err := s.repo.EnsureFriendVersion(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("ensure friend version failed").WithDetails(err)
	}

	out := &types.IncrementalFriendsResult{
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

	logs, err := s.repo.ListFriendVersionLogs(ctx, userID, version, ver.Version)
	if err != nil {
		return nil, apperrors.NewInternalError("list friend version logs failed").WithDetails(err)
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

	type change struct {
		state  int8
		isSort bool
	}
	m := make(map[string]change)
	var sortVersion uint64
	for _, lg := range logs {
		m[lg.FriendUserID] = change{state: lg.State, isSort: lg.IsSort}
		if lg.IsSort && lg.Version > sortVersion {
			sortVersion = lg.Version
		}
	}

	var delIDs, insertIDs, updateIDs, upsertIDs []string
	for fid, ch := range m {
		switch ch.state {
		case types.VersionStateDelete:
			delIDs = append(delIDs, fid)
		case types.VersionStateInsert:
			insertIDs = append(insertIDs, fid)
			upsertIDs = append(upsertIDs, fid)
		case types.VersionStateUpdate:
			updateIDs = append(updateIDs, fid)
			upsertIDs = append(upsertIDs, fid)
		}
	}

	friends, err := s.repo.ListFriendsByIDs(ctx, userID, upsertIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("list friends by ids failed").WithDetails(err)
	}
	byID := make(map[string]*types.Friend, len(friends))
	for _, f := range friends {
		byID[f.FriendUserID] = f
	}

	for _, id := range insertIDs {
		if f, ok := byID[id]; ok {
			out.Insert = append(out.Insert, f)
		}
	}
	for _, id := range updateIDs {
		if f, ok := byID[id]; ok {
			out.Update = append(out.Update, f)
		}
	}
	out.Delete = delIDs
	out.SortVersion = sortVersion
	return out, nil
}

// GetFullFriendUserIDs 返回排序后的完整好友 ID 列表。
func (s *relationService) GetFullFriendUserIDs(ctx context.Context, userID string) ([]string, error) {
	ids, err := s.repo.ListFriendUserIDs(ctx, userID)
	if err != nil {
		return nil, apperrors.NewInternalError("list friend user ids failed").WithDetails(err)
	}
	return ids, nil
}

// NotificationUserInfoUpdate bumps version for every owner who friended changedUserID and pushes tips.
func (s *relationService) NotificationUserInfoUpdate(ctx context.Context, changedUserID string) error {
	if changedUserID == "" {
		return apperrors.NewValidationError("user_id required")
	}
	owners, err := s.repo.FindOwnerUserIDsWhoFriended(ctx, changedUserID)
	if err != nil {
		return apperrors.NewInternalError("find friend owners failed").WithDetails(err)
	}
	for _, owner := range owners {
		if owner == changedUserID {
			continue
		}
		if err := s.repo.IncrVersion(ctx, owner, []string{changedUserID}, types.VersionStateUpdate, false); err != nil {
			logger.Error(ctx, "[relation] OwnerIncrVersion on profile update failed", "owner", owner, "error", err)
			continue
		}
		if s.notifier != nil {
			s.notifier.FriendInfoUpdatedNotification(ctx, changedUserID, owner)
		}
	}
	return nil
}
