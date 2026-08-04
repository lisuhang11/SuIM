// Package handler 将领域 CallService 适配到 gRPC 传输层。
package handler

import (
	"context"

	apperrors "rtc/internal/errors"
	"rtc/internal/logger"
	"rtc/internal/types"
	"rtc/internal/types/interfaces"

	pb "SuIM/proto/rtcpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type rtcHandler struct {
	pb.UnimplementedRtcServiceServer
	svc       interfaces.CallService
	liveKitURL string
}

// NewRtcHandler 创建 gRPC RtcServiceServer。
func NewRtcHandler(svc interfaces.CallService, liveKitURL string) pb.RtcServiceServer {
	return &rtcHandler{svc: svc, liveKitURL: liveKitURL}
}

func appErrorToStatus(err error) error {
	ae := apperrors.GetAppError(err)
	if ae == nil {
		return status.Error(codes.Internal, err.Error())
	}
	var code codes.Code
	switch ae.Code {
	case apperrors.CodeValidation:
		code = codes.InvalidArgument
	case apperrors.CodeNotFriend, apperrors.CodeNotParticipant:
		code = codes.PermissionDenied
	case apperrors.CodeNotFound:
		code = codes.NotFound
	case apperrors.CodeUnavailable:
		code = codes.Unavailable
	case apperrors.CodeBusy, apperrors.CodeInvalidState:
		code = codes.FailedPrecondition
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}

func toProtoCall(call *types.Call) *pb.CallInfo {
	if call == nil {
		return nil
	}
	return &pb.CallInfo{
		CallId:         call.CallID,
		ConversationId: call.ConversationID,
		CallerId:       call.CallerID,
		CalleeId:       call.CalleeID,
		MediaType:      call.MediaType,
		Status:         call.Status,
		EndReason:      call.EndReason,
		RoomName:       call.RoomName,
		StartedAt:      call.StartedAt,
		AnsweredAt:     call.AnsweredAt,
		EndedAt:        call.EndedAt,
		DurationSec:    call.DurationSec,
	}
}

func (h *rtcHandler) Invite(ctx context.Context, req *pb.InviteReq) (*pb.InviteResp, error) {
	call, token, err := h.svc.Invite(ctx, req.GetCallerId(), req.GetCalleeId(), req.GetMediaType())
	if err != nil {
		logger.Error(ctx, "invite call failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.InviteResp{
		Call:       toProtoCall(call),
		Token:      token,
		LivekitUrl: h.liveKitURL,
	}, nil
}

func (h *rtcHandler) Accept(ctx context.Context, req *pb.AcceptReq) (*pb.AcceptResp, error) {
	call, token, err := h.svc.Accept(ctx, req.GetUserId(), req.GetCallId())
	if err != nil {
		logger.Error(ctx, "accept call failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.AcceptResp{
		Call:       toProtoCall(call),
		Token:      token,
		LivekitUrl: h.liveKitURL,
	}, nil
}

func (h *rtcHandler) Reject(ctx context.Context, req *pb.RejectReq) (*pb.RejectResp, error) {
	call, err := h.svc.Reject(ctx, req.GetUserId(), req.GetCallId())
	if err != nil {
		logger.Error(ctx, "reject call failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.RejectResp{Call: toProtoCall(call)}, nil
}

func (h *rtcHandler) Cancel(ctx context.Context, req *pb.CancelReq) (*pb.CancelResp, error) {
	call, err := h.svc.Cancel(ctx, req.GetUserId(), req.GetCallId())
	if err != nil {
		logger.Error(ctx, "cancel call failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.CancelResp{Call: toProtoCall(call)}, nil
}

func (h *rtcHandler) Hangup(ctx context.Context, req *pb.HangupReq) (*pb.HangupResp, error) {
	call, err := h.svc.Hangup(ctx, req.GetUserId(), req.GetCallId())
	if err != nil {
		logger.Error(ctx, "hangup call failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.HangupResp{Call: toProtoCall(call)}, nil
}

func (h *rtcHandler) GetCall(ctx context.Context, req *pb.GetCallReq) (*pb.GetCallResp, error) {
	call, err := h.svc.GetCall(ctx, req.GetUserId(), req.GetCallId())
	if err != nil {
		logger.Error(ctx, "get call failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.GetCallResp{Call: toProtoCall(call)}, nil
}

func (h *rtcHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenReq) (*pb.RefreshTokenResp, error) {
	token, roomName, err := h.svc.RefreshToken(ctx, req.GetUserId(), req.GetCallId())
	if err != nil {
		logger.Error(ctx, "refresh token failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.RefreshTokenResp{
		Token:      token,
		RoomName:   roomName,
		LivekitUrl: h.liveKitURL,
	}, nil
}
