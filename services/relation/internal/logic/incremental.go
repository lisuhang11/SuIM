package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

// IncrementalLimit is how many version logs fit in one incremental response.
// Reaching it means the client should take a full snapshot instead.
const IncrementalLimit = 200

func (s *Server) GetIncrementalFriends(ctx context.Context, req *relationpb.GetIncrementalFriendsReq) (*relationpb.GetIncrementalFriendsResp, error) {
	// 1. 校验用户。调用方必须是本人，操作者身份尚未从上下文接入。
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 读取当前版本。版本号没变就原样返回。
	head, err := s.store.FriendVersion(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend version")
	}
	sortVersion, err := s.store.FriendSortVersion(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend sort version")
	}
	if !needFull(head, req.GetVersionId(), req.GetVersion()) && head.Version <= req.GetVersion() {
		return &relationpb.GetIncrementalFriendsResp{Version: head.Version, VersionId: head.ID, SortVersion: sortVersion}, nil
	}
	// 3. 版本差在限额内时返回增删改，否则整量返回当前好友。
	if !needFull(head, req.GetVersionId(), req.GetVersion()) {
		logs, err := s.store.FriendChanges(ctx, userID, req.GetVersion(), IncrementalLimit)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query friend changes")
		}
		if len(logs) < IncrementalLimit {
			resp, err := s.friendIncremental(ctx, userID, head, sortVersion, logs)
			if err != nil {
				return nil, err
			}
			return resp, nil
		}
	}
	rows, err := s.store.ListFriends(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list friends")
	}
	insert, err := s.friendInfos(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetIncrementalFriendsResp{
		Version:     head.Version,
		VersionId:   head.ID,
		Full:        true,
		Insert:      insert,
		SortVersion: sortVersion,
	}, nil
}

func (s *Server) GetIncrementalBlacks(ctx context.Context, req *relationpb.GetIncrementalBlacksReq) (*relationpb.GetIncrementalBlacksResp, error) {
	// 1. 校验用户。调用方必须是本人，操作者身份尚未从上下文接入。
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 读取当前版本。版本号没变就原样返回。
	head, err := s.store.BlackVersion(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query black version")
	}
	if !needFull(head, req.GetVersionId(), req.GetVersion()) && head.Version <= req.GetVersion() {
		return &relationpb.GetIncrementalBlacksResp{Version: head.Version, VersionId: head.ID}, nil
	}
	// 3. 版本差在限额内时返回增删改，否则整量返回当前黑名单。
	if !needFull(head, req.GetVersionId(), req.GetVersion()) {
		logs, err := s.store.BlackChanges(ctx, userID, req.GetVersion(), IncrementalLimit)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("query black changes")
		}
		if len(logs) < IncrementalLimit {
			return s.blackIncremental(ctx, userID, head, logs)
		}
	}
	rows, err := s.store.ListBlacks(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list blacks")
	}
	insert, err := s.blackInfos(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetIncrementalBlacksResp{
		Version:   head.Version,
		VersionId: head.ID,
		Full:      true,
		Insert:    insert,
	}, nil
}

func needFull(head model.VersionHead, versionID string, version uint64) bool {
	return version == 0 || versionID == "" || head.ID == "" || head.ID != versionID
}

func (s *Server) friendIncremental(ctx context.Context, userID string, head model.VersionHead, sortVersion uint64, logs []model.VersionLog) (*relationpb.GetIncrementalFriendsResp, error) {
	insertIDs, updateIDs, deleteIDs := splitLogs(logs)
	insertRows, err := s.store.FriendsByIDs(ctx, userID, insertIDs)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend inserts")
	}
	updateRows, err := s.store.FriendsByIDs(ctx, userID, updateIDs)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend updates")
	}
	insert, err := s.friendInfos(ctx, insertRows)
	if err != nil {
		return nil, err
	}
	update, err := s.friendInfos(ctx, updateRows)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetIncrementalFriendsResp{
		Version:     head.Version,
		VersionId:   head.ID,
		Delete:      deleteIDs,
		Insert:      insert,
		Update:      update,
		SortVersion: sortVersion,
	}, nil
}

func (s *Server) blackIncremental(ctx context.Context, userID string, head model.VersionHead, logs []model.VersionLog) (*relationpb.GetIncrementalBlacksResp, error) {
	insertIDs, updateIDs, deleteIDs := splitLogs(logs)
	insertRows, err := s.store.BlacksByIDs(ctx, userID, insertIDs)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query black inserts")
	}
	updateRows, err := s.store.BlacksByIDs(ctx, userID, updateIDs)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query black updates")
	}
	insert, err := s.blackInfos(ctx, insertRows)
	if err != nil {
		return nil, err
	}
	update, err := s.blackInfos(ctx, updateRows)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetIncrementalBlacksResp{
		Version:   head.Version,
		VersionId: head.ID,
		Delete:    deleteIDs,
		Insert:    insert,
		Update:    update,
	}, nil
}

func splitLogs(logs []model.VersionLog) (insertIDs, updateIDs, deleteIDs []string) {
	for _, log := range logs {
		if log.ElemID == model.SortElemID {
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

func (s *Server) friendInfos(ctx context.Context, rows []model.Friend) ([]*relationpb.FriendInfo, error) {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.FriendUserID
	}
	users, err := s.users.GetUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*relationpb.FriendInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, &relationpb.FriendInfo{
			OwnerUserId:    row.OwnerUserID,
			Remark:         row.Remark,
			CreateTime:     millis(row.CreateTime),
			FriendUser:     publicUser(row.FriendUserID, users),
			AddSource:      row.AddSource,
			OperatorUserId: row.OperatorUserID,
			IsPinned:       row.IsPinned,
		})
	}
	return out, nil
}

func (s *Server) blackInfos(ctx context.Context, rows []model.Black) ([]*relationpb.BlackInfo, error) {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.BlockUserID
	}
	users, err := s.users.GetUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*relationpb.BlackInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, &relationpb.BlackInfo{
			OwnerUserId:    row.OwnerUserID,
			CreateTime:     millis(row.CreateTime),
			BlackUser:      publicUser(row.BlockUserID, users),
			AddSource:      row.AddSource,
			OperatorUserId: row.OperatorUserID,
		})
	}
	return out, nil
}

func publicUser(id string, users map[string]model.UserProfile) *relationpb.PublicUserInfo {
	info := &relationpb.PublicUserInfo{UserId: id}
	if user, ok := users[id]; ok {
		info.Nickname = user.Nickname
		info.AvatarUrl = user.AvatarURL
	}
	return info
}

func millis(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}
