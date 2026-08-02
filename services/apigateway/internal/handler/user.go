// Package handler 提供 user 服务的 REST 端点。
package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	pb "SuIM/proto/userpb"
	"apigateway/internal/middleware"

	"github.com/gin-gonic/gin"
)

// UserHandler 处理 /api/v1/users 路由。
type UserHandler struct {
	client pb.UserServiceClient
}

// NewUserHandler 创建 user handler。
func NewUserHandler(client pb.UserServiceClient) *UserHandler {
	return &UserHandler{client: client}
}

// RegisterRoutes 注册路由。
func (h *UserHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/register", h.handleRegister)
	r.POST("/login", h.Login)
	// 用户资料读统一走 batch（对齐 OpenIM getDesignateUsers）；本人用 ids=<self>。
	r.GET("/batch", h.GetUsersByIDs)
	r.POST("/me/avatar/initiate", h.InitiateAvatarUpload)
	r.POST("/me/avatar/:file_id/complete", h.CompleteAvatarUpload)
	r.GET("/me/global-recv-msg-opt", h.GetGlobalRecvMessageOpt)
	r.PUT("/me/global-recv-msg-opt", h.SetGlobalRecvMessageOpt)
	r.GET("/search", h.SearchUsers)
	r.PUT("/password", h.ChangePassword)
	r.POST("/validate-token", h.ValidateToken)
	r.POST("/refresh-token", h.RefreshToken)
	r.POST("/logout", h.Logout)
	r.PUT("/me", h.UpdateMe)
	r.PUT("/:id", h.UpdateUser)
	r.DELETE("/:id", h.DeleteUser)
}

func (h *UserHandler) InitiateAvatarUpload(c *gin.Context) {
	var req pb.InitiateUserAvatarUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	resp, err := h.client.InitiateAvatarUpload(ctx, &req)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *UserHandler) CompleteAvatarUpload(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 90*time.Second)
	defer cancel()
	resp, err := h.client.CompleteAvatarUpload(ctx, &pb.CompleteUserAvatarUploadReq{FileId: c.Param("file_id")})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// handleRegister POST /users/register
func (h *UserHandler) handleRegister(c *gin.Context) {
	var req pb.RegisterReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.Register(ctx, &req)
	recordGRPC("user", "Register", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// Login POST /users/login
func (h *UserHandler) Login(c *gin.Context) {
	var req pb.LoginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.Login(ctx, &req)
	recordGRPC("user", "Login", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetGlobalRecvMessageOpt GET /users/me/global-recv-msg-opt
func (h *UserHandler) GetGlobalRecvMessageOpt(c *gin.Context) {
	userID := userIDFromCtx(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "not authenticated"})
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetGlobalRecvMessageOpt(ctx, &pb.GetGlobalRecvMessageOptReq{UserId: userID})
	recordGRPC("user", "GetGlobalRecvMessageOpt", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// SetGlobalRecvMessageOpt PUT /users/me/global-recv-msg-opt
func (h *UserHandler) SetGlobalRecvMessageOpt(c *gin.Context) {
	userID := userIDFromCtx(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "not authenticated"})
		return
	}
	var body struct {
		GlobalRecvMsgOpt *int32 `json:"global_recv_msg_opt"`
		// OpenIM 兼容字段名
		GlobalRecvMsgOptCamel *int32 `json:"globalRecvMsgOpt"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, err)
		return
	}
	opt := body.GlobalRecvMsgOpt
	if opt == nil {
		opt = body.GlobalRecvMsgOptCamel
	}
	if opt == nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "global_recv_msg_opt is required"})
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SetGlobalRecvMessageOpt(ctx, &pb.SetGlobalRecvMessageOptReq{
		UserId:           userID,
		GlobalRecvMsgOpt: *opt,
	})
	recordGRPC("user", "SetGlobalRecvMessageOpt", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetUsersByIDs GET /users/batch?ids=1,2,3 — 唯一用户资料读接口
func (h *UserHandler) GetUsersByIDs(c *gin.Context) {
	queryIDs := c.QueryArray("ids")
	if len(queryIDs) == 0 {
		// 也支持 JSON body
		var req pb.GetUsersByIDsReq
		if err := c.ShouldBindJSON(&req); err != nil {
			// 回退：尝试 query
			RespondError(c, err)
			return
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
		defer cancel()
		start := time.Now()
		resp, err := h.client.GetUsersByIDs(ctx, &req)
		recordGRPC("user", "GetUsersByIDs", err, start)
		if err != nil {
			RespondError(c, err)
			return
		}
		Respond(c, resp)
		return
	}
	// 同时支持 ids=a,b 和 ids=a&ids=b。
	ids := make([]string, 0, len(queryIDs))
	for _, value := range queryIDs {
		ids = append(ids, splitComma(value)...)
	}
	req := &pb.GetUsersByIDsReq{UserIds: ids}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetUsersByIDs(ctx, req)
	recordGRPC("user", "GetUsersByIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// UpdateMe PUT /users/me — 更新当前登录用户资料
func (h *UserHandler) UpdateMe(c *gin.Context) {
	c.Params = append(c.Params, gin.Param{Key: "id", Value: "me"})
	h.UpdateUser(c)
}

// UpdateUser PUT /users/:id|/me — 仅提交要改的字段，服务端 UpdateByMap。
func (h *UserHandler) UpdateUser(c *gin.Context) {
	var body struct {
		Nickname  *string      `json:"nickname"`
		AvatarURL *string      `json:"avatar_url"`
		Ex        *string      `json:"ex"`
		User      *pb.UserInfo `json:"user"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, err)
		return
	}
	userID := userIDFromCtx(c)
	if pathID := c.Param("id"); pathID != "me" && pathID != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "only your own profile can be updated"})
		return
	}
	if body.User != nil {
		if body.Nickname == nil && body.User.Nickname != "" {
			body.Nickname = &body.User.Nickname
		}
		if body.AvatarURL == nil && body.User.AvatarUrl != "" {
			body.AvatarURL = &body.User.AvatarUrl
		}
		if body.Ex == nil && body.User.Ex != "" {
			body.Ex = &body.User.Ex
		}
	}
	info := &pb.UserInfo{UserId: userID}
	if body.Nickname != nil {
		nickname := strings.TrimSpace(*body.Nickname)
		if nickname == "" || len([]rune(nickname)) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "nickname must be between 1 and 64 characters"})
			return
		}
		info.Nickname = nickname
	}
	if body.AvatarURL != nil {
		info.AvatarUrl = *body.AvatarURL
	}
	if body.Ex != nil {
		info.Ex = *body.Ex
	}
	if info.Nickname == "" && info.AvatarUrl == "" && info.Ex == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "at least one field must be updated"})
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.UpdateUser(ctx, &pb.UpdateUserReq{User: info})
	recordGRPC("user", "UpdateUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// DeleteUser DELETE /users/:id
func (h *UserHandler) DeleteUser(c *gin.Context) {
	userID := userIDFromCtx(c)
	if c.Param("id") != userID {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "only your own account can be deleted"})
		return
	}
	req := &pb.DeleteUserReq{UserId: userID}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.DeleteUser(ctx, req)
	recordGRPC("user", "DeleteUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// ChangePassword PUT /users/password
func (h *UserHandler) ChangePassword(c *gin.Context) {
	var req pb.ChangePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.UserId = userIDFromCtx(c)
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.ChangePassword(ctx, &req)
	recordGRPC("user", "ChangePassword", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// ValidateToken POST /users/validate-token
func (h *UserHandler) ValidateToken(c *gin.Context) {
	var req pb.ValidateTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.ValidateToken(ctx, &req)
	recordGRPC("user", "ValidateToken", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// RefreshToken POST /users/refresh-token
func (h *UserHandler) RefreshToken(c *gin.Context) {
	var req pb.RefreshTokenReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.RefreshToken(ctx, &req)
	recordGRPC("user", "RefreshToken", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// Logout POST /users/logout
func (h *UserHandler) Logout(c *gin.Context) {
	req := pb.LogoutReq{Token: middleware.GetToken(c)}
	if req.Token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "not authenticated"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.Logout(ctx, &req)
	recordGRPC("user", "Logout", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// SearchUsers GET /users/search?keyword=&offset=0&limit=20
func (h *UserHandler) SearchUsers(c *gin.Context) {
	keyword := c.Query("keyword")
	limit := parseInt32(c.Query("limit"), 20)
	req := &pb.SearchUsersReq{
		Query: keyword,
		Limit: limit,
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SearchUsers(ctx, req)
	recordGRPC("user", "SearchUsers", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
