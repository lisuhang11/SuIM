// Package rpcctx carries the authenticated user between the gateway and a gRPC service.
//
// The gateway reads the user from the login token and stores it on the context.
// The client interceptor copies it into outgoing metadata. The server interceptor
// copies that metadata back onto the handler context. A service then treats this
// user as the operator. The metadata value is not a token and is not checked again.
package rpcctx

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
)

const userKey = "opuserid"

type ctxKey struct{}

// WithUser returns a context that carries userID as the operator.
func WithUser(ctx context.Context, userID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, ctxKey{}, userID)
}

// Actor is the operator for this call.
// A context user wins. A non-empty claimed id must be that user, and an empty
// claimed id is filled from the context. An empty context returns claimed so
// in-process tests can keep passing the operator in the request body. The gRPC
// server interceptor rejects a call that arrives without a user.
func Actor(ctx context.Context, claimed string) (string, error) {
	claimed = strings.TrimSpace(claimed)
	op := strings.TrimSpace(UserID(ctx))
	if op == "" {
		return claimed, nil
	}
	if claimed != "" && claimed != op {
		return "", errorcode.ErrNoPermission
	}
	if claimed != "" {
		return claimed, nil
	}
	return op, nil
}

// OneOf requires the context user to be one of ids.
// An empty context allows the call so in-process tests keep working.
func OneOf(ctx context.Context, ids ...string) error {
	op := strings.TrimSpace(UserID(ctx))
	if op == "" {
		return nil
	}
	for _, id := range ids {
		if strings.TrimSpace(id) == op {
			return nil
		}
	}
	return errorcode.ErrNoPermission
}

// UserID returns the operator stored by WithUser or by the server interceptor.
func UserID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	id, _ := ctx.Value(ctxKey{}).(string)
	return id
}

// ClientInterceptor copies the context user into outgoing metadata.
func ClientInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if userID := UserID(ctx); userID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, userKey, userID)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

// ServerInterceptor requires an operator in incoming metadata and stores it on the context.
// A call without one is rejected. The user id is taken from the caller, the same way
// OpenIM trusts metadata after the gateway has checked the token.
func ServerInterceptor(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	userID := incomingUser(ctx)
	if userID == "" {
		return nil, errorcode.ErrNoPermission
	}
	return handler(WithUser(ctx, userID), req)
}

func incomingUser(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ""
	}
	values := md.Get(userKey)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
