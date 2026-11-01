package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

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
		value, err = s.beforeSetFriendRemark(ctx, owner, friend, value)
		if err != nil {
			return nil, err
		}
		value, err = normalizeRemark(value)
		if err != nil {
			return nil, err
		}
		remark = &value
	}
	// 2. 更新这一侧的备注或置顶。不是好友则拒绝。
	if err := s.store.UpdateFriend(ctx, owner, friend, remark, req.IsPinned, s.now()); err != nil {
		if errors.Is(err, store.ErrNotFriend) {
			return nil, errorcode.ErrRelationNotFriend
		}
		return nil, errorcode.ErrInternal.WithMsg("update friend")
	}
	// 3. 通知好友资料变更。尚未接入。
	if remark != nil {
		s.afterSetFriendRemark(ctx, owner, friend, *remark)
	}
	return &relationpb.UpdateFriendResp{}, nil
}
