package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

// 分页返回调用者自己的好友。
func (s *Server) GetFriendList(ctx context.Context, req *relationpb.GetFriendListReq) (*relationpb.GetFriendListResp, error) {
	// 1. 校验用户和分页。调用方必须是本人。
	userID, err := actorID(ctx, req.GetUserId())
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
