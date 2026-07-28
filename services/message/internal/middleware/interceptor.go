// Package middleware 提供 gRPC 拦截器，处理请求 ID 注入、panic 恢复和请求日志。
package middleware

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"message/internal/logger"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor 返回 gRPC 一元拦截器，依次执行：
//   1. 注入请求 ID
//   2. panic 恢复
//   3. 请求日志（耗时/状态码）
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		// 1. 注入请求 ID。
		requestID := uuid.New().String()
		ctx = logger.WithRequestID(ctx, requestID)

		// 2. panic 恢复。
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

		// 3. 请求日志。
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
