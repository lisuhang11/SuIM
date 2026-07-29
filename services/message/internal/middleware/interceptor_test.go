package middleware

import (
	"context"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type testAuthenticator struct{}

func (testAuthenticator) Authenticate(_ context.Context, token string) (string, error) {
	if token == "valid-token" {
		return "user-1", nil
	}
	return "", status.Error(codes.Unauthenticated, "invalid")
}

func TestUnaryServerInterceptorRequiresBearerToken(t *testing.T) {
	interceptor := UnaryServerInterceptor(testAuthenticator{})
	_, err := interceptor(context.Background(), nil, &grpc.UnaryServerInfo{FullMethod: "/message.message/GetHistoryMessages"}, func(context.Context, any) (any, error) {
		t.Fatal("handler must not be called")
		return nil, nil
	})
	if status.Code(err) != codes.Unauthenticated {
		t.Fatalf("code = %v, want Unauthenticated", status.Code(err))
	}
}

func TestUnaryServerInterceptorInjectsAuthenticatedUser(t *testing.T) {
	interceptor := UnaryServerInterceptor(testAuthenticator{})
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs("authorization", "Bearer valid-token"))
	_, err := interceptor(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "/message.message/GetHistoryMessages"}, func(ctx context.Context, _ any) (any, error) {
		userID, ok := UserIDFromContext(ctx)
		if !ok || userID != "user-1" {
			t.Fatalf("user = %q, ok = %v", userID, ok)
		}
		return nil, nil
	})
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}
}
