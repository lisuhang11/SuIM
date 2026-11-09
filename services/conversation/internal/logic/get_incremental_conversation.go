package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
)

// 按版本返回调用者的会话变更。版本对不上或差额达到限额时只返回 full。
func (s *Server) GetIncrementalConversation(ctx context.Context, req *conversationpb.GetIncrementalConversationReq) (*conversationpb.GetIncrementalConversationResp, error) {
	userID, err := actorID(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	head, err := s.store.Version(ctx, userID)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversation version")
	}
	if needFull(head, req.GetVersionId(), req.GetVersion()) {
		return &conversationpb.GetIncrementalConversationResp{Version: head.Version, VersionId: head.ID, Full: true}, nil
	}
	if head.Version <= req.GetVersion() {
		return &conversationpb.GetIncrementalConversationResp{Version: head.Version, VersionId: head.ID}, nil
	}
	logs, err := s.store.Changes(ctx, userID, req.GetVersion(), syncLimit)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversation changes")
	}
	if len(logs) >= syncLimit {
		return &conversationpb.GetIncrementalConversationResp{Version: head.Version, VersionId: head.ID, Full: true}, nil
	}
	insertIDs, updateIDs, deleteIDs := splitLogs(logs)
	insert, err := s.conversationsByID(ctx, userID, insertIDs)
	if err != nil {
		return nil, err
	}
	update, err := s.conversationsByID(ctx, userID, updateIDs)
	if err != nil {
		return nil, err
	}
	return &conversationpb.GetIncrementalConversationResp{
		Version:   head.Version,
		VersionId: head.ID,
		Delete:    deleteIDs,
		Insert:    insert,
		Update:    update,
	}, nil
}
