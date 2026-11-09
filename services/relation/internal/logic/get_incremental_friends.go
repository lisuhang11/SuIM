package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

// 一次增量响应最多带这么多版本日志。达到这个数时客户端应改拉全量。
const IncrementalLimit = 200

// 按版本返回好友变更。版本对不上或差额达到限额时只返回 full，不带好友资料。
func (s *Server) GetIncrementalFriends(ctx context.Context, req *relationpb.GetIncrementalFriendsReq) (*relationpb.GetIncrementalFriendsResp, error) {
	// 1. 校验用户。调用方必须是上下文里的本人。
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 读取当前版本。版本号没变就原样返回。
	head, err := s.friendHead(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend version")
	}
	// 3. 版本对不上，或差额达到限额，返回 full 且不带好友资料。
	if needFull(head, req.GetVersionId(), req.GetVersion()) {
		return &relationpb.GetIncrementalFriendsResp{Version: head.Version, VersionId: head.ID, Full: true}, nil
	}
	if head.Version <= req.GetVersion() {
		return &relationpb.GetIncrementalFriendsResp{Version: head.Version, VersionId: head.ID}, nil
	}
	logs, err := s.store.FriendChanges(ctx, userID, req.GetVersion(), IncrementalLimit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend changes")
	}
	if len(logs) >= IncrementalLimit {
		return &relationpb.GetIncrementalFriendsResp{Version: head.Version, VersionId: head.ID, Full: true}, nil
	}
	// 4. 限额内返回增删改的好友资料。这一批里有置顶时才带 sort_version。
	return s.friendIncremental(ctx, userID, head, logs)
}

func needFull(head model.VersionHead, versionID string, version uint64) bool {
	return version == 0 || versionID == "" || head.ID == "" || head.ID != versionID
}

func (s *Server) friendIncremental(ctx context.Context, userID string, head model.VersionHead, logs []model.VersionLog) (*relationpb.GetIncrementalFriendsResp, error) {
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
		SortVersion: sortVersionFrom(logs),
	}, nil
}

func sortVersionFrom(logs []model.VersionLog) uint64 {
	var version uint64
	for _, log := range logs {
		if log.ElemID == model.SortElemID && log.Version > version {
			version = log.Version
		}
	}
	return version
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
