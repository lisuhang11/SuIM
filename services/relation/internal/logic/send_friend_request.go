package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

func (s *Server) SendFriendRequest(ctx context.Context, req *relationpb.SendFriendRequestReq) (*relationpb.SendFriendRequestResp, error) {
	// 1. 校验双方 userID。申请人取上下文里的用户。
	from, err := actorID(ctx, req.GetFromUserId())
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
	// 3. 确认双方用户存在。
	if err := s.users.CheckUsers(ctx, []string{to, from}); err != nil {
		return nil, err
	}
	fromInfo, toInfo := callbackUser{UserID: from}, callbackUser{UserID: to}
	if profiles, err := s.users.GetUsers(ctx, []string{from, to}); err == nil {
		if profile, ok := profiles[from]; ok {
			fromInfo.Nickname = profile.Nickname
			fromInfo.FaceURL = profile.AvatarURL
		}
		if profile, ok := profiles[to]; ok {
			toInfo.Nickname = profile.Nickname
			toInfo.FaceURL = profile.AvatarURL
		}
	}
	// 4. 添加前 webhook。回调拒绝则中断。
	if err := s.beforeAddFriend(ctx, from, to, msg, fromInfo, toInfo); err != nil {
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
	// 6. 任一方拉黑则拒绝。查黑名单失败也不写入。
	if err := s.rejectIfBlocked(ctx, from, to); err != nil {
		return nil, err
	}
	// 7. 共同的未解散群禁止互加时拒绝。group 暂时不可用也拒绝，避免开关被绕过。
	if s.groups != nil {
		allowed, err := s.groups.MemberFriendAllowed(ctx, from, to)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("check group friend")
		}
		if !allowed {
			return nil, errorcode.ErrRelationGroupFriendForbidden
		}
	}
	// 8. 写入好友申请。待处理的重复申请拒绝；已拒绝或已同意但不再互为好友的申请重新打开。
	if err := s.store.AddFriendRequest(ctx, from, to, msg, s.now()); err != nil {
		if errors.Is(err, store.ErrRequestRepeated) {
			return nil, errorcode.ErrRelationRequestRepeated
		}
		return nil, errorcode.ErrInternal.WithMsg("add friend request")
	}
	// 9. 通知对方收到好友申请，并调用添加后 webhook。
	s.notifyFriendApply(ctx, from, to)
	s.afterAddFriend(ctx, from, to, msg, fromInfo, toInfo)
	return &relationpb.SendFriendRequestResp{}, nil
}
