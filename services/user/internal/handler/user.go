// Package handler 将领域 UserService 适配为 gRPC 传输层处理逻辑。
package handler

import (
	"context"
	"log/slog"

	apperrors "user/internal/errors"
	"user/internal/repository"
	"user/internal/types"
	"user/internal/types/interfaces"
	pb "SuIM/proto/userpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// userHandler 实现 pb.UserServiceServer，将请求委托给领域 UserService。
type userHandler struct {
	pb.UnimplementedUserServiceServer
	svc interfaces.UserService
}

// NewUserHandler 创建绑定到指定领域服务的 gRPC UserServiceServer。
func NewUserHandler(svc interfaces.UserService) pb.UserServiceServer {
	return &userHandler{svc: svc}
}

// --------------- 类型转换辅助函数 ---------------

// userToProto 将领域 User 模型转换为 proto UserInfo。
func userToProto(u *types.User) *pb.UserInfo {
	if u == nil {
		return nil
	}
	return &pb.UserInfo{
		UserId:           u.UserID,
		Nickname:         u.Nickname,
		Email:            u.Email,
		AvatarUrl:        u.AvatarURL,
		Ex:               u.Ex,
		AppMangerLevel:   int32(u.AppMangerLevel),
		GlobalRecvMsgOpt: int32(u.GlobalRecvMsgOpt),
		IsActive:         u.IsActive,
		CreateTime:       u.CreateTime.UnixMilli(),
		UpdatedAt:        u.UpdatedAt.UnixMilli(),
	}
}

// protoToUser 将 proto UserInfo 转换为领域 User 模型。
func protoToUser(p *pb.UserInfo) *types.User {
	if p == nil {
		return nil
	}
	return &types.User{
		UserID:           p.UserId,
		Email:            p.Email,
		Nickname:         p.Nickname,
		AvatarURL:        p.AvatarUrl,
		Ex:               p.Ex,
		AppMangerLevel:   int(p.AppMangerLevel),
		GlobalRecvMsgOpt: int(p.GlobalRecvMsgOpt),
		IsActive:         p.IsActive,
	}
}

// --------------- RPC 实现 ---------------

// Register 处理用户注册请求。
func (h *userHandler) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	user, err := h.svc.Register(ctx, req.Email, req.Username, req.Password)
	if err != nil {
		slog.ErrorContext(ctx, "register failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.RegisterResp{
		Success: true,
		Message: "registration successful",
		User:    userToProto(user),
	}, nil
}

// Login 处理用户登录请求，返回访问令牌和刷新令牌。
func (h *userHandler) Login(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	user, accessToken, refreshToken, err := h.svc.Login(ctx, req.Email, req.Password)
	if err != nil {
		slog.ErrorContext(ctx, "login failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.LoginResp{
		Success:      true,
		Message:      "login successful",
		User:         userToProto(user),
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// GetUser 根据用户 ID 获取用户信息。
func (h *userHandler) GetUser(ctx context.Context, req *pb.GetUserReq) (*pb.GetUserResp, error) {
	user, err := h.svc.GetUserByID(ctx, req.UserId)
	if err != nil {
		if err == repository.ErrUserNotFound {
			return &pb.GetUserResp{}, nil
		}
		return nil, err
	}
	return &pb.GetUserResp{User: userToProto(user)}, nil
}

// GetUsersByIDs 批量获取用户信息，返回 ID 到 UserInfo 的映射。
func (h *userHandler) GetUsersByIDs(ctx context.Context, req *pb.GetUsersByIDsReq) (*pb.GetUsersByIDsResp, error) {
	users, err := h.svc.GetUsersByIDs(ctx, req.UserIds)
	if err != nil {
		return nil, err
	}
	m := make(map[string]*pb.UserInfo, len(users))
	for id, u := range users {
		m[id] = userToProto(u)
	}
	return &pb.GetUsersByIDsResp{Users: m}, nil
}

// UpdateUser 更新用户信息。
func (h *userHandler) UpdateUser(ctx context.Context, req *pb.UpdateUserReq) (*pb.UpdateUserResp, error) {
	u := protoToUser(req.User)
	if u == nil {
		return &pb.UpdateUserResp{Success: false}, nil
	}
	if err := h.svc.UpdateUser(ctx, u); err != nil {
		return nil, err
	}
	return &pb.UpdateUserResp{Success: true}, nil
}

// DeleteUser 删除指定用户。
func (h *userHandler) DeleteUser(ctx context.Context, req *pb.DeleteUserReq) (*pb.DeleteUserResp, error) {
	if err := h.svc.DeleteUser(ctx, req.UserId); err != nil {
		return nil, err
	}
	return &pb.DeleteUserResp{Success: true}, nil
}

// ChangePassword 修改用户密码。
func (h *userHandler) ChangePassword(ctx context.Context, req *pb.ChangePasswordReq) (*pb.ChangePasswordResp, error) {
	if err := h.svc.ChangePassword(ctx, req.UserId, req.OldPassword, req.NewPassword); err != nil {
		return &pb.ChangePasswordResp{Success: false, Message: err.Error()}, nil
	}
	return &pb.ChangePasswordResp{Success: true, Message: "password changed"}, nil
}

// ValidateToken 验证访问令牌是否有效，返回关联的用户信息。
func (h *userHandler) ValidateToken(ctx context.Context, req *pb.ValidateTokenReq) (*pb.ValidateTokenResp, error) {
	user, err := h.svc.ValidateToken(ctx, req.Token)
	if err != nil {
		return &pb.ValidateTokenResp{Valid: false}, nil
	}
	return &pb.ValidateTokenResp{
		Valid: true,
		User:  userToProto(user),
	}, nil
}

// RefreshToken 使用刷新令牌换取新的访问令牌和刷新令牌。
func (h *userHandler) RefreshToken(ctx context.Context, req *pb.RefreshTokenReq) (*pb.RefreshTokenResp, error) {
	access, refresh, err := h.svc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &pb.RefreshTokenResp{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

// Logout 处理用户登出，吊销所有令牌。
func (h *userHandler) Logout(ctx context.Context, req *pb.LogoutReq) (*pb.LogoutResp, error) {
	if err := h.svc.Logout(ctx, req.Token); err != nil {
		return &pb.LogoutResp{Success: false}, nil
	}
	return &pb.LogoutResp{Success: true}, nil
}

// SearchUsers 根据昵称或邮箱搜索用户。
func (h *userHandler) SearchUsers(ctx context.Context, req *pb.SearchUsersReq) (*pb.SearchUsersResp, error) {
	users, err := h.svc.SearchUsers(ctx, req.Query, int(req.Limit))
	if err != nil {
		return nil, err
	}
	pbUsers := make([]*pb.UserInfo, 0, len(users))
	for _, u := range users {
		pbUsers = append(pbUsers, userToProto(u))
	}
	return &pb.SearchUsersResp{Users: pbUsers}, nil
}

// --------------- 错误转换 ---------------

// appErrorToStatus 将 *apperrors.AppError 映射为 gRPC status.Error，非 AppError 降级为 Internal。
func appErrorToStatus(err error) error {
	ae := apperrors.GetAppError(err)
	if ae == nil {
		return status.Error(codes.Internal, err.Error())
	}
	var code codes.Code
	switch ae.Code {
	case apperrors.CodeValidation:
		code = codes.InvalidArgument
	case apperrors.CodeUnauthorized, apperrors.CodePasswordInvalid,
		apperrors.CodeTokenInvalid, apperrors.CodeTokenExpired, apperrors.CodeTokenRevoked:
		code = codes.Unauthenticated
	case apperrors.CodeForbidden, apperrors.CodeUserInactive:
		code = codes.PermissionDenied
	case apperrors.CodeUserNotFound:
		code = codes.NotFound
	case apperrors.CodeUserExists, apperrors.CodeTokenWrongType:
		code = codes.AlreadyExists
	case apperrors.CodePasswordPolicy:
		code = codes.InvalidArgument
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}
