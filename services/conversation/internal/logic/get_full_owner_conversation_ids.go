package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

func (s *Server) GetFullOwnerConversationIDs(ctx context.Context, req *conversationpb.GetFullOwnerConversationIDsReq) (*conversationpb.GetFullOwnerConversationIDsResp, error) {
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	head, err := s.store.Version(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversation version")
	}
	ids, err := s.store.ConversationIDs(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("list conversation ids")
	}
	sum := idhash.Hash(ids)
	resp := &conversationpb.GetFullOwnerConversationIDsResp{
		Version:         head.Version,
		VersionId:       head.ID,
		Equal:           req.GetIdHash() == sum,
		ConversationIds: ids,
	}
	if resp.Equal {
		resp.ConversationIds = nil
	}
	return resp, nil
}
