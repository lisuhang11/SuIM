package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

func (s *Service) inviteGroup(ctx context.Context, caller string, req *rtcpb.SignalInviteInGroupReq) (*rtcpb.SignalMessageAssembleResp, error) {
	if req == nil || req.GetGroupId() == "" || (req.GetMediaType() != "" && req.GetMediaType() != call.MediaAudio) {
		return nil, errorcode.ErrCallInvalid
	}
	if s == nil || s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if s.groups == nil {
		return nil, errorcode.ErrInternal.WithMsg("group service is not configured")
	}
	if s.relations == nil {
		return nil, errorcode.ErrInternal.WithMsg("relation service is not configured")
	}
	ctx = rpcctx.WithUser(ctx, caller)
	member, err := s.groups.InGroup(ctx, req.GetGroupId(), caller)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check group")
	}
	if !member {
		return nil, errorcode.ErrGroupNotMember
	}
	invitees, blocked, err := s.ringable(ctx, caller, req.GetGroupId(), req.GetInviteeUserIds())
	if err != nil {
		return nil, err
	}
	if len(invitees) == 0 {
		if blocked {
			return nil, errorcode.ErrMsgBlocked
		}
		return nil, errorcode.ErrCallInvalid
	}
	if len(invitees) > call.MaxGroupVoice-1 {
		return nil, errorcode.ErrCallFull
	}
	step, err := s.calls.InviteGroup(ctx, caller, req.GetGroupId(), call.MediaAudio, invitees)
	if err != nil {
		return nil, err
	}
	if len(step.Ringing) == 0 {
		_, _ = s.calls.LeaveGroup(ctx, caller, step.CallID)
		return nil, errorcode.ErrCallPeerBusy
	}
	url, token, err := s.sign(caller, step.CallID, call.MediaAudio)
	if err != nil {
		_, _ = s.calls.LeaveGroup(ctx, caller, step.CallID)
		return nil, err
	}
	ring := step
	ring.Actor = ""
	s.publishGroup(ctx, ring, call.OpRing, step.Ringing, "", "")
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_InviteInGroup{
		InviteInGroup: &rtcpb.SignalInviteInGroupResp{
			Token: token, RoomId: step.CallID, LiveUrl: url, GroupId: step.GroupID, InviteeUserIds: step.Ringing,
		},
	}}}, nil
}

func (s *Service) acceptGroup(ctx context.Context, userID, connID, roomID string) (*rtcpb.SignalMessageAssembleResp, error) {
	step, err := s.calls.AcceptGroup(ctx, userID, roomID)
	if err != nil {
		return nil, err
	}
	url, token, err := s.sign(userID, step.CallID, step.Media)
	if err != nil {
		_ = s.calls.RestoreRing(ctx, userID, step.CallID)
		return nil, err
	}
	s.publishGroup(ctx, step, call.OpJoined, exceptID(step.Joined, userID), "", "")
	s.publishGroup(ctx, step, call.OpAnswered, []string{userID}, connID, "")
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_Accept{
		Accept: &rtcpb.SignalAcceptResp{
			Token: token, RoomId: step.CallID, LiveUrl: url, GroupId: step.GroupID, UserIds: step.Joined,
		},
	}}}, nil
}

func (s *Service) joinGroup(ctx context.Context, userID, connID, groupID string) (*rtcpb.SignalMessageAssembleResp, error) {
	if groupID == "" {
		return nil, errorcode.ErrCallInvalid
	}
	if s == nil || s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if s.groups == nil {
		return nil, errorcode.ErrInternal.WithMsg("group service is not configured")
	}
	ctx = rpcctx.WithUser(ctx, userID)
	member, err := s.groups.InGroup(ctx, groupID, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check group")
	}
	if !member {
		return nil, errorcode.ErrGroupNotMember
	}
	view, err := s.calls.Group(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if view.CallID == "" {
		return nil, errorcode.ErrCallNotFound
	}
	status, err := s.calls.MemberStatus(ctx, view.CallID, userID)
	if err != nil {
		return nil, err
	}
	step, err := s.calls.EnterGroup(ctx, userID, view.CallID)
	if err != nil {
		return nil, err
	}
	url, token, err := s.sign(userID, step.CallID, call.MediaAudio)
	if err != nil {
		if status == "ringing" {
			_ = s.calls.RestoreRing(ctx, userID, step.CallID)
		} else {
			_ = s.calls.UndoJoin(ctx, userID, step.CallID)
		}
		return nil, err
	}
	s.publishGroup(ctx, step, call.OpJoined, exceptID(step.Joined, userID), "", "")
	if status == "ringing" {
		s.publishGroup(ctx, step, call.OpAnswered, []string{userID}, connID, "")
	}
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_Join{
		Join: &rtcpb.SignalJoinResp{
			Token: token, RoomId: step.CallID, LiveUrl: url, GroupId: step.GroupID, UserIds: step.Joined,
		},
	}}}, nil
}

func (s *Service) groupCall(ctx context.Context, userID, groupID string) (*rtcpb.SignalMessageAssembleResp, error) {
	if groupID == "" {
		return nil, errorcode.ErrCallInvalid
	}
	if s == nil || s.calls == nil {
		return nil, errorcode.ErrInternal.WithMsg("call signaling is not configured")
	}
	if s.groups == nil {
		return nil, errorcode.ErrInternal.WithMsg("group service is not configured")
	}
	ctx = rpcctx.WithUser(ctx, userID)
	member, err := s.groups.InGroup(ctx, groupID, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("check group")
	}
	if !member {
		return nil, errorcode.ErrGroupNotMember
	}
	view, err := s.calls.Group(ctx, groupID)
	if err != nil {
		return nil, err
	}
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_GroupCall{
		GroupCall: &rtcpb.SignalGroupCallResp{
			RoomId: view.CallID, GroupId: groupID, CallerUserId: view.CallerID, UserIds: view.Joined,
		},
	}}}, nil
}

func (s *Service) endGroup(ctx context.Context, userID, roomID, op string) (*rtcpb.SignalMessageAssembleResp, error) {
	var (
		step call.GroupStep
		err  error
	)
	switch op {
	case call.OpCancel, call.OpHangup:
		step, err = s.calls.LeaveGroup(ctx, userID, roomID)
	case call.OpReject:
		step, err = s.calls.RejectGroup(ctx, userID, roomID)
	default:
		return nil, errorcode.ErrCallInvalid
	}
	if err != nil {
		return nil, err
	}
	s.afterGroup(ctx, step, op, "")
	return ended(op), nil
}

func (s *Service) afterGroup(ctx context.Context, step call.GroupStep, op, connID string) {
	if step.Ended {
		s.publishGroup(ctx, step, call.OpCancel, step.Ringing, "", call.ReasonEnded)
		s.publishGroup(ctx, step, call.OpTimeout, exceptID(step.Joined, step.Actor), "", call.ReasonEnded)
		switch op {
		case call.OpReject:
			s.publishGroup(ctx, step, call.OpReject, []string{step.Actor}, connID, "")
		case call.OpTimeout:
			s.publishGroup(ctx, step, call.OpTimeout, []string{step.Actor}, "", call.ReasonEnded)
		default:
			s.publishGroup(ctx, step, call.OpHangup, []string{step.Actor}, connID, call.ReasonEnded)
		}
		return
	}
	switch op {
	case call.OpTimeout:
		s.publishGroup(ctx, step, call.OpTimeout, []string{step.Actor}, "", call.ReasonUser)
		s.publishGroup(ctx, step, call.OpLeft, step.Joined, "", "")
	case call.OpReject:
		s.publishGroup(ctx, step, call.OpReject, []string{step.Actor}, connID, "")
		s.publishGroup(ctx, step, call.OpLeft, step.Joined, "", "")
	case call.OpHangup, call.OpCancel:
		s.publishGroup(ctx, step, call.OpHangup, []string{step.Actor}, connID, "")
		s.publishGroup(ctx, step, call.OpLeft, step.Joined, "", "")
	case call.OpJoined:
		s.publishGroup(ctx, step, call.OpJoined, exceptID(step.Joined, step.Actor), "", "")
		s.publishGroup(ctx, step, call.OpAnswered, []string{step.Actor}, connID, "")
	}
}

func (s *Service) publishGroup(ctx context.Context, step call.GroupStep, op string, users []string, except, reason string) {
	if len(users) == 0 {
		return
	}
	s.publish(ctx, call.Signal{
		CallID: step.CallID, CallerID: step.CallerID, Media: step.Media, Reason: reason,
		GroupID: step.GroupID, Actor: step.Actor, Roster: step.Joined,
	}, op, users, except)
}

func (s *Service) ringable(ctx context.Context, caller, groupID string, ids []string) ([]string, bool, error) {
	seen := make(map[string]struct{}, len(ids))
	out := make([]string, 0, len(ids))
	blocked := false
	for _, id := range ids {
		if id == "" || id == caller {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		member, err := s.groups.InGroup(ctx, groupID, id)
		if err != nil {
			return nil, false, errorcode.ErrInternal.WithMsg("check group")
		}
		if !member {
			continue
		}
		byCaller, byInvitee, err := s.relations.IsBlack(ctx, caller, id)
		if err != nil {
			return nil, false, errorcode.ErrInternal.WithMsg("check black")
		}
		if byCaller || byInvitee {
			blocked = true
			continue
		}
		out = append(out, id)
	}
	return out, blocked, nil
}

func exceptID(ids []string, skip string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && id != skip {
			out = append(out, id)
		}
	}
	return out
}
