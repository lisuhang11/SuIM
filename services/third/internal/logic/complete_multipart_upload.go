package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

func (s *Server) CompleteMultipartUpload(ctx context.Context, req *thirdpb.CompleteMultipartUploadReq) (*thirdpb.CompleteMultipartUploadResp, error) {
	if err := checkName(req.GetUserId(), req.GetName()); err != nil {
		return nil, err
	}
	if err := checkMeta(req.GetContentType(), req.GetCause(), req.GetUrlPrefix()); err != nil {
		return nil, err
	}
	sums, err := normalizeSums(req.GetParts())
	if err != nil {
		return nil, err
	}
	hash := joinHash(sums)
	sess, err := s.sessions.GetSession(ctx, req.GetUploadId())
	if errors.Is(err, model.ErrUploadNotFound) {
		return s.completeExisting(ctx, req, hash)
	}
	if err != nil {
		return nil, err
	}
	if sess.UserID != req.GetUserId() {
		return nil, errorcode.ErrNoPermission
	}
	if sess.Name != req.GetName() {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("name does not match the upload")
	}
	if len(sums) != sess.PartCount || hash != sess.Hash {
		return nil, errorcode.ErrThirdHashMismatch
	}
	if err := s.finishParts(ctx, sess, sums); err != nil {
		return nil, err
	}
	contentType := req.GetContentType()
	if contentType == "" {
		contentType = sess.ContentType
	}
	cause := req.GetCause()
	if cause == "" {
		cause = sess.Cause
	}
	if err := s.bind(ctx, &model.Object{
		Name:        req.GetName(),
		UserID:      req.GetUserId(),
		Hash:        sess.Hash,
		StorageKey:  sess.ObjectKey,
		Size:        sess.Size,
		ContentType: contentType,
		Cause:       cause,
	}); err != nil {
		return nil, err
	}
	_ = s.sessions.DeleteSession(ctx, sess.ID)
	return &thirdpb.CompleteMultipartUploadResp{Url: req.GetUrlPrefix() + req.GetName()}, nil
}
