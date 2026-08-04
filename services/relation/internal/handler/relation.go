// Package handler 将领域 RelationService 适配为 gRPC 传输层处理逻辑。
package handler

import (
	"context"

	apperrors "relation/internal/errors"
	"relation/internal/logger"
	"relation/internal/middleware"
	"relation/internal/types"
	"relation/internal/types/interfaces"

	pb "SuIM/proto/relationpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ProfileMap is nickname/avatar keyed by userID (injected from user service client).
type ProfileMap = map[string]struct {
	Nickname  string
	AvatarURL string
}

type profileLookup interface {
	LookupProfiles(ctx context.Context, userIDs []string) (ProfileMap, error)
}

// relationHandler 实现 pb.RelationServiceServer，将请求委托给领域 RelationService。
type relationHandler struct {
	pb.UnimplementedRelationServiceServer
	svc      interfaces.RelationService
	profiles profileLookup
}

// NewRelationHandler 创建绑定到指定领域服务的 gRPC RelationServiceServer。
func NewRelationHandler(svc interfaces.RelationService, profiles profileLookup) pb.RelationServiceServer {
	return &relationHandler{svc: svc, profiles: profiles}
}

func authenticatedUserID(ctx context.Context) (string, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authenticated user is missing")
	}
	return userID, nil
}

func requireParticipant(userID, user1, user2 string) error {
	if userID != user1 && userID != user2 {
		return status.Error(codes.PermissionDenied, "caller must be one of the queried users")
	}
	return nil
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
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.SendFriendRequest(ctx, userID, req.ToUserId, req.Message); err != nil {
		logger.Error(ctx, "send friend request failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SendFriendRequestResp{}, nil
}

// RespondFriendApply 响应好友请求（接受或拒绝）。
func (h *relationHandler) RespondFriendApply(ctx context.Context, req *pb.RespondFriendApplyReq) (*pb.RespondFriendApplyResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	handleResult := types.FriendRequestHandleResult(req.HandleResult)
	// ToUserId / 操作者一律取 JWT，防止代他人处理申请。
	if err := h.svc.RespondFriendApply(ctx, req.FromUserId, userID, userID, handleResult, req.HandleMsg); err != nil {
		logger.Error(ctx, "respond friend apply failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.RespondFriendApplyResp{}, nil
}

// GetIncomingApplyTo 获取收到的好友请求（分页，支持状态筛选）。
func (h *relationHandler) GetIncomingApplyTo(ctx context.Context, req *pb.GetIncomingApplyToReq) (*pb.GetIncomingApplyToResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	requests, total, err := h.svc.GetIncomingApplyTo(ctx, userID, req.HandleResults, int(req.Offset), int(req.Limit))
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
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	requests, total, err := h.svc.GetOutgoingApplyFrom(ctx, userID, req.HandleResults, int(req.Offset), int(req.Limit))
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
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	count, err := h.svc.GetUnhandledApplyCount(ctx, userID)
	if err != nil {
		logger.Error(ctx, "get unhandled apply count failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.GetUnhandledApplyCountResp{Count: count}, nil
}

// DeleteFriend 删除好友关系。
func (h *relationHandler) DeleteFriend(ctx context.Context, req *pb.DeleteFriendReq) (*pb.DeleteFriendResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteFriend(ctx, userID, req.FriendId); err != nil {
		logger.Error(ctx, "delete friend failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.DeleteFriendResp{}, nil
}

// GetFriends 获取好友列表。
func (h *relationHandler) GetFriends(ctx context.Context, req *pb.GetFriendsReq) (*pb.GetFriendsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	friends, total, err := h.svc.GetFriends(ctx, userID, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	out := h.toFriendInfos(ctx, friends)
	return &pb.GetFriendsResp{Friends: out, Total: int32(total)}, nil
}

// UpdateFriend 更新好友备注 / 置顶。
func (h *relationHandler) UpdateFriend(ctx context.Context, req *pb.UpdateFriendReq) (*pb.UpdateFriendResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.UpdateFriend(ctx, userID, req.FriendUserId, req.Remark, req.IsPinned); err != nil {
		logger.Error(ctx, "update friend failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.UpdateFriendResp{}, nil
}

// GetIncrementalFriends 好友列表增量同步。
func (h *relationHandler) GetIncrementalFriends(ctx context.Context, req *pb.GetIncrementalFriendsReq) (*pb.GetIncrementalFriendsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.UserId != "" && req.UserId != userID {
		return nil, status.Error(codes.PermissionDenied, "user_id must be the authenticated user")
	}
	res, err := h.svc.GetIncrementalFriends(ctx, userID, req.VersionId, req.Version)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetIncrementalFriendsResp{
		Version:     res.Version,
		VersionId:   res.VersionID,
		Full:        res.Full,
		Delete:      res.Delete,
		Insert:      h.toFriendInfos(ctx, res.Insert),
		Update:      h.toFriendInfos(ctx, res.Update),
		SortVersion: res.SortVersion,
	}, nil
}

// GetFullFriendUserIDs 完整好友 ID 列表。
func (h *relationHandler) GetFullFriendUserIDs(ctx context.Context, req *pb.GetFullFriendUserIDsReq) (*pb.GetFullFriendUserIDsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.UserId != "" && req.UserId != userID {
		return nil, status.Error(codes.PermissionDenied, "user_id must be the authenticated user")
	}
	ids, err := h.svc.GetFullFriendUserIDs(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetFullFriendUserIDsResp{UserIds: ids}, nil
}

// NotificationUserInfoUpdate 用户资料变更通知（调用者须为变更用户本人）。
func (h *relationHandler) NotificationUserInfoUpdate(ctx context.Context, req *pb.NotificationUserInfoUpdateReq) (*pb.NotificationUserInfoUpdateResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.UserId == "" {
		req.UserId = userID
	}
	if req.UserId != userID {
		return nil, status.Error(codes.PermissionDenied, "can only notify for self profile update")
	}
	if err := h.svc.NotificationUserInfoUpdate(ctx, req.UserId); err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.NotificationUserInfoUpdateResp{}, nil
}

func (h *relationHandler) toFriendInfos(ctx context.Context, friends []*types.Friend) []*pb.FriendInfo {
	out := make([]*pb.FriendInfo, 0, len(friends))
	ids := make([]string, 0, len(friends))
	for _, f := range friends {
		if f == nil {
			continue
		}
		ids = append(ids, f.FriendUserID)
	}
	var profiles ProfileMap
	if h.profiles != nil && len(ids) > 0 {
		if m, err := h.profiles.LookupProfiles(ctx, ids); err == nil {
			profiles = m
		}
	}
	for _, f := range friends {
		if f == nil {
			continue
		}
		info := &pb.FriendInfo{
			FriendUserId: f.FriendUserID,
			Remark:       f.Remark,
			IsPinned:     f.IsPinned,
			CreateTime:   f.CreateTime.UnixMilli(),
		}
		if p, ok := profiles[f.FriendUserID]; ok {
			info.Nickname = p.Nickname
			info.AvatarUrl = p.AvatarURL
		}
		out = append(out, info)
	}
	return out
}

// BlockUser 拉黑用户。
func (h *relationHandler) BlockUser(ctx context.Context, req *pb.BlockUserReq) (*pb.BlockUserResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.BlockUser(ctx, userID, req.BlockedUserId); err != nil {
		logger.Error(ctx, "block user failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.BlockUserResp{}, nil
}

// UnblockUser 取消拉黑用户。
func (h *relationHandler) UnblockUser(ctx context.Context, req *pb.UnblockUserReq) (*pb.UnblockUserResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.UnblockUser(ctx, userID, req.BlockedUserId); err != nil {
		logger.Error(ctx, "unblock user failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.UnblockUserResp{}, nil
}

// GetBlockedUsers 获取已拉黑的用户列表。
func (h *relationHandler) GetBlockedUsers(ctx context.Context, req *pb.GetBlockedUsersReq) (*pb.GetBlockedUsersResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	blocks, total, err := h.svc.GetBlockedUsers(ctx, userID, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	out := make([]*pb.BlackInfo, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, &pb.BlackInfo{
			BlockedUserId:  b.BlockUserID,
			CreateTime:     b.CreateTime.UnixMilli(),
			AddSource:      int32(b.AddSource),
			OperatorUserId: b.OperatorUserID,
			Ex:             b.Ex,
		})
	}
	return &pb.GetBlockedUsersResp{Blacks: out, Total: int32(total)}, nil
}

// IsFriend 判断两个用户之间的双向好友关系。
func (h *relationHandler) IsFriend(ctx context.Context, req *pb.IsFriendReq) (*pb.IsFriendResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireParticipant(userID, req.User1, req.User2); err != nil {
		return nil, err
	}
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
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := requireParticipant(userID, req.User1, req.User2); err != nil {
		return nil, err
	}
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
