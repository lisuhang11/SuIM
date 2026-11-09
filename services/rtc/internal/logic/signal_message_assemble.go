package logic

import (
	"context"
	"log/slog"
	"time"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

// 按信令种类转接邀请、接听、拒绝、取消、挂断和群通话。未识别的载荷拒绝。
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
	case *rtcpb.SignalReq_InviteInGroup:
		return s.inviteGroup(ctx, userID, payload.InviteInGroup)
	case *rtcpb.SignalReq_Cancel:
		return s.end(ctx, userID, payload.Cancel.GetRoomId(), call.OpCancel)
	case *rtcpb.SignalReq_Accept:
		return s.accept(ctx, userID, req.GetConnId(), payload.Accept.GetRoomId())
	case *rtcpb.SignalReq_HangUp:
		return s.end(ctx, userID, payload.HangUp.GetRoomId(), call.OpHangup)
	case *rtcpb.SignalReq_Reject:
		return s.end(ctx, userID, payload.Reject.GetRoomId(), call.OpReject)
	case *rtcpb.SignalReq_Join:
		return s.joinGroup(ctx, userID, req.GetConnId(), payload.Join.GetGroupId())
	case *rtcpb.SignalReq_GroupCall:
		return s.groupCall(ctx, userID, payload.GroupCall.GetGroupId())
	default:
		return nil, errorcode.ErrCallInvalid
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
	kind, err := s.calls.Kind(ctx, roomID)
	if err != nil {
		return nil, err
	}
	if kind == call.KindGroup {
		return s.acceptGroup(ctx, userID, connID, roomID)
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
	kind, kindErr := s.calls.Kind(ctx, roomID)
	if kindErr != nil {
		return nil, kindErr
	}
	if kind == call.KindGroup {
		return s.endGroup(ctx, userID, roomID, op)
	}
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
	// 关系服务会拒绝没有操作者的调用。网关已经鉴定过这个用户，并把 ID 放进上下文。
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
		Group:  out.GroupID,
		Actor:  out.Actor,
		Roster: out.Roster,
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
