package rpc

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

// RtcClient forwards a websocket call signal to the rtc service.
type RtcClient struct {
	client rtcpb.RtcServiceClient
}

func NewRtcClient(client rtcpb.RtcServiceClient) *RtcClient {
	return &RtcClient{client: client}
}

func (c *RtcClient) Assemble(ctx context.Context, userID, connID string, in *msgpb.CallSignal) (*msgpb.CallSignal, error) {
	if c == nil || c.client == nil {
		return nil, errorcode.ErrInternal.WithMsg("rtc service is not configured")
	}
	req, err := signalReq(in)
	if err != nil {
		return nil, err
	}
	resp, err := c.client.SignalMessageAssemble(rpcctx.WithUser(ctx, userID), &rtcpb.SignalMessageAssembleReq{
		ConnId:    connID,
		SignalReq: req,
	})
	if err != nil {
		return nil, err
	}
	return signalReply(userID, in, resp.GetSignalResp())
}

func signalReq(in *msgpb.CallSignal) (*rtcpb.SignalReq, error) {
	if in == nil {
		return nil, errorcode.ErrCallInvalid
	}
	switch in.GetOp() {
	case msgpb.CallOp_CALL_INVITE:
		if in.GetGroupId() != "" {
			return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_InviteInGroup{InviteInGroup: &rtcpb.SignalInviteInGroupReq{
				GroupId: in.GetGroupId(), InviteeUserIds: in.GetUserIds(), MediaType: mediaType(in.GetMedia()),
			}}}, nil
		}
		return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Invite{Invite: &rtcpb.SignalInviteReq{
			Invitation: &rtcpb.InvitationInfo{InviteeUserId: in.GetPeerId(), MediaType: mediaType(in.GetMedia())},
		}}}, nil
	case msgpb.CallOp_CALL_JOIN:
		return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Join{Join: &rtcpb.SignalJoinReq{GroupId: in.GetGroupId()}}}, nil
	case msgpb.CallOp_CALL_GROUP:
		return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_GroupCall{GroupCall: &rtcpb.SignalGroupCallReq{GroupId: in.GetGroupId()}}}, nil
	case msgpb.CallOp_CALL_CANCEL:
		return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Cancel{Cancel: &rtcpb.SignalCancelReq{RoomId: in.GetCallId()}}}, nil
	case msgpb.CallOp_CALL_ACCEPT:
		return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Accept{Accept: &rtcpb.SignalAcceptReq{RoomId: in.GetCallId()}}}, nil
	case msgpb.CallOp_CALL_HANGUP:
		return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_HangUp{HangUp: &rtcpb.SignalHangUpReq{RoomId: in.GetCallId()}}}, nil
	case msgpb.CallOp_CALL_REJECT:
		return &rtcpb.SignalReq{Payload: &rtcpb.SignalReq_Reject{Reject: &rtcpb.SignalRejectReq{RoomId: in.GetCallId()}}}, nil
	default:
		return nil, errorcode.ErrCallInvalid
	}
}

func mediaType(media msgpb.CallMedia) string {
	switch media {
	case msgpb.CallMedia_CALL_AUDIO:
		return call.MediaAudio
	case msgpb.CallMedia_CALL_VIDEO:
		return call.MediaVideo
	default:
		return ""
	}
}

func replyMedia(media string, fallback msgpb.CallMedia) msgpb.CallMedia {
	switch media {
	case call.MediaAudio:
		return msgpb.CallMedia_CALL_AUDIO
	case call.MediaVideo:
		return msgpb.CallMedia_CALL_VIDEO
	default:
		if fallback == msgpb.CallMedia_CALL_VIDEO {
			return msgpb.CallMedia_CALL_VIDEO
		}
		if fallback == msgpb.CallMedia_CALL_AUDIO {
			return msgpb.CallMedia_CALL_AUDIO
		}
		return msgpb.CallMedia_CALL_AUDIO
	}
}

func signalReply(userID string, in *msgpb.CallSignal, resp *rtcpb.SignalResp) (*msgpb.CallSignal, error) {
	if resp == nil {
		return nil, errorcode.ErrInternal.WithMsg("empty rtc response")
	}
	switch payload := resp.Payload.(type) {
	case *rtcpb.SignalResp_Invite:
		if payload.Invite.GetRoomId() == "" {
			return nil, errorcode.ErrInternal.WithMsg("empty rtc response")
		}
		return &msgpb.CallSignal{
			Op: msgpb.CallOp_CALL_INVITE, CallId: payload.Invite.GetRoomId(), PeerId: in.GetPeerId(),
			Media: in.GetMedia(), CallerId: userID, CalleeId: in.GetPeerId(),
		}, nil
	case *rtcpb.SignalResp_InviteInGroup:
		if payload.InviteInGroup.GetRoomId() == "" {
			return nil, errorcode.ErrInternal.WithMsg("empty rtc response")
		}
		return &msgpb.CallSignal{
			Op: msgpb.CallOp_CALL_INVITE, CallId: payload.InviteInGroup.GetRoomId(),
			Media: replyMedia(payload.InviteInGroup.GetMediaType(), in.GetMedia()), CallerId: userID, GroupId: payload.InviteInGroup.GetGroupId(),
			Url: payload.InviteInGroup.GetLiveUrl(), Token: payload.InviteInGroup.GetToken(),
			UserIds: payload.InviteInGroup.GetInviteeUserIds(),
		}, nil
	case *rtcpb.SignalResp_Accept:
		return &msgpb.CallSignal{
			Op: msgpb.CallOp_CALL_ACCEPT, CallId: payload.Accept.GetRoomId(),
			Url: payload.Accept.GetLiveUrl(), Token: payload.Accept.GetToken(), CalleeId: userID,
			Media: in.GetMedia(), GroupId: payload.Accept.GetGroupId(), UserIds: payload.Accept.GetUserIds(),
		}, nil
	case *rtcpb.SignalResp_Join:
		if payload.Join.GetRoomId() == "" {
			return nil, errorcode.ErrInternal.WithMsg("empty rtc response")
		}
		return &msgpb.CallSignal{
			Op: msgpb.CallOp_CALL_JOIN, CallId: payload.Join.GetRoomId(), CallerId: userID,
			Media: replyMedia(payload.Join.GetMediaType(), in.GetMedia()), GroupId: payload.Join.GetGroupId(),
			Url: payload.Join.GetLiveUrl(), Token: payload.Join.GetToken(), UserIds: payload.Join.GetUserIds(),
		}, nil
	case *rtcpb.SignalResp_GroupCall:
		return &msgpb.CallSignal{
			Op: msgpb.CallOp_CALL_GROUP, CallId: payload.GroupCall.GetRoomId(),
			GroupId: payload.GroupCall.GetGroupId(), CallerId: payload.GroupCall.GetCallerUserId(),
			UserIds: payload.GroupCall.GetUserIds(), Media: replyMedia(payload.GroupCall.GetMediaType(), msgpb.CallMedia_CALL_AUDIO),
		}, nil
	case *rtcpb.SignalResp_Cancel:
		return &msgpb.CallSignal{Op: msgpb.CallOp_CALL_CANCEL, CallId: in.GetCallId()}, nil
	case *rtcpb.SignalResp_Reject:
		return &msgpb.CallSignal{Op: msgpb.CallOp_CALL_REJECT, CallId: in.GetCallId()}, nil
	case *rtcpb.SignalResp_HangUp:
		return &msgpb.CallSignal{Op: msgpb.CallOp_CALL_HANGUP, CallId: in.GetCallId()}, nil
	default:
		return nil, errorcode.ErrInternal.WithMsg("empty rtc response")
	}
}
