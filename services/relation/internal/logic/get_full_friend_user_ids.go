package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
)

// 返回调用者的全部好友 ID 和当前版本。版本没变时不必再拉资料。
func (s *Server) GetFullFriendUserIDs(ctx context.Context, req *relationpb.GetFullFriendUserIDsReq) (*relationpb.GetFullFriendUserIDsResp, error) {
	// 1. 校验用户。
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	// 2. 读取版本和有序 ID。哈希相同则不回传 ID。
	head, err := s.friendHead(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query friend version")
	}
	ids, err := s.friendIDs(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list friend ids")
	}
	sum := idhash.Hash(ids)
	resp := &relationpb.GetFullFriendUserIDsResp{
		Version:   head.Version,
		VersionId: head.ID,
		Equal:     req.GetIdHash() == sum,
		UserIds:   ids,
	}
	if resp.Equal {
		resp.UserIds = nil
	}
	return resp, nil
}
