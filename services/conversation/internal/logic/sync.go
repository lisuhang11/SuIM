package logic

import (
	"context"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/idhash"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
)

const syncLimit = 200

func (s *Server) GetFullOwnerConversationIDs(ctx context.Context, req *conversationpb.GetFullOwnerConversationIDsReq) (*conversationpb.GetFullOwnerConversationIDsResp, error) {
	userID, err := normalizeUserID(req.GetUserId())
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

func (s *Server) GetIncrementalConversation(ctx context.Context, req *conversationpb.GetIncrementalConversationReq) (*conversationpb.GetIncrementalConversationResp, error) {
	userID, err := normalizeUserID(req.GetUserId())
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

func needFull(head model.VersionHead, versionID string, version uint64) bool {
	return version == 0 || versionID == "" || head.ID == "" || head.ID != versionID
}

func splitLogs(logs []model.VersionLog) (insertIDs, updateIDs, deleteIDs []string) {
	for _, log := range logs {
		switch log.State {
		case model.VersionInsert:
			insertIDs = append(insertIDs, log.ElemID)
		case model.VersionUpdate:
			updateIDs = append(updateIDs, log.ElemID)
		case model.VersionDelete:
			deleteIDs = append(deleteIDs, log.ElemID)
		}
	}
	return insertIDs, updateIDs, deleteIDs
}

func (s *Server) conversationsByID(ctx context.Context, owner string, ids []string) ([]*conversationpb.Conversation, error) {
	rows, err := s.store.ConversationsByID(ctx, owner, ids)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("query conversations")
	}
	out := make([]*conversationpb.Conversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, conversationPB(row))
	}
	return out, nil
}

func conversationPB(row *model.Conversation) *conversationpb.Conversation {
	if row == nil {
		return nil
	}
	var latest int64
	if !row.LatestMsgDestructTime.IsZero() {
		latest = row.LatestMsgDestructTime.UnixMilli()
	}
	return &conversationpb.Conversation{
		OwnerUserId:           row.OwnerUserID,
		ConversationId:        row.ConversationID,
		ConversationType:      row.ConversationType,
		UserId:                row.UserID,
		GroupId:               row.GroupID,
		RecvMsgOpt:            row.RecvMsgOpt,
		IsPinned:              row.IsPinned,
		AttachedInfo:          row.AttachedInfo,
		IsPrivateChat:         row.IsPrivateChat,
		GroupAtType:           row.GroupAtType,
		Ex:                    row.Ex,
		BurnDuration:          row.BurnDuration,
		MinSeq:                row.MinSeq,
		MaxSeq:                row.MaxSeq,
		MsgDestructTime:       row.MsgDestructTime,
		LatestMsgDestructTime: latest,
		IsMsgDestruct:         row.IsMsgDestruct,
		CreateTime:            row.CreateTime.UnixMilli(),
	}
}
