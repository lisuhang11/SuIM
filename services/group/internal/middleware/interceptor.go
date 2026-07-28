// Package middleware 提供 gRPC 拦截器，处理请求 ID 注入、panic 恢复和请求日志等横切关注点。
package middleware

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"group/internal/logger"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userIDContextKey struct{}

// Authenticator 验证调用方提供的访问令牌。
type Authenticator interface {
	Authenticate(ctx context.Context, token string) (string, error)
}

// UserIDFromContext 返回通过拦截器认证的调用方身份。
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey{}).(string)
	return userID, ok && userID != ""
}

// UnaryServerInterceptor 返回 gRPC 一元拦截器链，提供：
// 1) 请求 ID 注入与传播  2) panic 恢复  3) 请求日志记录。
func UnaryServerInterceptor(auth Authenticator) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		// 1. 注入或传播 request-id。
		requestID := uuid.New().String()
		ctx = logger.WithRequestID(ctx, requestID)

		// 2. 验证网关转发的 Bearer token，并将可信用户身份写入 context。
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
		}
		values := md.Get("authorization")
		if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization token")
		}
		token := strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
		if token == "" {
			return nil, status.Error(codes.Unauthenticated, "empty authorization token")
		}
		userID, authErr := auth.Authenticate(ctx, token)
		if authErr != nil || userID == "" {
			logger.Warn(ctx, "gRPC authentication failed", "method", info.FullMethod, "error", authErr)
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		ctx = context.WithValue(ctx, userIDContextKey{}, userID)

		// 3. panic 恢复，防止单个请求崩溃导致整个服务宕机。
		defer func() {
			if r := recover(); r != nil {
				slog.ErrorContext(ctx, "panic recovered",
					"method", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Errorf(codes.Internal, "internal server error")
			}
		}()

		// 4. 请求日志记录，包含方法名和耗时。
		start := time.Now()
		logger.Info(ctx, "gRPC request started", "method", info.FullMethod)

		resp, err = handler(ctx, req)

		latency := time.Since(start)
		code := status.Code(err)

		if err != nil {
			logger.Error(ctx, "gRPC request completed with error",
				"method", info.FullMethod,
				"code", code.String(),
				"latency_ms", latency.Milliseconds(),
			)
		} else {
			logger.Info(ctx, "gRPC request completed",
				"method", info.FullMethod,
				"code", code.String(),
				"latency_ms", latency.Milliseconds(),
			)
		}

		return resp, err
	}
}
