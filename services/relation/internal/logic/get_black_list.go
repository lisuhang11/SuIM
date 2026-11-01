package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

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
