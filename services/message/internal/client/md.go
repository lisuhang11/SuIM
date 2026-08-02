package client

import (
	"context"

	"google.golang.org/grpc/metadata"
)

// withForwardedMD 将入站 gRPC metadata（含 Bearer）复制到出站 context，
// 供跨服务调用通过下游鉴权拦截器。
func withForwardedMD(ctx context.Context) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return ctx
	}
	return metadata.NewOutgoingContext(ctx, md.Copy())
}
