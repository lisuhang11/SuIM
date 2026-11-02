package logic

import (
	"context"
	"errors"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
)

func (s *Server) BlockUser(ctx context.Context, req *relationpb.BlockUserReq) (*relationpb.BlockUserResp, error) {
	// 1. 校验双方 userID。调用方必须是拉黑者，操作者身份尚未从上下文接入。
	owner, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	block, err := normalizeUserID(req.GetBlockedUserId())
	if err != nil {
		return nil, err
	}
	if owner == block {
		return nil, errorcode.ErrRelationCannotBlockYourself
	}
	// 2. 拉黑前 webhook。回调拒绝则中断。
	if err := s.webhookBeforeAddBlack(ctx, owner, block); err != nil {
		return nil, err
	}
	// 3. 确认双方用户存在。
	if err := s.users.CheckUsers(ctx, []string{owner, block}); err != nil {
		return nil, err
	}
	// 4. 写入黑名单。操作者暂记为拉黑者本人。
	if err := s.store.CreateBlack(ctx, owner, block, owner, s.now()); err != nil {
		if errors.Is(err, store.ErrAlreadyBlocked) {
			return nil, errorcode.ErrRelationAlreadyBlocked
		}
		return nil, errorcode.ErrInternal.WithMsg("add black")
	}
	// 5. 通知对方已被拉黑。
	s.notifyBlackAdded(ctx, owner, block)
	return &relationpb.BlockUserResp{}, nil
}

// rejectIfBlocked stops a friend request or an agreement when either user has blocked the other.
func (s *Server) rejectIfBlocked(ctx context.Context, user1, user2 string) error {
	in1, in2, err := s.store.CheckBlack(ctx, user1, user2)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check black")
	}
	if in1 || in2 {
		return errorcode.ErrRelationBlocked
	}
	return nil
}
