// Package handler 提供 relation 服务的 REST 端点。
package handler

import (
	"context"
	"time"

	pb "SuIM/proto/relationpb"

	"github.com/gin-gonic/gin"
)

// RelationHandler 处理 /api/v1/relations 路由。
type RelationHandler struct {
	client pb.RelationServiceClient
}

// NewRelationHandler 创建 relation handler。
func NewRelationHandler(client pb.RelationServiceClient) *RelationHandler {
	return &RelationHandler{client: client}
}

// RegisterRoutes 注册路由。
func (h *RelationHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/friend-requests", h.SendFriendRequest)
	r.PUT("/friend-requests/:id/respond", h.RespondFriendApply)
	r.GET("/incoming-applies", h.GetIncomingApplyTo)
	r.GET("/outgoing-applies", h.GetOutgoingApplyFrom)
	r.GET("/unhandled-count", h.GetUnhandledApplyCount)
	r.DELETE("/friends/:friend_id", h.DeleteFriend)
	r.GET("/friends", h.GetFriends)
	r.POST("/blocks", h.BlockUser)
	r.DELETE("/blocks/:user_id", h.UnblockUser)
	r.GET("/blocks", h.GetBlockedUsers)
	r.GET("/is-friend", h.IsFriend)
	r.GET("/is-black", h.IsBlack)
}

// SendFriendRequest POST /relations/friend-requests
func (h *RelationHandler) SendFriendRequest(c *gin.Context) {
	var req pb.SendFriendRequestReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SendFriendRequest(ctx, &req)
	recordGRPC("relation", "SendFriendRequest", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// RespondFriendApply PUT /relations/friend-requests/:id/respond
func (h *RelationHandler) RespondFriendApply(c *gin.Context) {
	var req pb.RespondFriendApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.RespondFriendApply(ctx, &req)
	recordGRPC("relation", "RespondFriendApply", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetIncomingApplyTo GET /relations/incoming-applies?handle_results=0&offset=0&limit=20
func (h *RelationHandler) GetIncomingApplyTo(c *gin.Context) {
	req := &pb.GetIncomingApplyToReq{
		UserId:        userIDFromCtx(c),
		Limit:         parseInt32(c.Query("limit"), 20),
		Offset:        parseInt32(c.Query("offset"), 0),
		HandleResults: parseInt32Slice(c.Query("handle_results")),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetIncomingApplyTo(ctx, req)
	recordGRPC("relation", "GetIncomingApplyTo", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetOutgoingApplyFrom GET /relations/outgoing-applies?handle_results=0&offset=0&limit=20
func (h *RelationHandler) GetOutgoingApplyFrom(c *gin.Context) {
	req := &pb.GetOutgoingApplyFromReq{
		UserId:        userIDFromCtx(c),
		Limit:         parseInt32(c.Query("limit"), 20),
		Offset:        parseInt32(c.Query("offset"), 0),
		HandleResults: parseInt32Slice(c.Query("handle_results")),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetOutgoingApplyFrom(ctx, req)
	recordGRPC("relation", "GetOutgoingApplyFrom", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetUnhandledApplyCount GET /relations/unhandled-count
func (h *RelationHandler) GetUnhandledApplyCount(c *gin.Context) {
	req := &pb.GetUnhandledApplyCountReq{UserId: userIDFromCtx(c)}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetUnhandledApplyCount(ctx, req)
	recordGRPC("relation", "GetUnhandledApplyCount", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// DeleteFriend DELETE /relations/friends/:friend_id
func (h *RelationHandler) DeleteFriend(c *gin.Context) {
	req := &pb.DeleteFriendReq{
		UserId:   userIDFromCtx(c),
		FriendId: c.Param("friend_id"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.DeleteFriend(ctx, req)
	recordGRPC("relation", "DeleteFriend", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetFriends GET /relations/friends?offset=0&limit=20
func (h *RelationHandler) GetFriends(c *gin.Context) {
	req := &pb.GetFriendsReq{
		UserId: userIDFromCtx(c),
		Offset: parseInt32(c.Query("offset"), 0),
		Limit:  parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetFriends(ctx, req)
	recordGRPC("relation", "GetFriends", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// BlockUser POST /relations/blocks
func (h *RelationHandler) BlockUser(c *gin.Context) {
	var req pb.BlockUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.BlockUser(ctx, &req)
	recordGRPC("relation", "BlockUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// UnblockUser DELETE /relations/blocks/:user_id
func (h *RelationHandler) UnblockUser(c *gin.Context) {
	req := &pb.UnblockUserReq{
		UserId:        userIDFromCtx(c),
		BlockedUserId: c.Param("user_id"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.UnblockUser(ctx, req)
	recordGRPC("relation", "UnblockUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetBlockedUsers GET /relations/blocks?offset=0&limit=20
func (h *RelationHandler) GetBlockedUsers(c *gin.Context) {
	req := &pb.GetBlockedUsersReq{
		UserId: userIDFromCtx(c),
		Offset: parseInt32(c.Query("offset"), 0),
		Limit:  parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetBlockedUsers(ctx, req)
	recordGRPC("relation", "GetBlockedUsers", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// IsFriend GET /relations/is-friend?user1=&user2=
func (h *RelationHandler) IsFriend(c *gin.Context) {
	req := &pb.IsFriendReq{
		User1: c.Query("user1"),
		User2: c.Query("user2"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.IsFriend(ctx, req)
	recordGRPC("relation", "IsFriend", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// IsBlack GET /relations/is-black?user1=&user2=
func (h *RelationHandler) IsBlack(c *gin.Context) {
	req := &pb.IsBlackReq{
		User1: c.Query("user1"),
		User2: c.Query("user2"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.IsBlack(ctx, req)
	recordGRPC("relation", "IsBlack", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
