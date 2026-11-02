package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
)

// bindActor 用调用上下文里的用户作为操作者。
// 上下文里有用户时，请求体里的操作者必须为空或与它相同。
// 上下文为空时沿用请求体，只给进程内测试使用。对外的 gRPC 拦截器会拒绝没有操作者的调用。
func bindActor(ctx context.Context, claimed string) (string, error) {
	id, err := rpcctx.Actor(ctx, claimed)
	if err != nil {
		return "", err
	}
	return normalizeUserID(id)
}
