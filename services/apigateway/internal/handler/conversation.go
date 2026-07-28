// Package handler 提供 conversation 服务的 REST 端点。
package handler

import (
	"context"
	"time"

	pb "SuIM/proto/conversationpb"

	"github.com/gin-gonic/gin"
)

// ConversationHandler 处理 /api/v1/conversations 路由。
type ConversationHandler struct {
	client pb.ConversationClient
}

// NewConversationHandler 创建 conversation handler。
func NewConversationHandler(client pb.ConversationClient) *ConversationHandler {
	return &ConversationHandler{client: client}
}

// RegisterRoutes 注册路由。
func (h *ConversationHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("", h.SetConversation)
	r.GET("/all", h.GetAllConversations)
	r.GET("/ids", h.GetConversationIDs)
	r.GET("/full-ids", h.GetFullOwnerConversationIDs)
	r.GET("/sorted", h.GetSortedConversationList)
	r.GET("/pinned", h.GetPinnedConversationIDs)
	r.GET("/not-notify", h.GetNotNotifyConversationIDs)
	r.GET("/hash", h.GetUserConversationIDsHash)
	r.GET("/owner", h.GetOwnerConversation)
	r.GET("/incremental", h.GetIncrementalConversation)
	r.GET("/need-clear", h.GetConversationsNeedClearMsg)
	r.GET("/batch", h.GetConversations)
	r.GET("/by-ids", h.GetConversationsByConversationID)
	r.GET("/:id", h.GetConversation)
	r.POST("/single", h.CreateSingleChatConversations)
	r.POST("/group", h.CreateGroupChatConversations)
	r.POST("/batch-set", h.SetConversations)
	r.PUT("/:id", h.UpdateConversation)
	r.PUT("/:id/max-seq", h.SetConversationMaxSeq)
	r.PUT("/:id/min-seq", h.SetConversationMinSeq)
	r.PUT("/by-user", h.UpdateConversationsByUser)
	r.POST("/clear-msg", h.ClearUserConversationMsg)
	r.DELETE("/batch", h.DeleteConversations)
	r.GET("/:id/not-notify-users", h.GetRecvMsgNotNotifyUserIDs)
	r.GET("/:id/offline-push-users", h.GetConversationOfflinePushUserIDs)
	r.GET("/:id/not-receive-users", h.GetConversationNotReceiveMessageUserIDs)
}

// SetConversation POST /conversations
func (h *ConversationHandler) SetConversation(c *gin.Context) {
	var req pb.SetConversationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SetConversation(ctx, &req)
	recordGRPC("conversation", "SetConversation", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetConversation GET /conversations/:id?owner_user_id=
func (h *ConversationHandler) GetConversation(c *gin.Context) {
	req := &pb.GetConversationReq{
		OwnerUserId:    c.Query("owner_user_id"),
		ConversationId: c.Param("id"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetConversation(ctx, req)
	recordGRPC("conversation", "GetConversation", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetConversations GET /conversations/batch?owner_user_id=&ids=1,2,3
func (h *ConversationHandler) GetConversations(c *gin.Context) {
	req := &pb.GetConversationsReq{
		OwnerUserId:     c.Query("owner_user_id"),
		ConversationIds: splitComma(c.Query("ids")),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetConversations(ctx, req)
	recordGRPC("conversation", "GetConversations", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetAllConversations GET /conversations/all?owner_user_id=
func (h *ConversationHandler) GetAllConversations(c *gin.Context) {
	req := &pb.GetAllConversationsReq{OwnerUserId: c.Query("owner_user_id")}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetAllConversations(ctx, req)
	recordGRPC("conversation", "GetAllConversations", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetSortedConversationList GET /conversations/sorted?user_id=&ids=1,2,3&offset=0&limit=20
func (h *ConversationHandler) GetSortedConversationList(c *gin.Context) {
	req := &pb.GetSortedConversationListReq{
		UserId:          firstNonEmpty(c.Query("user_id"), userIDFromCtx(c)),
		ConversationIds: splitComma(c.Query("ids")),
		Offset:          parseInt32(c.Query("offset"), 0),
		Limit:           parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetSortedConversationList(ctx, req)
	recordGRPC("conversation", "GetSortedConversationList", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// CreateSingleChatConversations POST /conversations/single
func (h *ConversationHandler) CreateSingleChatConversations(c *gin.Context) {
	var req pb.CreateSingleChatConversationsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.CreateSingleChatConversations(ctx, &req)
	recordGRPC("conversation", "CreateSingleChatConversations", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// CreateGroupChatConversations POST /conversations/group
func (h *ConversationHandler) CreateGroupChatConversations(c *gin.Context) {
	var req pb.CreateGroupChatConversationsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.CreateGroupChatConversations(ctx, &req)
	recordGRPC("conversation", "CreateGroupChatConversations", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// SetConversationMaxSeq PUT /conversations/:id/max-seq
func (h *ConversationHandler) SetConversationMaxSeq(c *gin.Context) {
	var req pb.SetConversationMaxSeqReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.ConversationId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SetConversationMaxSeq(ctx, &req)
	recordGRPC("conversation", "SetConversationMaxSeq", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// SetConversationMinSeq PUT /conversations/:id/min-seq
func (h *ConversationHandler) SetConversationMinSeq(c *gin.Context) {
	var req pb.SetConversationMinSeqReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.ConversationId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SetConversationMinSeq(ctx, &req)
	recordGRPC("conversation", "SetConversationMinSeq", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetConversationIDs GET /conversations/ids?user_id=
func (h *ConversationHandler) GetConversationIDs(c *gin.Context) {
	req := &pb.GetConversationIDsReq{UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c))}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetConversationIDs(ctx, req)
	recordGRPC("conversation", "GetConversationIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// SetConversations POST /conversations/batch-set
func (h *ConversationHandler) SetConversations(c *gin.Context) {
	var req pb.SetConversationsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.SetConversations(ctx, &req)
	recordGRPC("conversation", "SetConversations", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// UpdateConversation PUT /conversations/:id
func (h *ConversationHandler) UpdateConversation(c *gin.Context) {
	var req pb.UpdateConversationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	req.ConversationId = c.Param("id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.UpdateConversation(ctx, &req)
	recordGRPC("conversation", "UpdateConversation", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetConversationsByConversationID GET /conversations/by-ids?ids=1,2,3
func (h *ConversationHandler) GetConversationsByConversationID(c *gin.Context) {
	req := &pb.GetConversationsByConversationIDReq{
		ConversationIds: splitComma(c.Query("ids")),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetConversationsByConversationID(ctx, req)
	recordGRPC("conversation", "GetConversationsByConversationID", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetRecvMsgNotNotifyUserIDs GET /conversations/:id/not-notify-users?group_id=
func (h *ConversationHandler) GetRecvMsgNotNotifyUserIDs(c *gin.Context) {
	req := &pb.GetRecvMsgNotNotifyUserIDsReq{GroupId: c.Query("group_id")}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetRecvMsgNotNotifyUserIDs(ctx, req)
	recordGRPC("conversation", "GetRecvMsgNotNotifyUserIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetConversationOfflinePushUserIDs GET /conversations/:id/offline-push-users?user_ids=1,2,3
func (h *ConversationHandler) GetConversationOfflinePushUserIDs(c *gin.Context) {
	req := &pb.GetConversationOfflinePushUserIDsReq{
		ConversationId: c.Param("id"),
		UserIds:        splitComma(c.Query("user_ids")),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetConversationOfflinePushUserIDs(ctx, req)
	recordGRPC("conversation", "GetConversationOfflinePushUserIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetConversationNotReceiveMessageUserIDs GET /conversations/:id/not-receive-users
func (h *ConversationHandler) GetConversationNotReceiveMessageUserIDs(c *gin.Context) {
	req := &pb.GetConversationNotReceiveMessageUserIDsReq{
		ConversationId: c.Param("id"),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetConversationNotReceiveMessageUserIDs(ctx, req)
	recordGRPC("conversation", "GetConversationNotReceiveMessageUserIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetPinnedConversationIDs GET /conversations/pinned?user_id=
func (h *ConversationHandler) GetPinnedConversationIDs(c *gin.Context) {
	req := &pb.GetPinnedConversationIDsReq{UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c))}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetPinnedConversationIDs(ctx, req)
	recordGRPC("conversation", "GetPinnedConversationIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetNotNotifyConversationIDs GET /conversations/not-notify?user_id=
func (h *ConversationHandler) GetNotNotifyConversationIDs(c *gin.Context) {
	req := &pb.GetNotNotifyConversationIDsReq{UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c))}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetNotNotifyConversationIDs(ctx, req)
	recordGRPC("conversation", "GetNotNotifyConversationIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// DeleteConversations DELETE /conversations/batch
func (h *ConversationHandler) DeleteConversations(c *gin.Context) {
	var req pb.DeleteConversationsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.DeleteConversations(ctx, &req)
	recordGRPC("conversation", "DeleteConversations", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// UpdateConversationsByUser PUT /conversations/by-user
func (h *ConversationHandler) UpdateConversationsByUser(c *gin.Context) {
	var req pb.UpdateConversationsByUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.UpdateConversationsByUser(ctx, &req)
	recordGRPC("conversation", "UpdateConversationsByUser", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetUserConversationIDsHash GET /conversations/hash?owner_user_id=
func (h *ConversationHandler) GetUserConversationIDsHash(c *gin.Context) {
	req := &pb.GetUserConversationIDsHashReq{OwnerUserId: c.Query("owner_user_id")}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetUserConversationIDsHash(ctx, req)
	recordGRPC("conversation", "GetUserConversationIDsHash", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetOwnerConversation GET /conversations/owner?user_id=&offset=0&limit=20
func (h *ConversationHandler) GetOwnerConversation(c *gin.Context) {
	req := &pb.GetOwnerConversationReq{
		UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c)),
		Offset: parseInt32(c.Query("offset"), 0),
		Limit:  parseInt32(c.Query("limit"), 20),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetOwnerConversation(ctx, req)
	recordGRPC("conversation", "GetOwnerConversation", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// ClearUserConversationMsg POST /conversations/clear-msg
func (h *ConversationHandler) ClearUserConversationMsg(c *gin.Context) {
	var req pb.ClearUserConversationMsgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.ClearUserConversationMsg(ctx, &req)
	recordGRPC("conversation", "ClearUserConversationMsg", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetConversationsNeedClearMsg GET /conversations/need-clear
func (h *ConversationHandler) GetConversationsNeedClearMsg(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetConversationsNeedClearMsg(ctx, &pb.GetConversationsNeedClearMsgReq{})
	recordGRPC("conversation", "GetConversationsNeedClearMsg", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetFullOwnerConversationIDs GET /conversations/full-ids?user_id=
func (h *ConversationHandler) GetFullOwnerConversationIDs(c *gin.Context) {
	req := &pb.GetFullOwnerConversationIDsReq{UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c))}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetFullOwnerConversationIDs(ctx, req)
	recordGRPC("conversation", "GetFullOwnerConversationIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetIncrementalConversation GET /conversations/incremental?user_id=
func (h *ConversationHandler) GetIncrementalConversation(c *gin.Context) {
	req := &pb.GetIncrementalConversationReq{UserId: firstNonEmpty(c.Query("user_id"), userIDFromCtx(c))}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetIncrementalConversation(ctx, req)
	recordGRPC("conversation", "GetIncrementalConversation", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
