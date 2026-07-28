// Package handler 实现 msgGateway gRPC 服务。
package handler

import (
	"context"
	"log/slog"

	pb "SuIM/proto/msggatewaypb"
	"msggateway/internal/ws"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// MsgGatewayHandler 实现 pb.MsgGatewayServer。
type MsgGatewayHandler struct {
	pb.UnimplementedMsgGatewayServer
	wsServer *ws.Server
}

// NewMsgGatewayHandler 创建 gRPC handler。
func NewMsgGatewayHandler(wsServer *ws.Server) *MsgGatewayHandler {
	return &MsgGatewayHandler{wsServer: wsServer}
}

// OnlinePushMsg 向单个在线用户的全部平台推送消息。
func (h *MsgGatewayHandler) OnlinePushMsg(ctx context.Context, req *pb.OnlinePushMsgReq) (*pb.OnlinePushMsgResp, error) {
	if req.PushToUserId == "" {
		return nil, status.Error(codes.InvalidArgument, "push_to_user_id is required")
	}
	if req.MsgData == nil {
		return nil, status.Error(codes.InvalidArgument, "msg_data is required")
	}

	// 获取在线用户的全部连接及其平台。
	_, platforms := h.wsServer.ConnMgr().GetOnlineStatus(req.PushToUserId)

	// protobuf → JSON，用于 WebSocket 推送。
	msgJSON, err := protojson.Marshal(req.MsgData)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal msg_data: %v", err)
	}

	// 按平台推送并收集结果。
	var results []*pb.SingleMsgToUserPlatform
	for _, p := range platforms {
		count := h.wsServer.PushToUserPlatform(req.PushToUserId, p.PlatformID, msgJSON)
		result := &pb.SingleMsgToUserPlatform{
			RecvId:         req.PushToUserId,
			RecvPlatformId: p.PlatformID,
		}
		if count > 0 {
			result.ResultCode = 0 // 成功。
		} else {
			result.ResultCode = 1 // 推送失败（可能已离线）。
		}
		results = append(results, result)
	}

	slog.Info("online push completed",
		"user_id", req.PushToUserId,
		"platforms_pushed", len(results),
	)

	return &pb.OnlinePushMsgResp{Resp: results}, nil
}

// GetUsersOnlineStatus 批量查询用户在线状态。
func (h *MsgGatewayHandler) GetUsersOnlineStatus(ctx context.Context, req *pb.GetUsersOnlineStatusReq) (*pb.GetUsersOnlineStatusResp, error) {
	if len(req.UserIds) == 0 {
		return &pb.GetUsersOnlineStatusResp{}, nil
	}

	var successResults []*pb.SuccessResult
	var failedResults []*pb.FailedDetail

	for _, userID := range req.UserIds {
		status, platforms := h.wsServer.ConnMgr().GetOnlineStatus(userID)

		if status == 0 {
			// 离线视为查询失败（用户不在线）。
			failedResults = append(failedResults, &pb.FailedDetail{UserId: userID})
			continue
		}

		var details []*pb.SuccessDetail
		for _, p := range platforms {
			isBackground := false
			// 多 token 时，第一个为前台，其余为后台。
			for i, token := range p.Tokens {
				isBg := isBackground
				if i > 0 {
					isBg = true
				}
				details = append(details, &pb.SuccessDetail{
					PlatformId:   p.PlatformID,
					ConnId:       "", // 使用 token 粒度的 conn 占位。
					IsBackground: isBg,
					Token:        token,
				})
			}
			_ = p.ConnIDs // 暂不暴露 connID 细节。
		}

		successResults = append(successResults, &pb.SuccessResult{
			UserId:                userID,
			Status:                status,
			DetailPlatformStatus: details,
		})
	}

	return &pb.GetUsersOnlineStatusResp{
		SuccessResult: successResults,
		FailedResult:  failedResults,
	}, nil
}

// OnlineBatchPushOneMsg 向多个在线用户推送同一条消息。
func (h *MsgGatewayHandler) OnlineBatchPushOneMsg(ctx context.Context, req *pb.OnlineBatchPushOneMsgReq) (*pb.OnlineBatchPushOneMsgResp, error) {
	if req.MsgData == nil {
		return nil, status.Error(codes.InvalidArgument, "msg_data is required")
	}

	msgJSON, err := protojson.Marshal(req.MsgData)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal msg_data: %v", err)
	}

	var results []*pb.SingleMsgToUserResults
	for _, userID := range req.PushToUserIds {
		count := h.wsServer.PushToUser(userID, msgJSON)

		// 构建该用户的平台级推送结果。
		var platResults []*pb.SingleMsgToUserPlatform
		_, platforms := h.wsServer.ConnMgr().GetOnlineStatus(userID)
		for _, p := range platforms {
			platResults = append(platResults, &pb.SingleMsgToUserPlatform{
				RecvId:         userID,
				RecvPlatformId: p.PlatformID,
				ResultCode:     0,
			})
		}
		// 若用户不在线，至少返回一个空结果标记。
		if len(platResults) == 0 {
			platResults = append(platResults, &pb.SingleMsgToUserPlatform{
				RecvId:     userID,
				ResultCode: 1,
			})
		}

		results = append(results, &pb.SingleMsgToUserResults{
			UserId:     userID,
			Resp:       platResults,
			OnlinePush: count > 0,
		})
	}

	slog.Info("batch push completed",
		"target_users", len(req.PushToUserIds),
		"pushed_count", len(results),
	)

	return &pb.OnlineBatchPushOneMsgResp{SinglePushResult: results}, nil
}

// SuperGroupOnlineBatchPushOneMsg 超群在线批量推送（与 OnlineBatchPushOneMsg 逻辑一致）。
func (h *MsgGatewayHandler) SuperGroupOnlineBatchPushOneMsg(ctx context.Context, req *pb.OnlineBatchPushOneMsgReq) (*pb.OnlineBatchPushOneMsgResp, error) {
	// 超群推送 = 向群成员批量推送，逻辑复用。
	return h.OnlineBatchPushOneMsg(ctx, req)
}

// KickUserOffline 踢用户下线。
func (h *MsgGatewayHandler) KickUserOffline(ctx context.Context, req *pb.KickUserOfflineReq) (*pb.KickUserOfflineResp, error) {
	if len(req.KickUserIdList) == 0 {
		return &pb.KickUserOfflineResp{}, nil
	}

	for _, userID := range req.KickUserIdList {
		kicked := h.wsServer.KickUser(userID, req.PlatformId)
		slog.Info("user kicked",
			"user_id", userID,
			"platform_id", req.PlatformId,
			"connections_closed", kicked,
		)
	}

	return &pb.KickUserOfflineResp{}, nil
}

// MultiTerminalLoginCheck 检查多端登录策略。
// 返回值通过 gRPC error 语义表示：nil error = 允许；PermissionDenied = 拒绝。
func (h *MsgGatewayHandler) MultiTerminalLoginCheck(ctx context.Context, req *pb.MultiTerminalLoginCheckReq) (*pb.MultiTerminalLoginCheckResp, error) {
	if req.UserId == "" {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	online, platforms := h.wsServer.ConnMgr().GetOnlineStatus(req.UserId)
	if online == 0 {
		return &pb.MultiTerminalLoginCheckResp{}, nil
	}

	// 检查同平台是否已有连接。
	for _, p := range platforms {
		if p.PlatformID == req.PlatformId && len(p.Tokens) > 0 {
			// 同平台已有 token 在线，允许（多端）。
			slog.Debug("multi-terminal login check: same platform has existing session",
				"user_id", req.UserId,
				"platform_id", req.PlatformId,
				"existing_tokens", len(p.Tokens),
			)
		}
	}

	slog.Debug("multi-terminal login check passed",
		"user_id", req.UserId,
		"platform_id", req.PlatformId,
		"online_platforms", len(platforms),
	)

	return &pb.MultiTerminalLoginCheckResp{}, nil
}

// 确保实现接口。
var _ pb.MsgGatewayServer = (*MsgGatewayHandler)(nil)
