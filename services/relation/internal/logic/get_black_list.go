package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/model"
)

// 分页返回调用者自己的黑名单。
func (s *Server) GetBlackList(ctx context.Context, req *relationpb.GetBlackListReq) (*relationpb.GetBlackListResp, error) {
	// 1. 校验用户和分页。
	userID, err := actorID(ctx, req.GetUserId())
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

func (s *Server) blackInfos(ctx context.Context, rows []model.Black) ([]*relationpb.BlackInfo, error) {
	ids := make([]string, len(rows))
	for i, row := range rows {
		ids[i] = row.BlockUserID
	}
	users, err := s.users.GetUsers(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]*relationpb.BlackInfo, 0, len(rows))
	for _, row := range rows {
		out = append(out, &relationpb.BlackInfo{
			OwnerUserId:    row.OwnerUserID,
			CreateTime:     millis(row.CreateTime),
			BlackUser:      publicUser(row.BlockUserID, users),
			AddSource:      row.AddSource,
			OperatorUserId: row.OperatorUserID,
		})
	}
	return out, nil
}
