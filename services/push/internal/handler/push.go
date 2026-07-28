// Package handler 将领域 PushService 适配到 gRPC 传输层。
package handler

import (
	"context"

	apperrors "push/internal/errors"
	"push/internal/logger"
	"push/internal/types/interfaces"

	pb "SuIM/proto/pushpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type pushHandler struct {
	pb.UnimplementedPushMsgServiceServer
	svc interfaces.PushService
}

// NewPushHandler 创建 gRPC PushMsgServiceServer，注入领域服务。
func NewPushHandler(svc interfaces.PushService) pb.PushMsgServiceServer {
	return &pushHandler{svc: svc}
}

// --------------- 错误转换 ---------------

// appErrorToStatus 将 AppError 映射为 gRPC status 错误。
func appErrorToStatus(err error) error {
	ae := apperrors.GetAppError(err)
	if ae == nil {
		return status.Error(codes.Internal, err.Error())
	}
	var code codes.Code
	switch ae.Code {
	case apperrors.CodeValidation:
		code = codes.InvalidArgument
	case apperrors.CodeTokenNotFound, apperrors.CodeNotFound:
		code = codes.NotFound
	case apperrors.CodePushFailed:
		code = codes.Internal
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}

// --------------- gRPC 接口实现 ---------------

// PushMsg 向指定用户推送消息通知。
func (h *pushHandler) PushMsg(ctx context.Context, req *pb.PushMsgReq) (*pb.PushMsgResp, error) {
	if err := h.svc.PushMsg(ctx, req); err != nil {
		logger.Error(ctx, "push message failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.PushMsgResp{}, nil
}

// SetUserPushToken 为用户注册或更新指定平台的设备推送令牌。
func (h *pushHandler) SetUserPushToken(ctx context.Context, req *pb.SetUserPushTokenReq) (*pb.SetUserPushTokenResp, error) {
	if err := h.svc.SetUserPushToken(ctx, req.UserId, req.PlatformId, req.Token); err != nil {
		logger.Error(ctx, "set push token failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SetUserPushTokenResp{}, nil
}

// DelUserPushToken 删除用户指定平台的设备推送令牌。
func (h *pushHandler) DelUserPushToken(ctx context.Context, req *pb.DelUserPushTokenReq) (*pb.DelUserPushTokenResp, error) {
	if err := h.svc.DelUserPushToken(ctx, req.UserId, req.PlatformId); err != nil {
		logger.Error(ctx, "delete push token failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.DelUserPushTokenResp{}, nil
}
