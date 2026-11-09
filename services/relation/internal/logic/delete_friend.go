package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

// 只删除调用者这一侧的好友关系。对方的好友行还在。
func (s *Server) DeleteFriend(ctx context.Context, req *relationpb.DeleteFriendReq) (*relationpb.DeleteFriendResp, error) {
	// 1. 校验双方 userID。关系拥有者取上下文里的用户。
	owner, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	friend, err := normalizeUserID(req.GetFriendId())
	if err != nil {
		return nil, err
	}
	// 2. 只删除这一侧的好友关系。
	if err := s.store.DeleteFriend(ctx, owner, friend, s.now()); err != nil {
		if errors.Is(err, store.ErrNotFriend) {
			return nil, errorcode.ErrRelationNotFriend
		}
		return nil, errorcode.ErrInternal.WithMsg("delete friend")
	}
	s.dropFriends(ctx, owner)
	// 3. 通知对方好友已删除，并调用删除后 webhook。
	s.notifyFriendDeleted(ctx, owner, friend)
	s.afterDeleteFriend(ctx, owner, friend)
	return &relationpb.DeleteFriendResp{}, nil
}
