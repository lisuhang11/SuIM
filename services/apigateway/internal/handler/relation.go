// Package handler 提供 relation 服务的 REST 端点。
package handler

import (
	"context"
	"net/http"
	"time"

	pbConv "SuIM/proto/conversationpb"
	pb "SuIM/proto/relationpb"

	"github.com/gin-gonic/gin"
)

// RelationHandler 处理 /api/v1/relations 路由。
type RelationHandler struct {
	client     pb.RelationServiceClient
	conversation pbConv.ConversationClient
}

// NewRelationHandler 创建 relation handler。
func NewRelationHandler(client pb.RelationServiceClient, conversation pbConv.ConversationClient) *RelationHandler {
	return &RelationHandler{client: client, conversation: conversation}
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
	r.POST("/friends/incremental", h.GetIncrementalFriends)
	r.POST("/friends/full-ids", h.GetFullFriendUserIDs)
	r.PUT("/friends/:friend_id", h.UpdateFriend)
	r.POST("/blocks", h.BlockUser)
	r.DELETE("/blocks/:user_id", h.UnblockUser)
	r.GET("/blocks", h.GetBlockedUsers)
	r.GET("/is-friend", h.IsFriend)
	r.GET("/is-black", h.IsBlack)
}

// SendFriendRequest POST /relations/friend-requests
func (h *RelationHandler) SendFriendRequest(c *gin.Context) {
	var body struct {
		ToUserID string `json:"to_user_id"`
		Message  string `json:"message"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, err)
		return
	}
	req := &pb.SendFriendRequestReq{
		FromUserId: userIDFromCtx(c),
		ToUserId:   body.ToUserID,
		Message:    body.Message,
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SendFriendRequest(ctx, req)
	recordGRPC("relation", "SendFriendRequest", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// RespondFriendApply PUT /relations/friend-requests/:id/respond
func (h *RelationHandler) RespondFriendApply(c *gin.Context) {
	var body struct {
		HandleResult int32  `json:"handle_result"`
		HandleMsg    string `json:"handle_msg"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, err)
		return
	}
	fromUserID := c.Param("id")
	toUserID := userIDFromCtx(c)
	req := &pb.RespondFriendApplyReq{
		FromUserId:   fromUserID,
		ToUserId:     toUserID,
		HandleResult: body.HandleResult,
		HandleMsg:    body.HandleMsg,
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.RespondFriendApply(ctx, req)
	recordGRPC("relation", "RespondFriendApply", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}

	// 同意后自动创建双向单聊会话（失败不影响好友关系已建立）。
	if body.HandleResult == 1 && h.conversation != nil {
		convStart := time.Now()
		_, convErr := h.conversation.CreateSingleChatConversations(ctx, &pbConv.CreateSingleChatConversationsReq{
			SendId:           toUserID,
			RecvId:           fromUserID,
			ConversationType: 1,
		})
		recordGRPC("conversation", "CreateSingleChatConversations", convErr, convStart)
		if convErr != nil {
			// 好友已建立；会话可稍后由客户端 createPrivateConversation 补建。
			_ = convErr
		}
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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

// GetIncrementalFriends POST /relations/friends/incremental
func (h *RelationHandler) GetIncrementalFriends(c *gin.Context) {
	var body struct {
		VersionID string `json:"version_id"`
		Version   uint64 `json:"version"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, err)
		return
	}
	req := &pb.GetIncrementalFriendsReq{
		UserId:    userIDFromCtx(c),
		VersionId: body.VersionID,
		Version:   body.Version,
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetIncrementalFriends(ctx, req)
	recordGRPC("relation", "GetIncrementalFriends", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetFullFriendUserIDs POST /relations/friends/full-ids
func (h *RelationHandler) GetFullFriendUserIDs(c *gin.Context) {
	req := &pb.GetFullFriendUserIDsReq{UserId: userIDFromCtx(c)}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetFullFriendUserIDs(ctx, req)
	recordGRPC("relation", "GetFullFriendUserIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// UpdateFriend PUT /relations/friends/:friend_id
func (h *RelationHandler) UpdateFriend(c *gin.Context) {
	var body struct {
		Remark   *string `json:"remark"`
		IsPinned *bool   `json:"is_pinned"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, err)
		return
	}
	req := &pb.UpdateFriendReq{
		OwnerUserId:  userIDFromCtx(c),
		FriendUserId: c.Param("friend_id"),
		Remark:       body.Remark,
		IsPinned:     body.IsPinned,
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.UpdateFriend(ctx, req)
	recordGRPC("relation", "UpdateFriend", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// BlockUser POST /relations/blocks
func (h *RelationHandler) BlockUser(c *gin.Context) {
	var body struct {
		BlockedUserID string `json:"blocked_user_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		RespondError(c, err)
		return
	}
	req := &pb.BlockUserReq{
		UserId:        userIDFromCtx(c),
		BlockedUserId: body.BlockedUserID,
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.BlockUser(ctx, req)
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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

// IsFriend GET /relations/is-friend?user2=
func (h *RelationHandler) IsFriend(c *gin.Context) {
	peer := c.Query("user2")
	if peer == "" {
		peer = c.Query("user1")
	}
	if peer == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "peer user id is required"})
		return
	}
	req := &pb.IsFriendReq{
		User1: userIDFromCtx(c),
		User2: peer,
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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

// IsBlack GET /relations/is-black?user2=
func (h *RelationHandler) IsBlack(c *gin.Context) {
	peer := c.Query("user2")
	if peer == "" {
		peer = c.Query("user1")
	}
	if peer == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "peer user id is required"})
		return
	}
	req := &pb.IsBlackReq{
		User1: userIDFromCtx(c),
		User2: peer,
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
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
