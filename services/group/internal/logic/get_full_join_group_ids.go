package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
)

// 返回调用者已加入的全部群 ID 和当前版本。
func (s *Server) GetFullJoinGroupIDs(ctx context.Context, req *grouppb.GetFullJoinGroupIDsReq) (*grouppb.GetFullJoinGroupIDsResp, error) {
	userID, err := bindActor(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	head, err := s.store.JoinVersion(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query join version")
	}
	ids, err := s.store.AllJoinedGroupIDs(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list joined group ids")
	}
	sum := idhash.Hash(ids)
	resp := &grouppb.GetFullJoinGroupIDsResp{
		Version:   head.Version,
		VersionId: head.ID,
		Equal:     req.GetIdHash() == sum,
		GroupIds:  ids,
	}
	if resp.Equal {
		resp.GroupIds = nil
	}
	return resp, nil
}
