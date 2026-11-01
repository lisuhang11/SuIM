package logic

import (
	"context"

	thirdpb "github.com/lisuhang11/SuIM/proto/third"
)

func (s *Server) PartLimit(context.Context, *thirdpb.PartLimitReq) (*thirdpb.PartLimitResp, error) {
	return &thirdpb.PartLimitResp{
		MinPartSize: s.limits.MinPartSize,
		MaxPartSize: s.limits.MaxPartSize,
		MaxNumSize:  int32(s.limits.MaxNumSize),
	}, nil
}
