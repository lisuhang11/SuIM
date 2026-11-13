package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

// 表单上传完成后登记对象名。会话过期或不属于调用者时拒绝。
func (s *Server) CompleteFormData(ctx context.Context, req *thirdpb.CompleteFormDataReq) (*thirdpb.CompleteFormDataResp, error) {
	if err := checkMeta("", "", req.GetUrlPrefix()); err != nil {
		return nil, err
	}
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	ticket, err := s.open(req.GetId())
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, errorcode.ErrNoPermission
	}
	if err := checkName(userID, ticket.Name); err != nil {
		return nil, err
	}
	if !validDirectKey(ticket.Key) {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("invalid id")
	}
	size, etag, err := s.parts.Head(ctx, ticket.Key)
	if errors.Is(err, model.ErrNotFound) {
		return nil, errorcode.ErrThirdNotFound.WithMsg("object not found")
	}
	if err != nil {
		return nil, err
	}
	// Size 0 must still be checked. Skipping it let empty MinIO objects register
	// under a non-zero declared ticket size.
	if size != ticket.Size {
		return nil, errorcode.ErrThirdSizeMismatch
	}
	if err := s.bind(ctx, &model.Object{
		Name:        ticket.Name,
		UserID:      ticket.UserID,
		Hash:        objectHash(etag),
		StorageKey:  ticket.Key,
		Size:        size,
		ContentType: ticket.ContentType,
		Cause:       ticket.Cause,
	}); err != nil {
		return nil, err
	}
	return &thirdpb.CompleteFormDataResp{Url: req.GetUrlPrefix() + ticket.Name}, nil
}
