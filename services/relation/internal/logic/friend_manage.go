package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

func (s *Server) IsFriend(ctx context.Context, req *relationpb.IsFriendReq) (*relationpb.IsFriendResp, error) {
	// 1. 校验双方 userID。调用方必须是其中一人，操作者身份尚未从上下文接入。
	user1, err := normalizeUserID(req.GetUser1())
	if err != nil {
		return nil, err
	}
	user2, err := normalizeUserID(req.GetUser2())
	if err != nil {
		return nil, err
	}
	// 2. 查询双方好友列表。
	in1, in2, err := s.store.CheckIn(ctx, user1, user2)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check friendship")
	}
	return &relationpb.IsFriendResp{InUser1Friends: in1, InUser2Friends: in2}, nil
}

func (s *Server) DeleteFriend(ctx context.Context, req *relationpb.DeleteFriendReq) (*relationpb.DeleteFriendResp, error) {
	// 1. 校验双方 userID。调用方必须是关系拥有者，操作者身份尚未从上下文接入。
	owner, err := normalizeUserID(req.GetUserId())
	if err != nil {
		return nil, err
	}
	friend, err := normalizeUserID(req.GetFriendId())
	if err != nil {
		return nil, err
	}
	// 2. 删除前 webhook。尚未接入。
	// 3. 只删除这一侧的好友关系。
	if err := s.store.DeleteFriend(ctx, owner, friend); err != nil {
		if errors.Is(err, store.ErrNotFriend) {
			return nil, errorcode.ErrRelationNotFriend
		}
		return nil, errorcode.ErrInternal.WithMsg("delete friend")
	}
	// 4. 通知对方好友已删除。尚未接入。
	// 5. 删除后 webhook。尚未接入。
	return &relationpb.DeleteFriendResp{}, nil
}

func (s *Server) UpdateFriend(ctx context.Context, req *relationpb.UpdateFriendReq) (*relationpb.UpdateFriendResp, error) {
	// 1. 校验双方 userID。调用方必须是关系拥有者，操作者身份尚未从上下文接入。
	owner, err := normalizeUserID(req.GetOwnerUserId())
	if err != nil {
		return nil, err
	}
	friend, err := normalizeUserID(req.GetFriendUserId())
	if err != nil {
		return nil, err
	}
	if req.Remark == nil && req.IsPinned == nil {
		return nil, errorcode.ErrRelationNothingToUpdate
	}
	var remark *string
	if req.Remark != nil {
		value, err := normalizeRemark(*req.Remark)
		if err != nil {
			return nil, err
		}
		remark = &value
	}
	// 2. 更新这一侧的备注或置顶。不是好友则拒绝。
	if err := s.store.UpdateFriend(ctx, owner, friend, remark, req.IsPinned); err != nil {
		if errors.Is(err, store.ErrNotFriend) {
			return nil, errorcode.ErrRelationNotFriend
		}
		return nil, errorcode.ErrInternal.WithMsg("update friend")
	}
	// 3. 通知好友资料变更。尚未接入。
	return &relationpb.UpdateFriendResp{}, nil
}
