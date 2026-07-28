// Package handler 提供 user 服务的 REST 端点。
package handler

import (
	"context"
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
	var req pb.UpdateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	// user_id 从 JSON body 的 User 字段中获取；path param 仅做 URL 级标识
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
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
