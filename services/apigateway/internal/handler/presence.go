package handler

import (
	"context"
	"net/http"
	"time"

	pb "SuIM/proto/msggatewaypb"

	"github.com/gin-gonic/gin"
)

// PresenceHandler 处理在线状态 REST（转发 msggateway）。
type PresenceHandler struct {
	client pb.MsgGatewayClient
}

// NewPresenceHandler 创建 presence handler。
func NewPresenceHandler(client pb.MsgGatewayClient) *PresenceHandler {
	return &PresenceHandler{client: client}
}

// RegisterRoutes 注册到 /users 组：POST /online-status。
func (h *PresenceHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.POST("/online-status", h.GetOnlineStatus)
}

type onlineStatusReq struct {
	UserIDs []string `json:"user_ids"`
}

type onlineStatusItem struct {
	UserID      string  `json:"user_id"`
	Status      int32   `json:"status"`
	PlatformIDs []int32 `json:"platform_ids,omitempty"`
}

// GetOnlineStatus POST /users/online-status
func (h *PresenceHandler) GetOnlineStatus(c *gin.Context) {
	if h.client == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "msggateway unavailable"})
		return
	}
	var req onlineStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if len(req.UserIDs) == 0 {
		Respond(c, gin.H{"statuses": []onlineStatusItem{}})
		return
	}

	ctx, cancel := context.WithTimeout(authenticatedGRPCContext(c), 3*time.Second)
	defer cancel()

	resp, err := h.client.GetUsersOnlineStatus(ctx, &pb.GetUsersOnlineStatusReq{UserIds: req.UserIDs})
	if err != nil {
		RespondError(c, err)
		return
	}

	byID := make(map[string]onlineStatusItem, len(req.UserIDs))
	for _, id := range req.UserIDs {
		byID[id] = onlineStatusItem{UserID: id, Status: 0}
	}
	for _, s := range resp.GetSuccessResult() {
		plats := make([]int32, 0, len(s.GetDetailPlatformStatus()))
		seen := map[int32]struct{}{}
		for _, d := range s.GetDetailPlatformStatus() {
			pid := d.GetPlatformId()
			if _, ok := seen[pid]; ok {
				continue
			}
			seen[pid] = struct{}{}
			plats = append(plats, pid)
		}
		byID[s.GetUserId()] = onlineStatusItem{
			UserID:      s.GetUserId(),
			Status:      s.GetStatus(),
			PlatformIDs: plats,
		}
	}

	out := make([]onlineStatusItem, 0, len(req.UserIDs))
	for _, id := range req.UserIDs {
		out = append(out, byID[id])
	}
	Respond(c, gin.H{"statuses": out})
}
