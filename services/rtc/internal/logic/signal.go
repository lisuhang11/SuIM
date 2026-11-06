package logic

import (
	"context"
	"log/slog"
	"time"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/livekit"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

// Relations is the friend and blacklist check used before a private call rings.
type Relations interface {
	IsFriend(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
	IsBlack(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
}

// Service owns private-call state. msggateway only forwards the websocket frame.
type Service struct {
	rtcpb.UnimplementedRtcServiceServer
	calls     *call.Board
	relations Relations
	media     livekit.Config
}

func New(calls *call.Board, relations Relations, media livekit.Config) *Service {
	return &Service{calls: calls, relations: relations, media: media}
}

func (s *Service) SignalMessageAssemble(ctx context.Context, req *rtcpb.SignalMessageAssembleReq) (*rtcpb.SignalMessageAssembleResp, error) {
	if req == nil || req.SignalReq == nil {
		return nil, errorcode.ErrCallInvalid
	}
	userID, err := rpcctx.Actor(ctx, "")
	if err != nil || userID == "" {
		return nil, errorcode.ErrNoPermission
	}
	switch payload := req.SignalReq.Payload.(type) {
	case *rtcpb.SignalReq_Invite:
		return s.invite(ctx, userID, payload.Invite)
	case *rtcpb.SignalReq_Cancel:
		return s.end(ctx, userID, payload.Cancel.GetRoomId(), call.OpCancel)
	case *rtcpb.SignalReq_Accept:
		return s.accept(ctx, userID, req.GetConnId(), payload.Accept.GetRoomId())
	case *rtcpb.SignalReq_HangUp:
		return s.end(ctx, userID, payload.HangUp.GetRoomId(), call.OpHangup)
	case *rtcpb.SignalReq_Reject:
		return s.end(ctx, userID, payload.Reject.GetRoomId(), call.OpReject)
	default:
		return nil, errorcode.ErrCallInvalid
	}
}

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

// Sweep ends ringing calls that nobody answered and tells both people.
func (s *Service) Sweep(ctx context.Context) {
	if s == nil || s.calls == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ended, err := s.calls.Expire(ctx, time.Now())
			if err != nil {
				slog.Warn("expire calls", "err", err)
				continue
			}
			for _, sig := range ended {
				s.publish(ctx, sig, call.OpTimeout, []string{sig.CallerID, sig.CalleeID}, "")
			}
		}
	}
}

func (s *Service) invite(ctx context.Context, caller string, req *rtcpb.SignalInviteReq) (*rtcpb.SignalMessageAssembleResp, error) {
	if req == nil || req.Invitation == nil || req.Invitation.InviteeUserId == "" {
		return nil, errorcode.ErrCallInvalid
	}
	media := req.Invitation.MediaType
	if media != call.MediaAudio && media != call.MediaVideo {
		return nil, errorcode.ErrCallInvalid
	}
	invitee := req.Invitation.InviteeUserId
	if err := s.authorize(ctx, caller, invitee); err != nil {
		return nil, err
	}
	if s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	sig, err := s.calls.Invite(ctx, caller, invitee, media)
	if err != nil {
		return nil, err
	}
	s.publish(ctx, sig, call.OpRing, []string{sig.CalleeID}, "")
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_Invite{
		Invite: &rtcpb.SignalInviteResp{RoomId: sig.CallID},
	}}}, nil
}

func (s *Service) accept(ctx context.Context, userID, connID, roomID string) (*rtcpb.SignalMessageAssembleResp, error) {
	if s == nil || s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	sig, err := s.calls.Accept(ctx, userID, roomID)
	if err != nil {
		return nil, err
	}
	calleeURL, calleeToken, err := s.sign(userID, sig.CallID, sig.Media)
	if err != nil {
		s.abort(ctx, userID, sig)
		return nil, err
	}
	callerURL, callerToken, err := s.sign(sig.CallerID, sig.CallID, sig.Media)
	if err != nil {
		s.abort(ctx, userID, sig)
		return nil, err
	}
	caller := sig
	caller.URL = callerURL
	caller.Token = callerToken
	s.publish(ctx, caller, call.OpAccept, []string{sig.CallerID}, "")
	s.publish(ctx, sig, call.OpAnswered, []string{sig.CalleeID}, connID)
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_Accept{
		Accept: &rtcpb.SignalAcceptResp{Token: calleeToken, RoomId: sig.CallID, LiveUrl: calleeURL},
	}}}, nil
}

func (s *Service) end(ctx context.Context, userID, roomID, op string) (*rtcpb.SignalMessageAssembleResp, error) {
	if s == nil || s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	var (
		sig call.Signal
		err error
	)
	switch op {
	case call.OpCancel:
		sig, err = s.calls.Cancel(ctx, userID, roomID)
	case call.OpReject:
		sig, err = s.calls.Reject(ctx, userID, roomID)
	case call.OpHangup:
		sig, err = s.calls.Hangup(ctx, userID, roomID)
	default:
		return nil, errorcode.ErrCallInvalid
	}
	if err != nil {
		return nil, err
	}
	s.publish(ctx, sig, op, []string{peerOf(userID, sig)}, "")
	return ended(op), nil
}

func (s *Service) authorize(ctx context.Context, caller, callee string) error {
	if s == nil || s.relations == nil {
		return errorcode.ErrInternal.WithMsg("relation service is not configured")
	}
	// Relation rejects a call that does not name the operator. The gateway
	// already authenticated this user and stored that id on the context.
	ctx = rpcctx.WithUser(ctx, caller)
	blockedByCaller, blockedByCallee, err := s.relations.IsBlack(ctx, caller, callee)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check black")
	}
	if blockedByCaller || blockedByCallee {
		return errorcode.ErrMsgBlocked
	}
	callerHasFriend, calleeHasFriend, err := s.relations.IsFriend(ctx, caller, callee)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check friend")
	}
	if !callerHasFriend || !calleeHasFriend {
		return errorcode.ErrMsgNotFriend
	}
	return nil
}

func (s *Service) sign(identity, room, media string) (string, string, error) {
	if s == nil || !s.media.Enabled() {
		return "", "", nil
	}
	url, token, err := s.media.SignCall(identity, room, media, time.Now())
	if err != nil {
		slog.Warn("sign call token", "err", err, "callID", room)
		return "", "", errorcode.ErrInternal.WithMsg("sign call token")
	}
	return url, token, nil
}

func (s *Service) abort(ctx context.Context, userID string, sig call.Signal) {
	if _, err := s.calls.Hangup(ctx, userID, sig.CallID); err != nil {
		slog.Warn("hang up after token failure", "err", err, "callID", sig.CallID)
	}
	s.publish(ctx, sig, call.OpHangup, []string{sig.CallerID, sig.CalleeID}, "")
}

func (s *Service) publish(ctx context.Context, sig call.Signal, op string, users []string, except string) {
	if s == nil || s.calls == nil {
		return
	}
	out := sig
	out.Op = op
	if op != call.OpAccept {
		out.URL = ""
		out.Token = ""
	}
	if err := s.calls.Publish(ctx, call.Event{
		Users:  users,
		Except: except,
		Op:     out.Op,
		CallID: out.CallID,
		Caller: out.CallerID,
		Callee: out.CalleeID,
		Media:  out.Media,
		Reason: out.Reason,
		URL:    out.URL,
		Token:  out.Token,
	}); err != nil {
		slog.Warn("publish call signal", "err", err, "op", op, "callID", sig.CallID)
	}
}

func peerOf(userID string, sig call.Signal) string {
	if userID == sig.CallerID {
		return sig.CalleeID
	}
	return sig.CallerID
}

func ended(op string) *rtcpb.SignalMessageAssembleResp {
	resp := &rtcpb.SignalResp{}
	switch op {
	case call.OpCancel:
		resp.Payload = &rtcpb.SignalResp_Cancel{Cancel: &rtcpb.SignalCancelResp{}}
	case call.OpReject:
		resp.Payload = &rtcpb.SignalResp_Reject{Reject: &rtcpb.SignalRejectResp{}}
	default:
		resp.Payload = &rtcpb.SignalResp_HangUp{HangUp: &rtcpb.SignalHangUpResp{}}
	}
	return &rtcpb.SignalMessageAssembleResp{SignalResp: resp}
}
