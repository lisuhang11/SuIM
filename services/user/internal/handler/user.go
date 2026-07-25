// Package handler adapts the domain UserService to the gRPC transport layer.
package handler

import (
	"context"
	"log/slog"

	"user/internal/repository"
	"user/internal/types"
	"user/internal/types/interfaces"
	pb "user/proto/userpb"
)

// userHandler implements pb.UserServiceServer by delegating to the domain UserService.
type userHandler struct {
	pb.UnimplementedUserServiceServer
	svc interfaces.UserService
}

// NewUserHandler creates a gRPC UserServiceServer wired to the given domain service.
func NewUserHandler(svc interfaces.UserService) pb.UserServiceServer {
	return &userHandler{svc: svc}
}

// --------------- conversion helpers ---------------

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

// --------------- RPC implementations ---------------

func (h *userHandler) Register(ctx context.Context, req *pb.RegisterReq) (*pb.RegisterResp, error) {
	resp, err := h.svc.Register(ctx, &types.RegisterRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		slog.ErrorContext(ctx, "register failed", "error", err)
		return &pb.RegisterResp{Success: false, Message: err.Error()}, nil
	}
	return &pb.RegisterResp{
		Success: resp.Success,
		Message: resp.Message,
		User:    userToProto(resp.User),
	}, nil
}

func (h *userHandler) Login(ctx context.Context, req *pb.LoginReq) (*pb.LoginResp, error) {
	resp, err := h.svc.Login(ctx, &types.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		return nil, err
	}
	return &pb.LoginResp{
		Success:      resp.Success,
		Message:      resp.Message,
		User:         userToProto(resp.User),
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
	}, nil
}

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

func (h *userHandler) DeleteUser(ctx context.Context, req *pb.DeleteUserReq) (*pb.DeleteUserResp, error) {
	if err := h.svc.DeleteUser(ctx, req.UserId); err != nil {
		return nil, err
	}
	return &pb.DeleteUserResp{Success: true}, nil
}

func (h *userHandler) ChangePassword(ctx context.Context, req *pb.ChangePasswordReq) (*pb.ChangePasswordResp, error) {
	if err := h.svc.ChangePassword(ctx, req.UserId, req.OldPassword, req.NewPassword); err != nil {
		return &pb.ChangePasswordResp{Success: false, Message: err.Error()}, nil
	}
	return &pb.ChangePasswordResp{Success: true, Message: "password changed"}, nil
}

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

func (h *userHandler) Logout(ctx context.Context, req *pb.LogoutReq) (*pb.LogoutResp, error) {
	if err := h.svc.Logout(ctx, req.Token); err != nil {
		return &pb.LogoutResp{Success: false}, nil
	}
	return &pb.LogoutResp{Success: true}, nil
}

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
