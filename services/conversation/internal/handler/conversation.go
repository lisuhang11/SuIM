// Package handler 将领域 ConversationService 适配为 gRPC 传输层处理逻辑。
package handler

import (
	"context"
	"time"

	apperrors "conversation/internal/errors"
	"conversation/internal/logger"
	"conversation/internal/middleware"
	"conversation/internal/types"
	"conversation/internal/types/interfaces"

	pb "SuIM/proto/conversationpb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// conversationHandler 实现 pb.ConversationServer，将请求委托给领域 ConversationService。
type conversationHandler struct {
	pb.UnimplementedConversationServer
	svc interfaces.ConversationService
}

// NewConversationHandler 创建绑定到指定领域服务的 gRPC ConversationServer。
func NewConversationHandler(svc interfaces.ConversationService) pb.ConversationServer {
	return &conversationHandler{svc: svc}
}

func authenticatedUserID(ctx context.Context) (string, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authenticated user is missing")
	}
	return userID, nil
}

func userInList(userID string, ids []string) bool {
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}

// --------------- 错误转换 ---------------

// appErrorToStatus 将 AppError 映射为 gRPC status 错误。
func appErrorToStatus(err error) error {
	ae := apperrors.GetAppError(err)
	if ae == nil {
		return status.Error(codes.Internal, err.Error())
	}
	var code codes.Code
	switch ae.Code {
	case apperrors.CodeValidation:
		code = codes.InvalidArgument
	case apperrors.CodeConversationNotFound:
		code = codes.NotFound
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}

// --------------- 类型转换辅助函数 ---------------

// conversationToProto 将领域 Conversation 转换为 proto Conversation。
func conversationToProto(c *types.Conversation) *pb.Conversation {
	if c == nil {
		return nil
	}
	var ct int64
	if !c.CreateTime.IsZero() {
		ct = c.CreateTime.UnixMilli()
	}
	return &pb.Conversation{
		OwnerUserId:           c.OwnerUserID,
		ConversationId:        c.ConversationID,
		ConversationType:      int32(c.ConversationType),
		UserId:                c.UserID,
		GroupId:               c.GroupID,
		RecvMsgOpt:            int32(c.RecvMsgOpt),
		IsPinned:              c.IsPinned,
		AttachedInfo:          c.AttachedInfo,
		IsPrivateChat:         c.IsPrivateChat,
		GroupAtType:           int32(c.GroupAtType),
		Ex:                    c.Ex,
		BurnDuration:          int32(c.BurnDuration),
		MinSeq:                c.MinSeq,
		MaxSeq:                c.MaxSeq,
		MsgDestructTime:       c.MsgDestructTime,
		LatestMsgDestructTime: c.LatestMsgDestructTime,
		IsMsgDestruct:         c.IsMsgDestruct,
		CreateTime:            ct,
	}
}

// protoToConversation 将 proto Conversation 转换为领域 Conversation。
func protoToConversation(c *pb.Conversation) *types.Conversation {
	if c == nil {
		return nil
	}
	conv := &types.Conversation{
		OwnerUserID:           c.OwnerUserId,
		ConversationID:        c.ConversationId,
		ConversationType:      int(c.ConversationType),
		UserID:                c.UserId,
		GroupID:               c.GroupId,
		RecvMsgOpt:            int(c.RecvMsgOpt),
		IsPinned:              c.IsPinned,
		AttachedInfo:          c.AttachedInfo,
		IsPrivateChat:         c.IsPrivateChat,
		GroupAtType:           int(c.GroupAtType),
		Ex:                    c.Ex,
		BurnDuration:          int(c.BurnDuration),
		MinSeq:                c.MinSeq,
		MaxSeq:                c.MaxSeq,
		MsgDestructTime:       c.MsgDestructTime,
		LatestMsgDestructTime: c.LatestMsgDestructTime,
		IsMsgDestruct:         c.IsMsgDestruct,
	}
	if c.CreateTime != 0 {
		conv.CreateTime = time.UnixMilli(c.CreateTime)
	}
	return conv
}

// conversationReqToConversation 将 ConversationReq 转为 Upsert 用的领域模型（SetConversations）。
func conversationReqToConversation(req *pb.ConversationReq) *types.Conversation {
	if req == nil {
		return nil
	}
	c := &types.Conversation{
		ConversationID:   req.ConversationId,
		ConversationType: int(req.ConversationType),
		UserID:           req.UserId,
		GroupID:          req.GroupId,
	}
	if req.RecvMsgOpt != nil {
		c.RecvMsgOpt = int(*req.RecvMsgOpt)
	}
	if req.IsPinned != nil {
		c.IsPinned = *req.IsPinned
	}
	if req.AttachedInfo != nil {
		c.AttachedInfo = *req.AttachedInfo
	}
	if req.IsPrivateChat != nil {
		c.IsPrivateChat = *req.IsPrivateChat
	}
	if req.Ex != nil {
		c.Ex = *req.Ex
	}
	if req.BurnDuration != nil {
		c.BurnDuration = int(*req.BurnDuration)
	}
	if req.MinSeq != nil {
		c.MinSeq = *req.MinSeq
	}
	if req.MaxSeq != nil {
		c.MaxSeq = *req.MaxSeq
	}
	if req.GroupAtType != nil {
		c.GroupAtType = int(*req.GroupAtType)
	}
	if req.MsgDestructTime != nil {
		c.MsgDestructTime = *req.MsgDestructTime
	}
	if req.IsMsgDestruct != nil {
		c.IsMsgDestruct = *req.IsMsgDestruct
	}
	if req.LatestMsgDestructTime != nil {
		c.LatestMsgDestructTime = *req.LatestMsgDestructTime
	}
	return c
}

func latestMsgToProto(m *types.LatestMsg) *pb.MsgInfo {
	if m == nil {
		return nil
	}
	return &pb.MsgInfo{
		ServerMsgId:       m.ServerMsgID,
		ClientMsgId:       m.ClientMsgID,
		SessionType:       int32(m.SessionType),
		SendId:            m.SendID,
		RecvId:            m.RecvID,
		SenderName:        m.SenderNickname,
		FaceUrl:           m.SenderFaceURL,
		GroupId:           m.GroupID,
		LatestMsgRecvTime: m.SendTime,
		MsgFrom:           int32(m.MsgFrom),
		ContentType:       int32(m.ContentType),
		Content:           m.Content,
		Ex:                m.Ex,
	}
}

// conversationReqToUpdate 将 proto ConversationReq 转换为 ConversationUpdate，nil 指针表示不修改。
func conversationReqToUpdate(req *pb.ConversationReq) *types.ConversationUpdate {
	if req == nil {
		return nil
	}
	u := &types.ConversationUpdate{}
	if req.RecvMsgOpt != nil {
		v := int(*req.RecvMsgOpt)
		u.RecvMsgOpt = &v
	}
	if req.IsPinned != nil {
		v := *req.IsPinned
		u.IsPinned = &v
	}
	if req.AttachedInfo != nil {
		v := *req.AttachedInfo
		u.AttachedInfo = &v
	}
	if req.IsPrivateChat != nil {
		v := *req.IsPrivateChat
		u.IsPrivateChat = &v
	}
	if req.Ex != nil {
		v := *req.Ex
		u.Ex = &v
	}
	if req.BurnDuration != nil {
		v := int(*req.BurnDuration)
		u.BurnDuration = &v
	}
	if req.MinSeq != nil {
		v := *req.MinSeq
		u.MinSeq = &v
	}
	if req.MaxSeq != nil {
		v := *req.MaxSeq
		u.MaxSeq = &v
	}
	if req.GroupAtType != nil {
		v := int(*req.GroupAtType)
		u.GroupAtType = &v
	}
	if req.MsgDestructTime != nil {
		v := *req.MsgDestructTime
		u.MsgDestructTime = &v
	}
	if req.IsMsgDestruct != nil {
		v := *req.IsMsgDestruct
		u.IsMsgDestruct = &v
	}
	if req.LatestMsgDestructTime != nil {
		v := *req.LatestMsgDestructTime
		u.LatestMsgDestructTime = &v
	}
	return u
}

// --------------- RPC 实现 ---------------

// SetConversation 设置或更新单个会话。
func (h *conversationHandler) SetConversation(ctx context.Context, req *pb.SetConversationReq) (*pb.SetConversationResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if req.Conversation != nil {
		req.Conversation.OwnerUserId = userID
	}
	if err := h.svc.SetConversation(ctx, protoToConversation(req.Conversation)); err != nil {
		logger.Error(ctx, "set conversation failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SetConversationResp{}, nil
}

// GetConversation 获取指定用户的指定会话。
func (h *conversationHandler) GetConversation(ctx context.Context, req *pb.GetConversationReq) (*pb.GetConversationResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	conv, err := h.svc.GetConversation(ctx, userID, req.ConversationId)
	if err != nil {
		logger.Error(ctx, "get conversation failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.GetConversationResp{Conversation: conversationToProto(conv)}, nil
}

// GetConversations 批量获取会话列表。
func (h *conversationHandler) GetConversations(ctx context.Context, req *pb.GetConversationsReq) (*pb.GetConversationsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	convs, err := h.svc.GetConversations(ctx, userID, req.ConversationIds)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetConversationsResp{Conversations: toProtoSlice(convs)}, nil
}

// GetAllConversations 获取用户所有会话。
func (h *conversationHandler) GetAllConversations(ctx context.Context, req *pb.GetAllConversationsReq) (*pb.GetAllConversationsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	convs, err := h.svc.GetAllConversations(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetAllConversationsResp{Conversations: toProtoSlice(convs)}, nil
}

// GetSortedConversationList 获取排序后的会话列表（含未读数与最后一条消息）。
func (h *conversationHandler) GetSortedConversationList(ctx context.Context, req *pb.GetSortedConversationListReq) (*pb.GetSortedConversationListResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	convs, total, err := h.svc.GetSortedConversationList(ctx, userID, req.ConversationIds, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	ids := make([]string, 0, len(convs))
	for _, c := range convs {
		ids = append(ids, c.ConversationID)
	}
	unreads, _ := h.svc.ListUnreadCounts(ctx, userID, ids)
	latest, _ := h.svc.ListLatestMsgs(ctx, userID, ids)

	elems := make([]*pb.ConversationElem, 0, len(convs))
	var unreadTotal int64
	for _, c := range convs {
		unread := unreads[c.ConversationID]
		// seq_user 缺失时回退 conversation.max_seq - min_seq
		if _, ok := unreads[c.ConversationID]; !ok {
			unread = c.MaxSeq - c.MinSeq
			if unread < 0 {
				unread = 0
			}
		}
		unreadTotal += unread
		elem := &pb.ConversationElem{
			ConversationId: c.ConversationID,
			RecvMsgOpt:     int32(c.RecvMsgOpt),
			UnreadCount:    unread,
			IsPinned:       c.IsPinned,
		}
		if m, ok := latest[c.ConversationID]; ok {
			elem.MsgInfo = latestMsgToProto(&m)
		}
		elems = append(elems, elem)
	}
	return &pb.GetSortedConversationListResp{
		Total:             total,
		UnreadTotal:       unreadTotal,
		ConversationElems: elems,
	}, nil
}

// CreateSingleChatConversations 创建单聊会话（双向）。
func (h *conversationHandler) CreateSingleChatConversations(ctx context.Context, req *pb.CreateSingleChatConversationsReq) (*pb.CreateSingleChatConversationsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.CreateSingleChatConversations(ctx, userID, req.RecvId, req.ConversationId, int(req.ConversationType)); err != nil {
		logger.Error(ctx, "create single chat conversation failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.CreateSingleChatConversationsResp{}, nil
}

// CreateGroupChatConversations 创建群聊会话。
func (h *conversationHandler) CreateGroupChatConversations(ctx context.Context, req *pb.CreateGroupChatConversationsReq) (*pb.CreateGroupChatConversationsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if !userInList(userID, req.UserIds) {
		return nil, status.Error(codes.PermissionDenied, "caller must be included in user_ids")
	}
	if err := h.svc.CreateGroupChatConversations(ctx, req.GroupId, req.UserIds); err != nil {
		logger.Error(ctx, "create group chat conversation failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.CreateGroupChatConversationsResp{}, nil
}

// SetConversationMaxSeq 设置会话最大序列号（已读位置）。
func (h *conversationHandler) SetConversationMaxSeq(ctx context.Context, req *pb.SetConversationMaxSeqReq) (*pb.SetConversationMaxSeqResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.SetConversationMaxSeq(ctx, req.ConversationId, []string{userID}, req.MaxSeq); err != nil {
		logger.Error(ctx, "set max seq failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SetConversationMaxSeqResp{}, nil
}

// SetConversationMinSeq 设置会话最小序列号。
func (h *conversationHandler) SetConversationMinSeq(ctx context.Context, req *pb.SetConversationMinSeqReq) (*pb.SetConversationMinSeqResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.SetConversationMinSeq(ctx, req.ConversationId, []string{userID}, req.MinSeq); err != nil {
		logger.Error(ctx, "set min seq failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SetConversationMinSeqResp{}, nil
}

// GetConversationIDs 获取用户的所有会话 ID。
func (h *conversationHandler) GetConversationIDs(ctx context.Context, req *pb.GetConversationIDsReq) (*pb.GetConversationIDsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := h.svc.GetConversationIDs(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetConversationIDsResp{ConversationIds: ids}, nil
}

// SetConversations 为多个用户批量设置同一会话（ConversationReq 部分字段）。
func (h *conversationHandler) SetConversations(ctx context.Context, req *pb.SetConversationsReq) (*pb.SetConversationsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.SetConversations(ctx, []string{userID}, conversationReqToConversation(req.Conversation)); err != nil {
		logger.Error(ctx, "set conversations failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SetConversationsResp{}, nil
}

// UpdateConversation 更新会话的可选字段。
func (h *conversationHandler) UpdateConversation(ctx context.Context, req *pb.UpdateConversationReq) (*pb.UpdateConversationResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.UpdateConversation(ctx, req.ConversationId, []string{userID}, conversationReqToUpdate(req.Conversation)); err != nil {
		logger.Error(ctx, "update conversation failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.UpdateConversationResp{}, nil
}

// GetConversationsByConversationID 根据会话 ID 查询所有用户的会话记录。
func (h *conversationHandler) GetConversationsByConversationID(ctx context.Context, req *pb.GetConversationsByConversationIDReq) (*pb.GetConversationsByConversationIDResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	convs, err := h.svc.GetConversationsByConversationID(ctx, req.ConversationIds)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	filtered := make([]types.Conversation, 0, len(convs))
	for _, c := range convs {
		if c.OwnerUserID == userID {
			filtered = append(filtered, c)
		}
	}
	return &pb.GetConversationsByConversationIDResp{Conversations: toProtoSlice(filtered)}, nil
}

// GetRecvMsgNotNotifyUserIDs 获取群内设置为不接受通知的用户 ID。
func (h *conversationHandler) GetRecvMsgNotNotifyUserIDs(ctx context.Context, req *pb.GetRecvMsgNotNotifyUserIDsReq) (*pb.GetRecvMsgNotNotifyUserIDsResp, error) {
	ids, err := h.svc.GetRecvMsgNotNotifyUserIDs(ctx, req.GroupId)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetRecvMsgNotNotifyUserIDsResp{UserIds: ids}, nil
}

// GetConversationOfflinePushUserIDs 获取需要离线推送的用户 ID。
func (h *conversationHandler) GetConversationOfflinePushUserIDs(ctx context.Context, req *pb.GetConversationOfflinePushUserIDsReq) (*pb.GetConversationOfflinePushUserIDsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := h.svc.GetConversationOfflinePushUserIDs(ctx, req.ConversationId, []string{userID})
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetConversationOfflinePushUserIDsResp{UserIds: ids}, nil
}

// GetConversationNotReceiveMessageUserIDs 获取不接受消息的用户 ID。
func (h *conversationHandler) GetConversationNotReceiveMessageUserIDs(ctx context.Context, req *pb.GetConversationNotReceiveMessageUserIDsReq) (*pb.GetConversationNotReceiveMessageUserIDsResp, error) {
	ids, err := h.svc.GetConversationNotReceiveMessageUserIDs(ctx, req.ConversationId)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetConversationNotReceiveMessageUserIDsResp{UserIds: ids}, nil
}

// GetPinnedConversationIDs 获取用户已置顶的会话 ID 列表。
func (h *conversationHandler) GetPinnedConversationIDs(ctx context.Context, req *pb.GetPinnedConversationIDsReq) (*pb.GetPinnedConversationIDsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := h.svc.GetPinnedConversationIDs(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetPinnedConversationIDsResp{ConversationIds: ids}, nil
}

// GetNotNotifyConversationIDs 获取用户静音的会话 ID 列表。
func (h *conversationHandler) GetNotNotifyConversationIDs(ctx context.Context, req *pb.GetNotNotifyConversationIDsReq) (*pb.GetNotNotifyConversationIDsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := h.svc.GetNotNotifyConversationIDs(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetNotNotifyConversationIDsResp{ConversationIds: ids}, nil
}

// DeleteConversations 删除用户的指定会话。
// 网关 HTTP 会强制 OwnerUserId=调用者；群服务解散群时可传入成员 ID 做内部清理。
func (h *conversationHandler) DeleteConversations(ctx context.Context, req *pb.DeleteConversationsReq) (*pb.DeleteConversationsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	ownerID := userID
	if req.OwnerUserId != "" {
		ownerID = req.OwnerUserId
	}
	if err := h.svc.DeleteConversations(ctx, ownerID, req.ConversationIds, req.NeedDeleteTime); err != nil {
		logger.Error(ctx, "delete conversations failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.DeleteConversationsResp{}, nil
}

// UpdateConversationsByUser 更新用户所有会话的扩展字段。
func (h *conversationHandler) UpdateConversationsByUser(ctx context.Context, req *pb.UpdateConversationsByUserReq) (*pb.UpdateConversationsByUserResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.UpdateConversationsByUser(ctx, userID, req.Ex); err != nil {
		logger.Error(ctx, "update conversations by user failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.UpdateConversationsByUserResp{}, nil
}

// GetUserConversationIDsHash 获取用户会话 ID 列表的哈希值。
func (h *conversationHandler) GetUserConversationIDsHash(ctx context.Context, req *pb.GetUserConversationIDsHashReq) (*pb.GetUserConversationIDsHashResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	hash, err := h.svc.GetUserConversationIDsHash(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetUserConversationIDsHashResp{Hash: hash}, nil
}

// GetOwnerConversation 分页获取用户所属会话列表（附带最后一条消息预览）。
func (h *conversationHandler) GetOwnerConversation(ctx context.Context, req *pb.GetOwnerConversationReq) (*pb.GetOwnerConversationResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	convs, total, err := h.svc.GetOwnerConversation(ctx, userID, int(req.Offset), int(req.Limit))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	out := toProtoSlice(convs)
	ids := make([]string, 0, len(convs))
	for _, c := range convs {
		ids = append(ids, c.ConversationID)
	}
	if latest, err := h.svc.ListLatestMsgs(ctx, userID, ids); err == nil {
		for _, c := range out {
			if m, ok := latest[c.ConversationId]; ok {
				c.MsgInfo = latestMsgToProto(&m)
			}
		}
	}
	return &pb.GetOwnerConversationResp{Total: total, Conversations: out}, nil
}

// ClearUserConversationMsg 清理用户会话中到期的阅后即焚消息。
func (h *conversationHandler) ClearUserConversationMsg(ctx context.Context, req *pb.ClearUserConversationMsgReq) (*pb.ClearUserConversationMsgResp, error) {
	count, err := h.svc.ClearUserConversationMsg(ctx, req.Timestamp, int(req.Limit))
	if err != nil {
		logger.Error(ctx, "clear user conversation msg failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.ClearUserConversationMsgResp{Count: int32(count)}, nil
}

// GetConversationsNeedClearMsg 获取需要清理消息的会话列表。
func (h *conversationHandler) GetConversationsNeedClearMsg(ctx context.Context, req *pb.GetConversationsNeedClearMsgReq) (*pb.GetConversationsNeedClearMsgResp, error) {
	convs, err := h.svc.GetConversationsNeedClearMsg(ctx)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetConversationsNeedClearMsgResp{Conversations: toProtoSlice(convs)}, nil
}

// GetFullOwnerConversationIDs 获取用户全部会话 ID。
func (h *conversationHandler) GetFullOwnerConversationIDs(ctx context.Context, req *pb.GetFullOwnerConversationIDsReq) (*pb.GetFullOwnerConversationIDsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := h.svc.GetFullOwnerConversationIDs(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetFullOwnerConversationIDsResp{ConversationIds: ids}, nil
}

// GetIncrementalConversation 获取用户增量会话数据。
func (h *conversationHandler) GetIncrementalConversation(ctx context.Context, req *pb.GetIncrementalConversationReq) (*pb.GetIncrementalConversationResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	convs, err := h.svc.GetIncrementalConversation(ctx, userID)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetIncrementalConversationResp{Full: true, Conversations: toProtoSlice(convs)}, nil
}

// --------------- 批量转换辅助 ---------------

// toProtoSlice 批量转换 Conversation 切片为 proto Conversation 切片。
func toProtoSlice(convs []types.Conversation) []*pb.Conversation {
	out := make([]*pb.Conversation, 0, len(convs))
	for i := range convs {
		out = append(out, conversationToProto(&convs[i]))
	}
	return out
}
