// Package handler 提供 message 服务的 REST 端点。
package handler

import (
	"context"
	"encoding/json"
	"time"

	pbFile "SuIM/proto/filepb"
	pb "SuIM/proto/messagepb"

	"github.com/gin-gonic/gin"
)

// MessageHandler 处理 /api/v1/messages 路由。
type MessageHandler struct {
	client     pb.MessageClient
	fileClient pbFile.FileServiceClient
}

// NewMessageHandler 创建 message handler。
func NewMessageHandler(client pb.MessageClient, fileClient pbFile.FileServiceClient) *MessageHandler {
	return &MessageHandler{client: client, fileClient: fileClient}
}

// RegisterRoutes 注册路由。
func (h *MessageHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("", h.SendMsg)
	r.GET("/history", h.GetHistoryMessages)
	r.GET("/by-seq", h.GetMessagesBySeq)
	r.GET("/by-client-ids", h.GetMessagesByClientMsgIDs)
	r.POST("/revoke", h.RevokeMsg)
	r.POST("/read", h.MarkMsgsAsRead)
	r.DELETE("", h.DeleteMsgs)
}

// SendMsg POST /messages
func (h *MessageHandler) SendMsg(c *gin.Context) {
	var req pb.SendMsgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	if req.MsgData == nil {
		c.JSON(400, gin.H{"code": 400, "message": "msg_data is required"})
		return
	}
	userID := userIDFromCtx(c)
	req.MsgData.SendId = userID
	if req.MsgData.ContentType == 2 {
		var content struct {
			FileID string `json:"file_id"`
		}
		if err := json.Unmarshal([]byte(req.MsgData.Content), &content); err != nil || content.FileID == "" {
			c.JSON(400, gin.H{"code": 400, "message": "file message content must contain file_id"})
			return
		}
		if h.fileClient == nil {
			c.JSON(503, gin.H{"code": 503, "message": "file service unavailable"})
			return
		}
		ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
		_, err := h.fileClient.BindFile(ctx, &pbFile.BindFileReq{UserId: userID, FileId: content.FileID, ConversationId: req.MsgData.ConversationId})
		cancel()
		if err != nil {
			RespondError(c, err)
			return
		}
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second) // 发消息允许更长超时
	defer cancel()
	start := time.Now()
	resp, err := h.client.SendMsg(ctx, &req)
	recordGRPC("message", "SendMsg", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetHistoryMessages GET /messages/history?conversation_id=&seq=0&limit=20&order=0
func (h *MessageHandler) GetHistoryMessages(c *gin.Context) {
	req := &pb.GetHistoryMessagesReq{
		ConversationId: c.Query("conversation_id"),
		Seq:            parseInt64(c.Query("seq"), 0),
		Limit:          parseInt32(c.Query("limit"), 20),
		Order:          parseInt32(c.Query("order"), 0),
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetHistoryMessages(ctx, req)
	recordGRPC("message", "GetHistoryMessages", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetMessagesBySeq GET /messages/by-seq?conversation_id=&seqs=1,2,3
func (h *MessageHandler) GetMessagesBySeq(c *gin.Context) {
	req := &pb.GetMessagesBySeqReq{
		ConversationId: c.Query("conversation_id"),
		Seqs:           parseInt64Slice(c.Query("seqs")),
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetMessagesBySeq(ctx, req)
	recordGRPC("message", "GetMessagesBySeq", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// GetMessagesByClientMsgIDs GET /messages/by-client-ids?ids=xxx,yyy
func (h *MessageHandler) GetMessagesByClientMsgIDs(c *gin.Context) {
	req := &pb.GetMessagesByClientMsgIDsReq{
		ClientMsgIds: splitComma(c.Query("ids")),
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.GetMessagesByClientMsgIDs(ctx, req)
	recordGRPC("message", "GetMessagesByClientMsgIDs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// RevokeMsg POST /messages/revoke
func (h *MessageHandler) RevokeMsg(c *gin.Context) {
	var req pb.RevokeMsgReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.RevokeMsg(ctx, &req)
	recordGRPC("message", "RevokeMsg", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// MarkMsgsAsRead POST /messages/read
func (h *MessageHandler) MarkMsgsAsRead(c *gin.Context) {
	var req pb.MarkMsgsAsReadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.MarkMsgsAsRead(ctx, &req)
	recordGRPC("message", "MarkMsgsAsRead", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

// DeleteMsgs DELETE /messages
func (h *MessageHandler) DeleteMsgs(c *gin.Context) {
	var req pb.DeleteMsgsReq
	if err := c.ShouldBindJSON(&req); err != nil {
		RespondError(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	start := time.Now()
	resp, err := h.client.DeleteMsgs(ctx, &req)
	recordGRPC("message", "DeleteMsgs", err, start)
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
