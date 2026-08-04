// Package handler 提供 REST → gRPC 转发层，统一错误响应格式。
package handler

import (
	"context"
	"net/http"
	"time"

	"apigateway/internal/middleware"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// authenticatedGRPCContext 将已由网关验证的原始令牌转发给后端服务再次校验。
func authenticatedGRPCContext(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	token := middleware.GetToken(c)
	if token == "" {
		return ctx
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+token)
}

// gRPCError 将 gRPC status error 映射为 HTTP 状态码和可读消息。
type gRPCError struct {
	HTTPStatus int    `json:"-"`
	Code       int    `json:"code"`
	Message    string `json:"message"`
}

func mapGRPCError(err error) *gRPCError {
	s, ok := status.FromError(err)
	if !ok {
		return &gRPCError{HTTPStatus: http.StatusInternalServerError, Code: 500, Message: err.Error()}
	}
	httpStatus := http.StatusInternalServerError
	switch s.Code() {
	case codes.InvalidArgument:
		httpStatus = http.StatusBadRequest
	case codes.NotFound:
		httpStatus = http.StatusNotFound
	case codes.AlreadyExists:
		httpStatus = http.StatusConflict
	case codes.PermissionDenied:
		httpStatus = http.StatusForbidden
	case codes.Unauthenticated:
		httpStatus = http.StatusUnauthorized
	case codes.Unavailable:
		httpStatus = http.StatusServiceUnavailable
	case codes.DeadlineExceeded:
		httpStatus = http.StatusGatewayTimeout
	default:
		httpStatus = http.StatusInternalServerError
	}
	return &gRPCError{
		HTTPStatus: httpStatus,
		Code:       int(s.Code()),
		Message:    s.Message(),
	}
}

// Respond 成功时返回 200 + data。
func Respond(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "ok",
		"data":    data,
	})
}

// RespondError 返回统一错误格式。
func RespondError(c *gin.Context, err error) {
	ge := mapGRPCError(err)
	c.JSON(ge.HTTPStatus, gin.H{
		"code":    ge.Code,
		"message": ge.Message,
	})
}

// recordGRPC 记录 gRPC 调用指标。
func recordGRPC(service, method string, err error, start time.Time) {
	middleware.RecordGRPCCall(service, method, err, start)
}

// userIDFromCtx 从 gin.Context 提取已认证的 user_id，未登录返回空。
func userIDFromCtx(c *gin.Context) string {
	return middleware.GetUserID(c)
}
