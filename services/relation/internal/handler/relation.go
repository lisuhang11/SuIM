// Package handler 将领域 RelationService 适配为 gRPC 传输层处理逻辑。
package handler

import (
	"context"

	apperrors "relation/internal/errors"
	"relation/internal/logger"
	"relation/internal/types"
	"relation/internal/types/interfaces"

	pb "SuIM/proto/relationpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// relationHandler 实现 pb.RelationServiceServer，将请求委托给领域 RelationService。
type relationHandler struct {
	pb.UnimplementedRelationServiceServer
	svc interfaces.RelationService
}

// NewRelationHandler 创建绑定到指定领域服务的 gRPC RelationServiceServer。
func NewRelationHandler(svc interfaces.RelationService) pb.RelationServiceServer {
	return &relationHandler{svc: svc}
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
	case apperrors.CodeValidation, apperrors.CodeCannotFriendSelf:
		code = codes.InvalidArgument
	case apperrors.CodeRelationNotFound, apperrors.CodeFriendRequestNotFound,
		apperrors.CodeNotBlocked:
		code = codes.NotFound
	case apperrors.CodeAlreadyFriends, apperrors.CodeAlreadyBlocked,
		apperrors.CodeAlreadyRequested, apperrors.CodeRequestAlreadyProcessed:
		code = codes.AlreadyExists
	case apperrors.CodeNotAuthorized:
		code = codes.PermissionDenied
	default:
		code = codes.Internal
	}

	return status.Error(code, ae.Message)
}

// --------------- 类型转换辅助函数 ---------------

// friendRequestToProto 将领域 FriendRequest 转换为 proto FriendRequestInfo。
func friendRequestToProto(req *types.FriendRequest) *pb.FriendRequestInfo {
	if req == nil {
		return nil
	}
	var handleTime int64
	if req.HandleTime != nil {
		handleTime = req.HandleTime.UnixMilli()
	}
	return &pb.FriendRequestInfo{
		FromUserId: req.FromUserID,
		ToUserId:   req.ToUserID,
		Message:    req.ReqMsg,
		HandleMsg:  req.HandleMsg,
		Status:     int32(req.HandleResult),
		CreatedAt:  req.CreateTime.UnixMilli(),
		UpdatedAt:  handleTime,
	}
}

// --------------- RPC 实现 ---------------

// SendFriendRequest 发送好友请求。
func (h *relationHandler) SendFriendRequest(ctx context.Context, req *pb.SendFriendRequestReq) (*pb.SendFriendRequestResp, error) {
	if err := h.svc.SendFriendRequest(ctx, req.FromUserId, req.ToUserId, req.Message); err != nil {
		logger.Error(ctx, "send friend request failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SendFriendRequestResp{}, nil
}

// RespondFriendApply 响应好友请求（接受或拒绝）。
func (h *relationHandler) RespondFriendApply(ctx context.Context, req *pb.RespondFriendApplyReq) (*pb.RespondFriendApplyResp, error) {
	handleResult := types.FriendRequestHandleResult(req.HandleResult)
	if err := h.svc.RespondFriendApply(ctx, req.FromUserId, req.ToUserId, req.ToUserId, handleResult, req.HandleMsg); err != nil {
		logger.Error(ctx, "respond friend apply failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.RespondFriendApplyResp{}, nil
}

// GetIncomingApplyTo 获取收到的好友请求（分页，支持状态筛选）。
func (h *relationHandler) GetIncomingApplyTo(ctx context.Context, req *pb.GetIncomingApplyToReq) (*pb.GetIncomingApplyToResp, error) {
	requests, total, err := h.svc.GetIncomingApplyTo(ctx, req.UserId, req.HandleResults, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}

	pbReqs := make([]*pb.FriendRequestInfo, 0, len(requests))
	for _, r := range requests {
		pbReqs = append(pbReqs, friendRequestToProto(r))
	}
	return &pb.GetIncomingApplyToResp{Requests: pbReqs, Total: int32(total)}, nil
}

// GetOutgoingApplyFrom 获取发出的好友请求（分页，支持状态筛选）。
func (h *relationHandler) GetOutgoingApplyFrom(ctx context.Context, req *pb.GetOutgoingApplyFromReq) (*pb.GetOutgoingApplyFromResp, error) {
	requests, total, err := h.svc.GetOutgoingApplyFrom(ctx, req.UserId, req.HandleResults, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}

	pbReqs := make([]*pb.FriendRequestInfo, 0, len(requests))
	for _, r := range requests {
		pbReqs = append(pbReqs, friendRequestToProto(r))
	}
	return &pb.GetOutgoingApplyFromResp{Requests: pbReqs, Total: int32(total)}, nil
}

// GetUnhandledApplyCount 获取未处理的好友请求数量。
func (h *relationHandler) GetUnhandledApplyCount(ctx context.Context, req *pb.GetUnhandledApplyCountReq) (*pb.GetUnhandledApplyCountResp, error) {
	count, err := h.svc.GetUnhandledApplyCount(ctx, req.UserId)
	if err != nil {
		logger.Error(ctx, "get unhandled apply count failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.GetUnhandledApplyCountResp{Count: count}, nil
}

// DeleteFriend 删除好友关系。
func (h *relationHandler) DeleteFriend(ctx context.Context, req *pb.DeleteFriendReq) (*pb.DeleteFriendResp, error) {
	if err := h.svc.DeleteFriend(ctx, req.UserId, req.FriendId); err != nil {
		logger.Error(ctx, "delete friend failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.DeleteFriendResp{}, nil
}

// GetFriends 获取好友列表。
func (h *relationHandler) GetFriends(ctx context.Context, req *pb.GetFriendsReq) (*pb.GetFriendsResp, error) {
	friendIDs, total, err := h.svc.GetFriends(ctx, req.UserId, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetFriendsResp{FriendIds: friendIDs, Total: int32(total)}, nil
}

// BlockUser 拉黑用户。
func (h *relationHandler) BlockUser(ctx context.Context, req *pb.BlockUserReq) (*pb.BlockUserResp, error) {
	if err := h.svc.BlockUser(ctx, req.UserId, req.BlockedUserId); err != nil {
		logger.Error(ctx, "block user failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.BlockUserResp{}, nil
}

// UnblockUser 取消拉黑用户。
func (h *relationHandler) UnblockUser(ctx context.Context, req *pb.UnblockUserReq) (*pb.UnblockUserResp, error) {
	if err := h.svc.UnblockUser(ctx, req.UserId, req.BlockedUserId); err != nil {
		logger.Error(ctx, "unblock user failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.UnblockUserResp{}, nil
}

// GetBlockedUsers 获取已拉黑的用户列表。
func (h *relationHandler) GetBlockedUsers(ctx context.Context, req *pb.GetBlockedUsersReq) (*pb.GetBlockedUsersResp, error) {
	blockedIDs, total, err := h.svc.GetBlockedUsers(ctx, req.UserId, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetBlockedUsersResp{BlockedUserIds: blockedIDs, Total: int32(total)}, nil
}

// IsFriend 判断两个用户之间的双向好友关系。
func (h *relationHandler) IsFriend(ctx context.Context, req *pb.IsFriendReq) (*pb.IsFriendResp, error) {
	inUser1Friends, inUser2Friends, err := h.svc.IsFriend(ctx, req.User1, req.User2)
	if err != nil {
		logger.Error(ctx, "is friend failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.IsFriendResp{
		InUser1Friends: inUser1Friends,
		InUser2Friends: inUser2Friends,
	}, nil
}

// IsBlack 判断两个用户之间的双向拉黑关系。
func (h *relationHandler) IsBlack(ctx context.Context, req *pb.IsBlackReq) (*pb.IsBlackResp, error) {
	inUser1Blacklist, inUser2Blacklist, err := h.svc.IsBlack(ctx, req.User1, req.User2)
	if err != nil {
		logger.Error(ctx, "is black failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.IsBlackResp{
		InUser1Blacklist: inUser1Blacklist,
		InUser2Blacklist: inUser2Blacklist,
	}, nil
}
