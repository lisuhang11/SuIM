package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

func (s *Server) GetRecvFriendRequests(ctx context.Context, req *relationpb.GetRecvFriendRequestsReq) (*relationpb.GetRecvFriendRequestsResp, error) {
	// 1. 校验用户、分页和处理状态。调用方必须是本人，操作者身份尚未从上下文接入。
	rows, total, err := s.friendRequests(ctx, req.GetUserId(), req.GetOffset(), req.GetLimit(), req.GetHandleResults(), true)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetRecvFriendRequestsResp{Requests: rows, Total: total}, nil
}

func (s *Server) GetSentFriendRequests(ctx context.Context, req *relationpb.GetSentFriendRequestsReq) (*relationpb.GetSentFriendRequestsResp, error) {
	// 1. 校验用户、分页和处理状态。
	rows, total, err := s.friendRequests(ctx, req.GetUserId(), req.GetOffset(), req.GetLimit(), req.GetHandleResults(), false)
	if err != nil {
		return nil, err
	}
	return &relationpb.GetSentFriendRequestsResp{Requests: rows, Total: total}, nil
}

func (s *Server) friendRequests(ctx context.Context, userID string, offset, limit int32, results []int32, recv bool) ([]*relationpb.FriendRequestInfo, int32, error) {
	id, err := normalizeUserID(userID)
	if err != nil {
		return nil, 0, err
	}
	off, lim, err := page(offset, limit)
	if err != nil {
		return nil, 0, err
	}
	handles, err := normalizeHandleResults(results)
	if err != nil {
		return nil, 0, err
	}
	// 2. 按申请时间倒序分页。申请记录不走缓存。
	var (
		total int
		rows  []model.FriendRequest
	)
	if recv {
		total, rows, err = s.store.PageRecvFriendRequests(ctx, id, handles, off, lim)
	} else {
		total, rows, err = s.store.PageSentFriendRequests(ctx, id, handles, off, lim)
	}
	if err != nil {
		return nil, 0, errorcode.ErrInternal.WithMsg("list friend requests")
	}
	// 3. 补双方昵称和头像。用户不存在时资料留空。
	infos, err := s.requestInfos(ctx, rows)
	if err != nil {
		return nil, 0, err
	}
	return infos, int32(total), nil
}

func normalizeHandleResults(results []int32) ([]int32, error) {
	if len(results) == 0 {
		return nil, nil
	}
	seen := make(map[int32]struct{}, len(results))
	out := make([]int32, 0, len(results))
	for _, result := range results {
		switch result {
		case model.HandlePending, model.HandleAgree, model.HandleRefuse:
		default:
			return nil, errorcode.ErrRelationInvalidHandle
		}
		if _, ok := seen[result]; ok {
			continue
		}
		seen[result] = struct{}{}
		out = append(out, result)
	}
	return out, nil
}

func (s *Server) requestInfos(ctx context.Context, rows []model.FriendRequest) ([]*relationpb.FriendRequestInfo, error) {
	seen := make(map[string]struct{}, len(rows)*2)
	ids := make([]string, 0, len(rows)*2)
	for _, row := range rows {
		for _, id := range []string{row.FromUserID, row.ToUserID} {
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			ids = append(ids, id)
		}
	}
	users, err := s.users.GetUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*relationpb.FriendRequestInfo, 0, len(rows))
	for _, row := range rows {
		info := &relationpb.FriendRequestInfo{
			FromUserId:    row.FromUserID,
			ToUserId:      row.ToUserID,
			Message:       row.ReqMsg,
			HandleMsg:     row.HandleMsg,
			HandleResult:  row.HandleResult,
			CreateTime:    millis(row.CreateTime),
			HandleTime:    millis(row.HandleTime),
			HandlerUserId: row.HandlerUserID,
		}
		if user, ok := users[row.FromUserID]; ok {
			info.FromNickname = user.Nickname
			info.FromAvatarUrl = user.AvatarURL
		}
		if user, ok := users[row.ToUserID]; ok {
			info.ToNickname = user.Nickname
			info.ToAvatarUrl = user.AvatarURL
		}
		out = append(out, info)
	}
	return out, nil
}
