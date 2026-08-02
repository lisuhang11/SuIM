// Package middleware 提供 msggateway gRPC 鉴权拦截器。
package middleware

import (
	"context"
	"strings"

	"msggateway/internal/auth"

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

// UnaryJWT 要求 Bearer access JWT（与 WS 鉴权共用密钥）。
func UnaryJWT(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "missing authorization metadata")
		}
		values := md.Get("authorization")
		if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "missing or invalid authorization token")
		}
		raw := strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
		userID, _, err := auth.ParseAccessToken(raw, jwtSecret)
		if err != nil || userID == "" {
			return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
		}
		return handler(context.WithValue(ctx, userIDContextKey{}, userID), req)
	}
}
