package middleware

import (
	"context"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type userKey struct{}

func UserID(ctx context.Context) string { v, _ := ctx.Value(userKey{}).(string); return v }
func Unary(jwtSecret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, _ := metadata.FromIncomingContext(ctx)
		values := md.Get("authorization")
		if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
			return nil, status.Error(codes.Unauthenticated, "missing bearer token")
		}
		raw := strings.TrimSpace(strings.TrimPrefix(values[0], "Bearer "))
		token, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			return nil, status.Error(codes.Unauthenticated, "invalid bearer token")
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return nil, status.Error(codes.Unauthenticated, "invalid token claims")
		}
		uid, _ := claims["user_id"].(string)
		if uid == "" {
			uid, _ = claims["sub"].(string)
		}
		if uid == "" {
			return nil, status.Error(codes.Unauthenticated, "token missing user identity")
		}
		return handler(context.WithValue(ctx, userKey{}, uid), req)
	}
}
