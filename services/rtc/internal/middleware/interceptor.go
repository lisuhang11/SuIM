// Package middleware 提供 gRPC 拦截器，处理鉴权、请求 ID、panic 恢复和请求日志。
package middleware

import (
	"context"
	"log/slog"
	"runtime/debug"
	"strings"
	"time"

	"rtc/internal/logger"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userIDContextKey struct{}

// UserIDFromContext 返回鉴权后的用户 ID。
func UserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(userIDContextKey{}).(string)
	return userID, ok && userID != ""
}

// UnaryServerInterceptor 要求 Bearer JWT，并将 user_id 写入 context。
func UnaryServerInterceptor(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		requestID := uuid.New().String()
		ctx = logger.WithRequestID(ctx, requestID)

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
		}
		values := md.Get("authorization")
		if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization token")
		}
		raw := strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
		if raw == "" {
			return nil, status.Error(codes.Unauthenticated, "empty authorization token")
		}
		userID, authErr := parseUserID(raw, jwtSecret)
		if authErr != nil || userID == "" {
			logger.Warn(ctx, "gRPC authentication failed", "method", info.FullMethod, "error", authErr)
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		ctx = context.WithValue(ctx, userIDContextKey{}, userID)

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

func parseUserID(tokenStr, secret string) (string, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "invalid token claims")
	}
	uid, _ := claims["user_id"].(string)
	if uid == "" {
		uid, _ = claims["sub"].(string)
	}
	return uid, nil
}
