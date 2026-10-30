package logic

import (
	"strings"
	"unicode/utf8"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	"github.com/lisuhang11/SuIM/services/conversation/internal/model"
	"github.com/lisuhang11/SuIM/services/conversation/internal/store"
)

func normalizeUserID(id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 || strings.Contains(id, ":") {
		return "", errorcode.ErrConversationInvalidUserID
	}
	return id, nil
}

func normalizeUserIDs(ids []string) ([]string, error) {
	if len(ids) == 0 {
		return nil, errorcode.ErrConversationInvalidArgs.WithMsg("user ids are empty")
	}
	out := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, raw := range ids {
		id, err := normalizeUserID(raw)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[id]; ok {
			return nil, errorcode.ErrConversationInvalidArgs.WithMsg("user id is repeated")
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

func conversationSpec(req *conversationpb.ConversationReq) (store.Spec, error) {
	if req == nil {
		return store.Spec{}, errorcode.ErrConversationInvalidArgs.WithMsg("conversation is required")
	}
	id := strings.TrimSpace(req.GetConversationId())
	if id == "" || len(id) > 255 {
		return store.Spec{}, errorcode.ErrConversationInvalidArgs.WithMsg("conversation_id is invalid")
	}
	switch req.GetConversationType() {
	case model.ConversationSingle, model.ConversationNotification:
		peer, err := normalizeUserID(req.GetUserId())
		if err != nil {
			return store.Spec{}, err
		}
		if req.GetGroupId() != "" {
			return store.Spec{}, errorcode.ErrConversationInvalidArgs.WithMsg("group_id must be empty")
		}
		spec, err := specWithPatch(id, req.GetConversationType(), peer, "", req)
		return spec, err
	case model.ConversationGroup, model.ConversationSuperGroup:
		groupID := strings.TrimSpace(req.GetGroupId())
		if groupID == "" || len(groupID) > 64 {
			return store.Spec{}, errorcode.ErrConversationInvalidArgs.WithMsg("group_id is invalid")
		}
		return specWithPatch(id, req.GetConversationType(), strings.TrimSpace(req.GetUserId()), groupID, req)
	default:
		return store.Spec{}, errorcode.ErrConversationInvalidArgs.WithMsg("conversation_type is invalid")
	}
}

func specWithPatch(id string, conversationType int32, userID, groupID string, req *conversationpb.ConversationReq) (store.Spec, error) {
	patch, err := patchFrom(req)
	if err != nil {
		return store.Spec{}, err
	}
	spec := store.Spec{
		ConversationID:   id,
		ConversationType: conversationType,
		UserID:           userID,
		GroupID:          groupID,
		Patch:            patch,
	}
	if req.IsPrivateChat != nil && conversationType == model.ConversationSingle {
		spec.Private = req.IsPrivateChat
	}
	return spec, nil
}

func patchFrom(req *conversationpb.ConversationReq) (model.Patch, error) {
	var patch model.Patch
	if req.RecvMsgOpt != nil {
		switch *req.RecvMsgOpt {
		case model.ReceiveMessage, model.NotReceiveMessage, model.ReceiveNotNotify:
		default:
			return model.Patch{}, errorcode.ErrConversationInvalidArgs.WithMsg("recv_msg_opt is invalid")
		}
		patch.RecvMsgOpt = req.RecvMsgOpt
	}
	if req.IsPinned != nil {
		patch.IsPinned = req.IsPinned
	}
	if req.AttachedInfo != nil {
		if len(*req.AttachedInfo) > 4096 {
			return model.Patch{}, errorcode.ErrConversationInvalidArgs.WithMsg("attached_info is too long")
		}
		patch.AttachedInfo = req.AttachedInfo
	}
	if req.Ex != nil {
		if utf8.RuneCountInString(*req.Ex) > 1024 {
			return model.Patch{}, errorcode.ErrConversationInvalidArgs.WithMsg("ex is too long")
		}
		patch.Ex = req.Ex
	}
	if req.GroupAtType != nil {
		if *req.GroupAtType < 0 || *req.GroupAtType > 4 {
			return model.Patch{}, errorcode.ErrConversationInvalidArgs.WithMsg("group_at_type is invalid")
		}
		patch.GroupAtType = req.GroupAtType
	}
	if req.MsgDestructTime != nil {
		if *req.MsgDestructTime < 0 {
			return model.Patch{}, errorcode.ErrConversationInvalidArgs.WithMsg("msg_destruct_time is invalid")
		}
		patch.MsgDestructTime = req.MsgDestructTime
	}
	if req.IsMsgDestruct != nil {
		patch.IsMsgDestruct = req.IsMsgDestruct
	}
	if req.BurnDuration != nil {
		if *req.BurnDuration < 0 {
			return model.Patch{}, errorcode.ErrConversationInvalidArgs.WithMsg("burn_duration is invalid")
		}
		patch.BurnDuration = req.BurnDuration
	}
	return patch, nil
}
