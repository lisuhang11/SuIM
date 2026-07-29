// Package handler 将领域 GroupService 适配为 gRPC 传输层处理逻辑。
package handler

import (
	"context"

	apperrors "group/internal/errors"
	"group/internal/logger"
	"group/internal/middleware"
	"group/internal/types"
	"group/internal/types/interfaces"

	filePB "SuIM/proto/filepb"
	pb "SuIM/proto/grouppb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func authenticatedUserID(ctx context.Context) (string, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authenticated user is missing")
	}
	return userID, nil
}

// groupHandler 实现 pb.GroupServiceServer，将请求委托给领域 GroupService。
type groupHandler struct {
	pb.UnimplementedGroupServiceServer
	svc   interfaces.GroupService
	files filePB.FileServiceClient
}

// NewGroupHandler 创建绑定到指定领域服务的 gRPC GroupServiceServer。
func NewGroupHandler(svc interfaces.GroupService, files filePB.FileServiceClient) pb.GroupServiceServer {
	return &groupHandler{svc: svc, files: files}
}

func fileContext(ctx context.Context) context.Context {
	md, _ := metadata.FromIncomingContext(ctx)
	return metadata.NewOutgoingContext(ctx, md.Copy())
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
	case apperrors.CodeValidation, apperrors.CodeCannotQuitAsOwner,
		apperrors.CodeCannotKickRole, apperrors.CodeUserNotExist,
		apperrors.CodeAlreadyMember, apperrors.CodeAlreadyRequested,
		apperrors.CodeRequestAlreadyHandled:
		code = codes.InvalidArgument
	case apperrors.CodeGroupNotFound, apperrors.CodeMemberNotFound, apperrors.CodeRequestNotFound:
		code = codes.NotFound
	case apperrors.CodeNotGroupOwner, apperrors.CodeNotAuthorized:
		code = codes.PermissionDenied
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}

// --------------- 类型转换辅助函数 ---------------

// groupToProto 将领域 Group 模型转换为 proto GroupInfo。
func groupToProto(g *types.Group) *pb.GroupInfo {
	if g == nil {
		return nil
	}
	return &pb.GroupInfo{
		GroupId:                g.GroupID,
		GroupName:              g.GroupName,
		Notification:           g.Notification,
		Introduction:           g.Introduction,
		FaceUrl:                g.FaceURL,
		CreateTime:             g.CreateTime.UnixMilli(),
		Ex:                     g.Ex,
		Status:                 int32(g.Status),
		CreatorUserId:          g.CreatorUserID,
		GroupType:              int32(g.GroupType),
		NeedVerification:       int32(g.NeedVerification),
		LookMemberInfo:         int32(g.LookMemberInfo),
		ApplyMemberFriend:      int32(g.ApplyMemberFriend),
		NotificationUpdateTime: g.NotificationUpdateTime.UnixMilli(),
		NotificationUserId:     g.NotificationUserID,
	}
}

// memberToProto 将领域 GroupMember 模型转换为 proto GroupMemberFullInfo。
func memberToProto(m *types.GroupMember) *pb.GroupMemberFullInfo {
	if m == nil {
		return nil
	}
	var muteEnd int64
	if m.MuteEndTime != nil {
		muteEnd = m.MuteEndTime.UnixMilli()
	}
	return &pb.GroupMemberFullInfo{
		GroupId:        m.GroupID,
		UserId:         m.UserID,
		Nickname:       m.Nickname,
		FaceUrl:        m.FaceURL,
		RoleLevel:      int32(m.RoleLevel),
		JoinTime:       m.JoinTime.UnixMilli(),
		JoinSource:     int32(m.JoinSource),
		InviterUserId:  m.InviterUserID,
		OperatorUserId: m.OperatorUserID,
		MuteEndTime:    muteEnd,
		Ex:             m.Ex,
	}
}

// requestToProto 将领域 GroupRequest 模型转换为 proto GroupRequestInfo。
func requestToProto(req *types.GroupRequest) *pb.GroupRequestInfo {
	if req == nil {
		return nil
	}
	var handledTime int64
	if req.HandledTime != nil {
		handledTime = req.HandledTime.UnixMilli()
	}
	return &pb.GroupRequestInfo{
		UserId:        req.UserID,
		GroupId:       req.GroupID,
		HandleResult:  int32(req.HandleResult),
		ReqMsg:        req.ReqMsg,
		HandledMsg:    req.HandledMsg,
		ReqTime:       req.ReqTime.UnixMilli(),
		HandleUserId:  req.HandleUserID,
		HandledTime:   handledTime,
		JoinSource:    int32(req.JoinSource),
		InviterUserId: req.InviterUserID,
		Ex:            req.Ex,
	}
}

// --------------- RPC 实现 ---------------

// CreateGroup 创建群组。
func (h *groupHandler) CreateGroup(ctx context.Context, req *pb.CreateGroupReq) (*pb.CreateGroupResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	groupID, g, err := h.svc.CreateGroup(ctx, &types.CreateGroupInput{
		CreatorUserID:     userID,
		GroupName:         req.GroupName,
		Notification:      req.Notification,
		Introduction:      req.Introduction,
		FaceURL:           req.FaceUrl,
		GroupType:         int(req.GroupType),
		NeedVerification:  int(req.NeedVerification),
		LookMemberInfo:    int(req.LookMemberInfo),
		ApplyMemberFriend: int(req.ApplyMemberFriend),
		MemberIDs:         req.MemberIds,
		Ex:                req.Ex,
	})
	if err != nil {
		logger.Error(ctx, "create group failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.CreateGroupResp{Success: true, Message: "group created", GroupId: groupID, Group: groupToProto(g)}, nil
}

// DismissGroup 解散群组。
func (h *groupHandler) DismissGroup(ctx context.Context, req *pb.DismissGroupReq) (*pb.DismissGroupResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.DismissGroup(ctx, req.GroupId, userID); err != nil {
		logger.Error(ctx, "dismiss group failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.DismissGroupResp{Success: true, Message: "group dismissed"}, nil
}

// TransferGroupOwner 转让群主。
func (h *groupHandler) TransferGroupOwner(ctx context.Context, req *pb.TransferGroupOwnerReq) (*pb.TransferGroupOwnerResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.TransferGroupOwner(ctx, req.GroupId, userID, req.NewOwnerUserId); err != nil {
		logger.Error(ctx, "transfer owner failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.TransferGroupOwnerResp{Success: true, Message: "owner transferred"}, nil
}

// UpdateGroupInfo 更新群组信息。
func (h *groupHandler) UpdateGroupInfo(ctx context.Context, req *pb.UpdateGroupInfoReq) (*pb.UpdateGroupInfoResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	in := &types.UpdateGroupInfoInput{
		GroupID:      req.GroupId,
		OpUserID:     userID,
		GroupName:    req.GroupName,
		Notification: req.Notification,
		Introduction: req.Introduction,
		FaceURL:      req.FaceUrl,
		Ex:           req.Ex,
	}
	if req.NeedVerification != nil {
		v := int(*req.NeedVerification)
		in.NeedVerification = &v
	}
	if req.LookMemberInfo != nil {
		v := int(*req.LookMemberInfo)
		in.LookMemberInfo = &v
	}
	if req.ApplyMemberFriend != nil {
		v := int(*req.ApplyMemberFriend)
		in.ApplyMemberFriend = &v
	}
	g, err := h.svc.UpdateGroupInfo(ctx, in)
	if err != nil {
		logger.Error(ctx, "update group info failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.UpdateGroupInfoResp{Success: true, Message: "group updated", Group: groupToProto(g)}, nil
}

func (h *groupHandler) InitiateAvatarUpload(ctx context.Context, req *pb.InitiateGroupAvatarUploadReq) (*pb.InitiateGroupAvatarUploadResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.CanManageGroup(ctx, req.GroupId, userID); err != nil {
		return nil, appErrorToStatus(err)
	}
	upload, err := h.files.InitiateUpload(fileContext(ctx), &filePB.InitiateUploadReq{UserId: userID, Name: req.Name, ContentType: req.ContentType, Size: req.Size, Sha256: req.Sha256, Purpose: "avatar"})
	if err != nil {
		return nil, err
	}
	return &pb.InitiateGroupAvatarUploadResp{Upload: upload}, nil
}

func (h *groupHandler) CompleteAvatarUpload(ctx context.Context, req *pb.CompleteGroupAvatarUploadReq) (*pb.CompleteGroupAvatarUploadResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.CanManageGroup(ctx, req.GroupId, userID); err != nil {
		return nil, appErrorToStatus(err)
	}
	completed, err := h.files.CompleteUpload(fileContext(ctx), &filePB.CompleteUploadReq{UserId: userID, FileId: req.FileId})
	if err != nil {
		return nil, err
	}
	current, err := h.svc.GetGroup(ctx, req.GroupId)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	oldURL := current.FaceURL
	avatarURL := "/api/v1/files/" + req.FileId + "/avatar"
	updated, err := h.svc.UpdateGroupInfo(ctx, &types.UpdateGroupInfoInput{GroupID: req.GroupId, OpUserID: userID, FaceURL: &avatarURL})
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	activated, err := h.files.ActivateAvatar(fileContext(ctx), &filePB.ActivateAvatarReq{UserId: userID, FileId: req.FileId, TargetType: "group", TargetId: req.GroupId})
	if err != nil {
		_, _ = h.svc.UpdateGroupInfo(ctx, &types.UpdateGroupInfoInput{GroupID: req.GroupId, OpUserID: userID, FaceURL: &oldURL})
		return nil, err
	}
	if activated.File != nil {
		completed.File = activated.File
	}
	return &pb.CompleteGroupAvatarUploadResp{AvatarUrl: avatarURL, Group: groupToProto(updated), File: completed.File}, nil
}

// GetGroup 获取群组信息。
func (h *groupHandler) GetGroup(ctx context.Context, req *pb.GetGroupReq) (*pb.GetGroupResp, error) {
	g, err := h.svc.GetGroup(ctx, req.GroupId)
	if err != nil {
		logger.Error(ctx, "get group failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.GetGroupResp{Success: true, Message: "ok", Group: groupToProto(g)}, nil
}

// InviteUserToGroup 邀请用户加入群组。
func (h *groupHandler) InviteUserToGroup(ctx context.Context, req *pb.InviteUserToGroupReq) (*pb.InviteUserToGroupResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.InviteUserToGroup(ctx, &types.InviteInput{
		GroupID:  req.GroupId,
		OpUserID: userID,
		UserIDs:  req.UserIds,
		Reason:   req.Reason,
	}); err != nil {
		logger.Error(ctx, "invite members failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.InviteUserToGroupResp{Success: true, Message: "members invited"}, nil
}

// KickGroupMember 踢出群成员。
func (h *groupHandler) KickGroupMember(ctx context.Context, req *pb.KickGroupMemberReq) (*pb.KickGroupMemberResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.KickGroupMember(ctx, req.GroupId, userID, req.UserId); err != nil {
		logger.Error(ctx, "kick member failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.KickGroupMemberResp{Success: true, Message: "member kicked"}, nil
}

// QuitGroup 退出群组。
func (h *groupHandler) QuitGroup(ctx context.Context, req *pb.QuitGroupReq) (*pb.QuitGroupResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.QuitGroup(ctx, req.GroupId, userID); err != nil {
		logger.Error(ctx, "quit group failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.QuitGroupResp{Success: true, Message: "quit group"}, nil
}

// GetGroupMembers 获取群成员列表。
func (h *groupHandler) GetGroupMembers(ctx context.Context, req *pb.GetGroupMembersReq) (*pb.GetGroupMembersResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	members, total, err := h.svc.GetGroupMembers(ctx, req.GroupId, userID, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	pbMembers := make([]*pb.GroupMemberFullInfo, 0, len(members))
	for _, m := range members {
		pbMembers = append(pbMembers, memberToProto(m))
	}
	return &pb.GetGroupMembersResp{Members: pbMembers, Total: int32(total)}, nil
}

// GetJoinedGroups 获取用户已加入的群组列表。
func (h *groupHandler) GetJoinedGroups(ctx context.Context, req *pb.GetJoinedGroupsReq) (*pb.GetJoinedGroupsResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	groups, total, err := h.svc.GetJoinedGroups(ctx, userID, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	pbGroups := make([]*pb.GroupInfo, 0, len(groups))
	for _, g := range groups {
		pbGroups = append(pbGroups, groupToProto(g))
	}
	return &pb.GetJoinedGroupsResp{Groups: pbGroups, Total: int32(total)}, nil
}

// SetGroupMute 设置群全员禁言。
func (h *groupHandler) SetGroupMute(ctx context.Context, req *pb.SetGroupMuteReq) (*pb.SetGroupMuteResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.SetGroupMute(ctx, req.GroupId, userID, req.IsMuted); err != nil {
		logger.Error(ctx, "set group mute failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SetGroupMuteResp{Success: true, Message: "group mute updated"}, nil
}

// SetMemberMute 设置单个成员禁言。
func (h *groupHandler) SetMemberMute(ctx context.Context, req *pb.SetMemberMuteReq) (*pb.SetMemberMuteResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.SetMemberMute(ctx, req.GroupId, userID, req.UserId, req.MuteEndTime); err != nil {
		logger.Error(ctx, "set member mute failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SetMemberMuteResp{Success: true, Message: "member mute updated"}, nil
}

// ApplyToJoinGroup 申请加入群组。
func (h *groupHandler) ApplyToJoinGroup(ctx context.Context, req *pb.ApplyToJoinGroupReq) (*pb.ApplyToJoinGroupResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.ApplyToJoinGroup(ctx, &types.ApplyInput{
		GroupID:       req.GroupId,
		UserID:        userID,
		ReqMsg:        req.ReqMsg,
		JoinSource:    int(req.JoinSource),
		InviterUserID: "",
	}); err != nil {
		logger.Error(ctx, "apply to join failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.ApplyToJoinGroupResp{Success: true, Message: "application submitted"}, nil
}

// GetPendingApplications 获取待处理的入群申请。
func (h *groupHandler) GetPendingApplications(ctx context.Context, req *pb.GetPendingApplicationsReq) (*pb.GetPendingApplicationsResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	reqs, total, err := h.svc.GetPendingApplications(ctx, req.GroupId, userID, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	pbReqs := make([]*pb.GroupRequestInfo, 0, len(reqs))
	for _, r := range reqs {
		pbReqs = append(pbReqs, requestToProto(r))
	}
	return &pb.GetPendingApplicationsResp{Requests: pbReqs, Total: int32(total)}, nil
}

// GetUserApplications 获取用户的入群申请记录。
func (h *groupHandler) GetUserApplications(ctx context.Context, req *pb.GetUserApplicationsReq) (*pb.GetUserApplicationsResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	reqs, total, err := h.svc.GetUserApplications(ctx, userID, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	pbReqs := make([]*pb.GroupRequestInfo, 0, len(reqs))
	for _, r := range reqs {
		pbReqs = append(pbReqs, requestToProto(r))
	}
	return &pb.GetUserApplicationsResp{Requests: pbReqs, Total: int32(total)}, nil
}

// HandleApplication 处理入群申请（同意/拒绝）。
func (h *groupHandler) HandleApplication(ctx context.Context, req *pb.HandleApplicationReq) (*pb.HandleApplicationResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	if err := h.svc.HandleApplication(ctx, &types.HandleInput{
		GroupID:      req.GroupId,
		UserID:       req.UserId,
		OpUserID:     userID,
		HandleResult: int(req.HandleResult),
		HandledMsg:   req.HandledMsg,
	}); err != nil {
		logger.Error(ctx, "handle application failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.HandleApplicationResp{Success: true, Message: "application handled"}, nil
}

// GetUnhandledApplicationCount 获取未处理的入群申请数量。
func (h *groupHandler) GetUnhandledApplicationCount(ctx context.Context, req *pb.GetUnhandledApplicationCountReq) (*pb.GetUnhandledApplicationCountResp, error) {
	userID, authErr := authenticatedUserID(ctx)
	if authErr != nil {
		return nil, authErr
	}
	count, err := h.svc.GetUnhandledApplicationCount(ctx, req.GroupId, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetUnhandledApplicationCountResp{Count: int32(count)}, nil
}
