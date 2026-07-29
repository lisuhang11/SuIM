// Package handler 提供 user 服务的 REST 端点。
package handler

import (
	"context"
	"net/http"
	"strings"
	"time"

	pb "SuIM/proto/userpb"

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
	r.GET("/me", h.GetCurrentUser) // 从 JWT 上下文获取当前用户
	r.POST("/me/avatar/initiate", h.InitiateAvatarUpload)
	r.POST("/me/avatar/:file_id/complete", h.CompleteAvatarUpload)
	r.GET("/:id", h.GetUser)
	r.GET("/batch", h.GetUsersByIDs)
	r.PUT("/:id", h.UpdateUser)
	r.DELETE("/:id", h.DeleteUser)
	r.PUT("/password", h.ChangePassword)
	r.POST("/validate-token", h.ValidateToken)
	r.POST("/refresh-token", h.RefreshToken)
	r.POST("/logout", h.Logout)
	r.GET("/search", h.SearchUsers)
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

// GetCurrentUser GET /users/me — 从 JWT 鉴权上下文中获取当前登录用户信息
func (h *UserHandler) GetCurrentUser(c *gin.Context) {
	userID := userIDFromCtx(c)
	if userID == "" {
		c.JSON(401, gin.H{"code": 401, "message": "not authenticated"})
		return
	}
	req := &pb.GetUserReq{UserId: userID}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetUser(ctx, req)
	recordGRPC("user", "GetUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetUser GET /users/:id
func (h *UserHandler) GetUser(c *gin.Context) {
	req := &pb.GetUserReq{UserId: c.Param("id")}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetUser(ctx, req)
	recordGRPC("user", "GetUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetUsersByIDs GET /users/batch?ids=1,2,3
func (h *UserHandler) GetUsersByIDs(c *gin.Context) {
	idsStr := c.Query("ids")
	if idsStr == "" {
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
	// 从逗号分隔的 query 参数构建请求
	ids := splitComma(idsStr)
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

// UpdateUser PUT /users/:id
func (h *UserHandler) UpdateUser(c *gin.Context) {
	var body struct {
		Nickname  *string      `json:"nickname"`
		AvatarURL *string      `json:"avatar_url"`
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	current, err := h.client.GetUser(ctx, &pb.GetUserReq{UserId: userID})
	if err != nil {
		RespondError(c, err)
		return
	}
	if current.User == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "user not found"})
		return
	}
	if body.User != nil {
		if body.Nickname == nil {
			body.Nickname = &body.User.Nickname
		}
		if body.AvatarURL == nil {
			body.AvatarURL = &body.User.AvatarUrl
		}
	}
	if body.Nickname != nil {
		nickname := strings.TrimSpace(*body.Nickname)
		if nickname == "" || len([]rune(nickname)) > 64 {
			c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "nickname must be between 1 and 64 characters"})
			return
		}
		current.User.Nickname = nickname
	}
	if body.AvatarURL != nil {
		current.User.AvatarUrl = *body.AvatarURL
	}
	req := pb.UpdateUserReq{User: current.User}
	start := time.Now()
	resp, err := h.client.UpdateUser(ctx, &req)
	recordGRPC("user", "UpdateUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// DeleteUser DELETE /users/:id
func (h *UserHandler) DeleteUser(c *gin.Context) {
	req := &pb.DeleteUserReq{UserId: c.Param("id")}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
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
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
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
	var req pb.LogoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
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
