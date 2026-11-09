package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

// 按文件大小开始分片上传。相同哈希已有对象时直接复用，不再上传。
func (s *Server) InitiateMultipartUpload(ctx context.Context, req *thirdpb.InitiateMultipartUploadReq) (*thirdpb.InitiateMultipartUploadResp, error) {
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	hash, err := normalizeHash(req.GetHash())
	if err != nil {
		return nil, err
	}
	if err := checkName(userID, req.GetName()); err != nil {
		return nil, err
	}
	if err := checkMeta(req.GetContentType(), req.GetCause(), req.GetUrlPrefix()); err != nil {
		return nil, err
	}
	partSize, count, err := Plan(s.limits, req.GetSize())
	if err != nil {
		return nil, err
	}
	if req.GetPartSize() != 0 && req.GetPartSize() != partSize {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part_size does not match the server plan")
	}

	key := objectKey(hash)
	if resp, done, err := s.reuse(ctx, req, userID, hash); done || err != nil {
		return resp, err
	}

	storeID, err := s.parts.Create(ctx, key, req.GetContentType())
	if err != nil {
		return nil, err
	}
	id, err := newUploadID()
	if err != nil {
		_ = s.parts.Abort(ctx, key, storeID)
		return nil, err
	}
	expireAt := s.now().Add(s.expire)
	sess := &model.Session{
		ID:            id,
		UserID:        userID,
		Hash:          hash,
		Size:          req.GetSize(),
		PartSize:      partSize,
		PartCount:     count,
		ContentType:   req.GetContentType(),
		Cause:         req.GetCause(),
		Name:          req.GetName(),
		ExpireAt:      expireAt,
		StoreUploadID: storeID,
		ObjectKey:     key,
	}
	if err := s.sessions.CreateSession(ctx, sess); err != nil {
		_ = s.parts.Abort(ctx, key, storeID)
		return nil, err
	}
	sign, err := s.signParts(ctx, sess, 1, signCount(req.GetMaxParts(), count))
	if err != nil {
		_ = s.sessions.DeleteSession(ctx, id)
		_ = s.parts.Abort(ctx, key, storeID)
		return nil, err
	}
	return &thirdpb.InitiateMultipartUploadResp{
		Upload: &thirdpb.UploadInfo{
			UploadId:   id,
			PartSize:   partSize,
			ExpireTime: expireAt.UnixMilli(),
			Sign:       sign,
		},
	}, nil
}
