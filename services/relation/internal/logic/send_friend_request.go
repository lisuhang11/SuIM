package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

func (s *Server) SendFriendRequest(ctx context.Context, req *relationpb.SendFriendRequestReq) (*relationpb.SendFriendRequestResp, error) {
	// 1. 校验双方 userID。调用方必须是申请人，操作者身份尚未从上下文接入。
	from, err := normalizeUserID(req.GetFromUserId())
	if err != nil {
		return nil, err
	}
	to, err := normalizeUserID(req.GetToUserId())
	if err != nil {
		return nil, err
	}
	msg, err := normalizeMessage(req.GetMessage())
	if err != nil {
		return nil, err
	}
	// 2. 不能添加自己。
	if from == to {
		return nil, errorcode.ErrRelationCanNotAddYourself
	}
	// 3. 添加前 webhook。尚未接入。
	// 4. 确认双方用户存在。
	if err := s.users.CheckUsers(ctx, []string{to, from}); err != nil {
		return nil, err
	}
	// 5. 已经互为好友则拒绝。单方面关系仍允许再次申请。
	in1, in2, err := s.store.CheckIn(ctx, from, to)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check friendship")
	}
	if in1 && in2 {
		return nil, errorcode.ErrRelationAlreadyFriends
	}
	// 6. 写入好友申请。待处理的重复申请拒绝；已拒绝或已同意但不再互为好友的申请重新打开。
	if err := s.store.AddFriendRequest(ctx, from, to, msg, s.now()); err != nil {
		if errors.Is(err, store.ErrRequestRepeated) {
			return nil, errorcode.ErrRelationRequestRepeated
		}
		return nil, errorcode.ErrInternal.WithMsg("add friend request")
	}
	// 7. 通知对方收到好友申请。尚未接入。
	// 8. 添加后 webhook。尚未接入。
	return &relationpb.SendFriendRequestResp{}, nil
}
