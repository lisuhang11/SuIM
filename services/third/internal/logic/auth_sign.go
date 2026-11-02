package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/model"
)

func (s *Server) AuthSign(ctx context.Context, req *thirdpb.AuthSignReq) (*thirdpb.AuthSignResp, error) {
	userID, err := actorUser(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	if !validID(req.GetUploadId()) {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("upload_id is invalid")
	}
	if len(req.GetPartNumbers()) == 0 || len(req.GetPartNumbers()) > maxSignBatch {
		return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part_numbers is invalid")
	}
	sess, err := s.sessions.GetSession(ctx, req.GetUploadId())
	if errors.Is(err, model.ErrUploadNotFound) {
		return nil, errorcode.ErrThirdNotFound
	}
	if err != nil {
		return nil, err
	}
	if sess.UserID != userID {
		return nil, errorcode.ErrNoPermission
	}
	if !s.now().Before(sess.ExpireAt) {
		return nil, errorcode.ErrThirdNotFound.WithMsg("upload expired")
	}
	seen := make(map[int32]struct{}, len(req.GetPartNumbers()))
	numbers := make([]int, 0, len(req.GetPartNumbers()))
	for _, n := range req.GetPartNumbers() {
		if _, ok := seen[n]; ok {
			return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part number is repeated")
		}
		seen[n] = struct{}{}
		if n < 1 || int(n) > sess.PartCount {
			return nil, errorcode.ErrThirdInvalidArgs.WithMsg("part number is out of range")
		}
		numbers = append(numbers, int(n))
	}
	signed, err := s.signNumbers(ctx, sess, numbers)
	if err != nil {
		return nil, err
	}
	return &thirdpb.AuthSignResp{Parts: signed}, nil
}
