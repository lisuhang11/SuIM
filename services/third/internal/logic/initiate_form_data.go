package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
)

func (s *Server) InitiateFormData(ctx context.Context, req *thirdpb.InitiateFormDataReq) (*thirdpb.InitiateFormDataResp, error) {
	if err := checkName(req.GetUserId(), req.GetName()); err != nil {
		return nil, err
	}
	if err := checkMeta(req.GetContentType(), req.GetCause(), ""); err != nil {
		return nil, err
	}
	if req.GetSize() <= 0 {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("size must be greater than 0")
	}
	if req.GetSize() > s.limits.MaxPartSize {
		return nil, errorcode.ErrThirdTooLarge
	}
	if strings.Contains(req.GetUserId(), "/") || strings.Contains(req.GetUserId(), `\`) {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("user_id is invalid")
	}
	key, err := directKey(req.GetUserId(), req.GetName(), s.now())
	if err != nil {
		return nil, err
	}
	expires := s.now().Add(formExpire)
	cred, err := s.parts.Form(ctx, key, req.GetSize(), req.GetContentType(), expires)
	if err != nil {
		return nil, err
	}
	id, err := s.seal(formTicket{
		Name:        req.GetName(),
		Size:        req.GetSize(),
		ContentType: req.GetContentType(),
		Cause:       req.GetCause(),
		Key:         key,
		UserID:      req.GetUserId(),
	})
	if err != nil {
		return nil, err
	}
	return &thirdpb.InitiateFormDataResp{
		Id:           id,
		Url:          cred.URL,
		File:         cred.File,
		FormData:     cred.FormData,
		Expires:      expires.UnixMilli(),
		SuccessCodes: int32s(cred.SuccessCodes),
	}, nil
}
