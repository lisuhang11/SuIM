package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
)

func actorID(ctx context.Context, claimed string) (string, error) {
	id, err := rpcctx.Actor(ctx, claimed)
	if err != nil {
		return "", err
	}
	return normalizeUserID(id)
}
