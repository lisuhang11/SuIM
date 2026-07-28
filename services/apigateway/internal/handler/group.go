// Package handler 提供 group 服务的 REST 端点。
package handler

import (
	"context"
	"time"

	pb "SuIM/proto/grouppb"

	"github.com/gin-gonic/gin"
)

// GroupHandler 处理 /api/v1/groups 路由。
type GroupHandler struct {
	client pb.GroupServiceClient
}

// NewGroupHandler 创建 group handler。
func NewGroupHandler(client pb.GroupServiceClient) *GroupHandler {
	return &GroupHandler{client: client}
}

// RegisterRoutes 注册路由。
func (h *GroupHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("", h.CreateGroup)
	r.GET("/joined", h.GetJoinedGroups)
	r.GET("/:id", h.GetGroup)
	r.PUT("/:id", h.UpdateGroupInfo)
	r.DELETE("/:id", h.DismissGroup)
	r.PUT("/:id/owner", h.TransferGroupOwner)
	r.POST("/:id/members", h.InviteUserToGroup)
	r.DELETE("/:id/members/:user_id", h.KickGroupMember)
	r.POST("/:id/quit", h.QuitGroup)
	r.GET("/:id/members", h.GetGroupMembers)
	r.PUT("/:id/mute", h.SetGroupMute)
	r.PUT("/:id/members/:user_id/mute", h.SetMemberMute)
	r.POST("/:id/apply", h.ApplyToJoinGroup)
	r.GET("/:id/applications", h.GetPendingApplications)
	r.GET("/applications/mine", h.GetUserApplications)
	r.PUT("/applications/:id", h.HandleApplication)
	r.GET("/unhandled-application-count", h.GetUnhandledApplicationCount)
}

// CreateGroup POST /groups
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	var req pb.CreateGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.CreateGroup(ctx, &req)
	recordGRPC("group", "CreateGroup", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// DismissGroup DELETE /groups/:id
func (h *GroupHandler) DismissGroup(c *gin.Context) {
	req := &pb.DismissGroupReq{GroupId: c.Param("id")}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.DismissGroup(ctx, req)
	recordGRPC("group", "DismissGroup", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// TransferGroupOwner PUT /groups/:id/owner
func (h *GroupHandler) TransferGroupOwner(c *gin.Context) {
	var req pb.TransferGroupOwnerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.GroupId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.TransferGroupOwner(ctx, &req)
	recordGRPC("group", "TransferGroupOwner", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// UpdateGroupInfo PUT /groups/:id
func (h *GroupHandler) UpdateGroupInfo(c *gin.Context) {
	var req pb.UpdateGroupInfoReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.GroupId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.UpdateGroupInfo(ctx, &req)
	recordGRPC("group", "UpdateGroupInfo", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetGroup GET /groups/:id
func (h *GroupHandler) GetGroup(c *gin.Context) {
	req := &pb.GetGroupReq{GroupId: c.Param("id")}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetGroup(ctx, req)
	recordGRPC("group", "GetGroup", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// InviteUserToGroup POST /groups/:id/members
func (h *GroupHandler) InviteUserToGroup(c *gin.Context) {
	var req pb.InviteUserToGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.GroupId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.InviteUserToGroup(ctx, &req)
	recordGRPC("group", "InviteUserToGroup", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// KickGroupMember DELETE /groups/:id/members/:user_id
func (h *GroupHandler) KickGroupMember(c *gin.Context) {
	req := &pb.KickGroupMemberReq{
		GroupId: c.Param("id"),
		UserId:  c.Param("user_id"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.KickGroupMember(ctx, req)
	recordGRPC("group", "KickGroupMember", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// QuitGroup POST /groups/:id/quit
func (h *GroupHandler) QuitGroup(c *gin.Context) {
	var req pb.QuitGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.GroupId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.QuitGroup(ctx, &req)
	recordGRPC("group", "QuitGroup", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetGroupMembers GET /groups/:id/members?offset=0&limit=20
func (h *GroupHandler) GetGroupMembers(c *gin.Context) {
	req := &pb.GetGroupMembersReq{
		GroupId: c.Param("id"),
		Offset:  parseInt32(c.Query("offset"), 0),
		Limit:   parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetGroupMembers(ctx, req)
	recordGRPC("group", "GetGroupMembers", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetJoinedGroups GET /groups/joined?user_id=&offset=0&limit=20
func (h *GroupHandler) GetJoinedGroups(c *gin.Context) {
	req := &pb.GetJoinedGroupsReq{
		UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c)),
		Offset: parseInt32(c.Query("offset"), 0),
		Limit:  parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetJoinedGroups(ctx, req)
	recordGRPC("group", "GetJoinedGroups", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// SetGroupMute PUT /groups/:id/mute
func (h *GroupHandler) SetGroupMute(c *gin.Context) {
	var req pb.SetGroupMuteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.GroupId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SetGroupMute(ctx, &req)
	recordGRPC("group", "SetGroupMute", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// SetMemberMute PUT /groups/:id/members/:user_id/mute
func (h *GroupHandler) SetMemberMute(c *gin.Context) {
	var req pb.SetMemberMuteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.GroupId = c.Param("id")
	req.UserId = c.Param("user_id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SetMemberMute(ctx, &req)
	recordGRPC("group", "SetMemberMute", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// ApplyToJoinGroup POST /groups/:id/apply
func (h *GroupHandler) ApplyToJoinGroup(c *gin.Context) {
	var req pb.ApplyToJoinGroupReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.GroupId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.ApplyToJoinGroup(ctx, &req)
	recordGRPC("group", "ApplyToJoinGroup", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetPendingApplications GET /groups/:id/applications?offset=0&limit=20
func (h *GroupHandler) GetPendingApplications(c *gin.Context) {
	req := &pb.GetPendingApplicationsReq{
		GroupId: c.Param("id"),
		Offset:  parseInt32(c.Query("offset"), 0),
		Limit:   parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetPendingApplications(ctx, req)
	recordGRPC("group", "GetPendingApplications", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetUserApplications GET /groups/applications/mine?offset=0&limit=20
func (h *GroupHandler) GetUserApplications(c *gin.Context) {
	req := &pb.GetUserApplicationsReq{
		UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c)),
		Offset: parseInt32(c.Query("offset"), 0),
		Limit:  parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetUserApplications(ctx, req)
	recordGRPC("group", "GetUserApplications", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// HandleApplication PUT /groups/applications/:id
func (h *GroupHandler) HandleApplication(c *gin.Context) {
	var req pb.HandleApplicationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.HandleApplication(ctx, &req)
	recordGRPC("group", "HandleApplication", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetUnhandledApplicationCount GET /groups/unhandled-application-count
func (h *GroupHandler) GetUnhandledApplicationCount(c *gin.Context) {
	req := &pb.GetUnhandledApplicationCountReq{
		GroupId: c.Query("group_id"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetUnhandledApplicationCount(ctx, req)
	recordGRPC("group", "GetUnhandledApplicationCount", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// firstNonEmpty 返回第一个非空字符串。
func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
