package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	"github.com/lisuhang11/SuIM/proto/relationpb"
)

func (s *Server) GetFriendList(ctx context.Context, req *relationpb.GetFriendListReq) (*relationpb.GetFriendListResp, error) {
	// 1. 校验用户和分页。调用方必须是本人，操作者身份尚未从上下文接入。
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	// 2. 按置顶、添加时间、用户 ID 的固定顺序分页。
	total, rows, err := s.store.PageFriends(ctx, userID, offset, limit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list friends")
	}
	friends, err := s.friendInfos(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetFriendListResp{Friends: friends, Total: int32(total)}, nil
}

func (s *Server) GetFullFriendUserIDs(ctx context.Context, req *relationpb.GetFullFriendUserIDsReq) (*relationpb.GetFullFriendUserIDsResp, error) {
	// 1. 校验用户。
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 读取版本和有序 ID。哈希相同则不回传 ID。
	head, err := s.store.FriendVersion(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend version")
	}
	ids, err := s.store.FriendUserIDs(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list friend ids")
	}
	sum := idhash.Hash(ids)
	resp := &relationpb.GetFullFriendUserIDsResp{
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

func (s *Server) GetBlackList(ctx context.Context, req *relationpb.GetBlackListReq) (*relationpb.GetBlackListResp, error) {
	// 1. 校验用户和分页。
	userID, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	offset, limit, err := page(req.GetOffset(), req.GetLimit())
	if err != nil {
		return nil, err
	}
	// 2. 按拉黑时间、用户 ID 的固定顺序分页。
	total, rows, err := s.store.PageBlacks(ctx, userID, offset, limit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list blacks")
	}
	blacks, err := s.blackInfos(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetBlackListResp{Blacks: blacks, Total: int32(total)}, nil
}

func page(offset, limit int32) (int, int, error) {
	if offset < 0 || limit < 0 || limit > 200 {
		return 0, 0, errorcode.ErrRelationInvalidPage
	}
	if limit == 0 {
		limit = 20
	}
	return int(offset), int(limit), nil
}
