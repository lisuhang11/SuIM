package handler

import (
	"context"
	"net/http"
	"time"

	pb "SuIM/proto/rtcpb"

	"github.com/gin-gonic/gin"
)

// CallHandler 将 /api/v1/calls REST 转发到 rtc gRPC。
type CallHandler struct {
	client pb.RtcServiceClient
}

func NewCallHandler(client pb.RtcServiceClient) *CallHandler {
	return &CallHandler{client: client}
}

func (h *CallHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/invite", h.Invite)
	r.POST("/:call_id/accept", h.Accept)
	r.POST("/:call_id/reject", h.Reject)
	r.POST("/:call_id/cancel", h.Cancel)
	r.POST("/:call_id/hangup", h.Hangup)
	r.GET("/:call_id", h.GetCall)
	r.POST("/:call_id/token", h.RefreshToken)
}

func (h *CallHandler) Invite(c *gin.Context) {
	var body struct {
		CalleeID  string `json:"callee_id" binding:"required"`
		MediaType string `json:"media_type"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 10*time.Second)
	defer cancel()
	resp, err := h.client.Invite(ctx, &pb.InviteReq{
		CallerId:  userIDFromCtx(c),
		CalleeId:  body.CalleeID,
		MediaType: body.MediaType,
	})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *CallHandler) Accept(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 10*time.Second)
	defer cancel()
	resp, err := h.client.Accept(ctx, &pb.AcceptReq{
		UserId: userIDFromCtx(c),
		CallId: c.Param("call_id"),
	})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *CallHandler) Reject(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	resp, err := h.client.Reject(ctx, &pb.RejectReq{
		UserId: userIDFromCtx(c),
		CallId: c.Param("call_id"),
	})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *CallHandler) Cancel(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	resp, err := h.client.Cancel(ctx, &pb.CancelReq{
		UserId: userIDFromCtx(c),
		CallId: c.Param("call_id"),
	})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *CallHandler) Hangup(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	resp, err := h.client.Hangup(ctx, &pb.HangupReq{
		UserId: userIDFromCtx(c),
		CallId: c.Param("call_id"),
	})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *CallHandler) GetCall(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()
	resp, err := h.client.GetCall(ctx, &pb.GetCallReq{
		UserId: userIDFromCtx(c),
		CallId: c.Param("call_id"),
	})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}

func (h *CallHandler) RefreshToken(c *gin.Context) {
	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 5*time.Second)
	defer cancel()
	resp, err := h.client.RefreshToken(ctx, &pb.RefreshTokenReq{
		UserId: userIDFromCtx(c),
		CallId: c.Param("call_id"),
	})
	if err != nil {
		RespondError(c, err)
		return
	}
	Respond(c, resp)
}
