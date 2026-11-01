package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

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
