package logic

import (
	"context"
	"time"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
)

// DeleteOutdatedData removes aged object rows and unused MinIO keys.
func (s *Server) DeleteOutdatedData(ctx context.Context, req *thirdpb.DeleteOutdatedDataReq) (*thirdpb.DeleteOutdatedDataResp, error) {
	if req.GetExpireTime() <= 0 {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("expire_time is required")
	}
	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 100
	}
	expireAt := time.UnixMilli(req.GetExpireTime()).UTC()
	objs, err := s.objects.FindExpired(ctx, expireAt, req.GetObjectGroup(), limit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query outdated objects")
	}
	for _, obj := range objs {
		if obj == nil {
			continue
		}
		if err := s.objects.DeleteByName(ctx, obj.Name); err != nil {
			return nil, errorcode.ErrInternal.WithMsg("delete object row")
		}
		count, err := s.objects.CountByStorageKey(ctx, obj.StorageKey)
		if err != nil {
			return nil, errorcode.ErrInternal.WithMsg("count storage key")
		}
		if count == 0 && obj.StorageKey != "" {
			if err := s.parts.Delete(ctx, obj.StorageKey); err != nil {
				return nil, errorcode.ErrInternal.WithMsg("delete object bytes")
			}
		}
	}
	return &thirdpb.DeleteOutdatedDataResp{Count: int32(len(objs))}, nil
}
