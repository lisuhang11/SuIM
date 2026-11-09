package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

// 给已经接听的参与者签发进房令牌。未接听或不在这通电话里时拒绝。
func (s *Service) SignalGetTokenByRoomID(ctx context.Context, req *rtcpb.SignalGetTokenByRoomIDReq) (*rtcpb.SignalGetTokenByRoomIDResp, error) {
	if req == nil {
		return nil, errorcode.ErrCallInvalid
	}
	userID, err := rpcctx.Actor(ctx, req.GetUserId())
	if err != nil || userID == "" {
		return nil, errorcode.ErrNoPermission
	}
	if s == nil || s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if !s.media.Enabled() {
		return nil, errorcode.ErrInternal.WithMsg("livekit is not configured")
	}
	sig, err := s.calls.Accepted(ctx, userID, req.GetRoomId())
	if err != nil {
		return nil, err
	}
	url, token, err := s.sign(userID, req.GetRoomId(), sig.Media)
	if err != nil || token == "" {
		return nil, errorcode.ErrInternal.WithMsg("sign call token")
	}
	return &rtcpb.SignalGetTokenByRoomIDResp{Token: token, LiveUrl: url}, nil
}
