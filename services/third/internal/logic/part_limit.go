package logic

import (
	"context"

	thirdpb "github.com/lisuhang11/SuIM/proto/third"
)

// 返回当前的分片大小和分片数量上限。不看请求体。
func (s *Server) PartLimit(context.Context, *thirdpb.PartLimitReq) (*thirdpb.PartLimitResp, error) {
	return &thirdpb.PartLimitResp{
		MinPartSize: s.limits.MinPartSize,
		MaxPartSize: s.limits.MaxPartSize,
		MaxNumSize:  int32(s.limits.MaxNumSize),
	}, nil
}
