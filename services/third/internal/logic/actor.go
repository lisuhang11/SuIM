package logic

import (
	"context"
	"strings"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
)

func actorUser(ctx context.Context, claimed string) (string, error) {
	id, err := rpcctx.Actor(ctx, claimed)
	if err != nil {
		return "", err
	}
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, `\`) {
		return "", errorcode.ErrThirdInvalidArgs.WithMsg("user_id is invalid")
	}
	return id, nil
}
