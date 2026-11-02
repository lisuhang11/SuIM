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
	id = strings.TrimSpace(id)
	if id == "" {
		return "", errorcode.ErrMsgInvalidArgs.WithMsg("user id is required")
	}
	return id, nil
}
