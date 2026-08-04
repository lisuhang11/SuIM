// Package handler 将领域 MessageService 适配到 gRPC 传输层。
package handler

import (
	"context"
	"encoding/json"

	apperrors "message/internal/errors"
	"message/internal/logger"
	"message/internal/middleware"
	"message/internal/types"
	"message/internal/types/interfaces"

	pb "SuIM/proto/messagepb"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func authenticatedUserID(ctx context.Context) (string, error) {
	userID, ok := middleware.UserIDFromContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "authenticated user is missing")
	}
	return userID, nil
}

// messageHandler 实现 pb.MessageServer，委托给领域 MessageService。
type messageHandler struct {
	pb.UnimplementedMessageServer
	svc interfaces.MessageService
}

// NewMessageHandler 创建 gRPC MessageServer，注入领域服务。
func NewMessageHandler(svc interfaces.MessageService) pb.MessageServer {
	return &messageHandler{svc: svc}
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
	case apperrors.CodeValidation, apperrors.CodeInvalidMessage:
		code = codes.InvalidArgument
	case apperrors.CodeMessageNotFound, apperrors.CodeNotFound:
		code = codes.NotFound
	case apperrors.CodeRevokePermission, apperrors.CodeBlockedByPeer,
		apperrors.CodeNotFriend, apperrors.CodeNotGroupMember:
		code = codes.PermissionDenied
	case apperrors.CodeRevokeExpired:
		code = codes.FailedPrecondition
	default:
		code = codes.Internal
	}
	return status.Error(code, ae.Message)
}

// --------------- 模型转换 ---------------

// messageToProto 将领域模型转为 protobuf 消息。
func messageToProto(m *types.Message) *pb.MsgData {
	if m == nil {
		return nil
	}
	out := &pb.MsgData{
		ClientMsgId:      m.ClientMsgID,
		ServerMsgId:      m.ServerMsgID,
		ConversationId:   m.ConversationID,
		SendId:           m.SendID,
		RecvId:           m.RecvID,
		GroupId:          m.GroupID,
		SessionType:      int32(m.SessionType),
		MsgFrom:          int32(m.MsgFrom),
		ContentType:      int32(m.ContentType),
		Content:          m.Content,
		Seq:              m.Seq,
		SendTime:         m.SendTime,
		CreateTime:       m.CreateTime,
		Status:           int32(m.Status),
		Ex:               m.Ex,
		IsRead:           m.IsRead,
		SenderPlatformId: int32(m.SenderPlatformID),
		SenderNickname:   m.SenderNickname,
		SenderFaceUrl:    m.SenderFaceURL,
		Options:          m.Options,
		AttachedInfo:     m.AttachedInfo,
		DocId:            m.DocID,
		MsgIndex:         int32(m.MsgIndex),
	}
	// @用户列表：从 JSON 字符串反序列化。
	if m.AtUserIDList != "" {
		var list []string
		if err := json.Unmarshal([]byte(m.AtUserIDList), &list); err == nil {
			out.AtUserIdList = list
		}
	}
	// 离线推送信息。
	if m.OfflinePushTitle != "" || m.OfflinePushDesc != "" || m.OfflinePushEx != "" || m.OfflinePushIOSound != "" || m.OfflinePushIOSBadgeCount != 0 {
		out.OfflinePush = &pb.OfflinePush{
			Title:         m.OfflinePushTitle,
			Desc:          m.OfflinePushDesc,
			Ex:            m.OfflinePushEx,
			IosSound:      m.OfflinePushIOSound,
			IosBadgeCount: m.OfflinePushIOSBadgeCount != 0,
		}
	}
	// 撤回信息。
	if m.RevokeUserID != "" {
		out.Revoke = &pb.RevokeInfo{
			Role:     int32(m.RevokeRole),
			UserId:   m.RevokeUserID,
			Nickname: m.RevokeNickname,
			Time:     m.RevokeTime,
		}
	}
	return out
}

// protoToMessage 将 protobuf 消息转为领域模型。
func protoToMessage(m *pb.MsgData) *types.Message {
	if m == nil {
		return nil
	}
	out := &types.Message{
		ConversationID: m.ConversationId,
		RecvUserIDs:    m.RecvUserIds, // 不持久化，仅发送时使用
		MsgDataModel: types.MsgDataModel{
			ClientMsgID:      m.ClientMsgId,
			ServerMsgID:      m.ServerMsgId,
			SendID:           m.SendId,
			RecvID:           m.RecvId,
			GroupID:          m.GroupId,
			SessionType:      int(m.SessionType),
			MsgFrom:          int(m.MsgFrom),
			ContentType:      int(m.ContentType),
			Content:          m.Content,
			Seq:              m.Seq,
			SendTime:         m.SendTime,
			CreateTime:       m.CreateTime,
			Status:           int(m.Status),
			Ex:               m.Ex,
			SenderPlatformID: int(m.SenderPlatformId),
			SenderNickname:   m.SenderNickname,
			SenderFaceURL:    m.SenderFaceUrl,
			Options:          m.Options,
			AtUserIDList:     atUserIDListJSON(m.AtUserIdList),
			AttachedInfo:     m.AttachedInfo,
		},
	}
	if len(m.AtUserIdList) > 0 {
		if b, err := json.Marshal(m.AtUserIdList); err == nil {
			out.AtUserIDList = string(b)
		}
	}
	if m.OfflinePush != nil {
		out.OfflinePushTitle = m.OfflinePush.Title
		out.OfflinePushDesc = m.OfflinePush.Desc
		out.OfflinePushEx = m.OfflinePush.Ex
		out.OfflinePushIOSound = m.OfflinePush.IosSound
		if m.OfflinePush.IosBadgeCount {
			out.OfflinePushIOSBadgeCount = 1
		}
	}
	if m.Revoke != nil {
		out.RevokeRole = int(m.Revoke.Role)
		out.RevokeUserID = m.Revoke.UserId
		out.RevokeNickname = m.Revoke.Nickname
		out.RevokeTime = m.Revoke.Time
	}
	return out
}

// toProtoSlice 批量转换领域模型为 protobuf 切片。
func toProtoSlice(msgs []types.Message) []*pb.MsgData {
	out := make([]*pb.MsgData, 0, len(msgs))
	for i := range msgs {
		out = append(out, messageToProto(&msgs[i]))
	}
	return out
}

// atUserIDListJSON 将 @用户列表序列化为 JSON 存储形式。
func atUserIDListJSON(list []string) string {
	if len(list) == 0 {
		return ""
	}
	b, err := json.Marshal(list)
	if err != nil {
		return ""
	}
	return string(b)
}

// --------------- gRPC 接口实现 ---------------

// SendMsg 发送消息。幂等（基于 client_msg_id），返回服务端填充的消息字段。
func (h *messageHandler) SendMsg(ctx context.Context, req *pb.SendMsgReq) (*pb.SendMsgResp, error) {
	if req.MsgData == nil {
		return nil, appErrorToStatus(apperrors.NewValidationError("msg_data is required"))
	}
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	req.MsgData.SendId = userID
	saved, err := h.svc.SendMsg(ctx, protoToMessage(req.MsgData))
	if err != nil {
		logger.Error(ctx, "send message failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.SendMsgResp{
		ServerMsgId: saved.ServerMsgID,
		Seq:         saved.Seq,
		SendTime:    saved.SendTime,
		DocId:       saved.DocID,
		MsgIndex:    int32(saved.MsgIndex),
		ClientMsgId: saved.ClientMsgID,
	}, nil
}

// GetHistoryMessages 获取历史消息（游标分页）。
func (h *messageHandler) GetHistoryMessages(ctx context.Context, req *pb.GetHistoryMessagesReq) (*pb.GetHistoryMessagesResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 20
	} else if limit > 100 {
		limit = 100
	}
	msgs, matched, err := h.svc.GetHistoryMessages(ctx, userID, req.ConversationId, req.Seq, limit, int(req.Order))
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	// is_end：当匹配行数不超过 limit 时说明没有更多数据。
	isEnd := matched <= int64(limit)
	return &pb.GetHistoryMessagesResp{MsgData: toProtoSlice(msgs), IsEnd: isEnd}, nil
}

// GetMessagesBySeq 按 seq 获取消息。
func (h *messageHandler) GetMessagesBySeq(ctx context.Context, req *pb.GetMessagesBySeqReq) (*pb.GetMessagesBySeqResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	msgs, err := h.svc.GetMessagesBySeq(ctx, userID, req.ConversationId, req.Seqs)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetMessagesBySeqResp{MsgData: toProtoSlice(msgs)}, nil
}

// GetMessagesByClientMsgIDs 按客户端消息 ID 列表获取消息。
func (h *messageHandler) GetMessagesByClientMsgIDs(ctx context.Context, req *pb.GetMessagesByClientMsgIDsReq) (*pb.GetMessagesByClientMsgIDsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	msgs, err := h.svc.GetMessagesByClientMsgIDs(ctx, userID, req.ClientMsgIds)
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	return &pb.GetMessagesByClientMsgIDsResp{MsgData: toProtoSlice(msgs)}, nil
}

// RevokeMsg 撤回消息（仅发送者可撤回）。
func (h *messageHandler) RevokeMsg(ctx context.Context, req *pb.RevokeMsgReq) (*pb.RevokeMsgResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.RevokeMsg(ctx, req.ConversationId, req.ClientMsgId, userID, 0, ""); err != nil {
		logger.Error(ctx, "revoke message failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.RevokeMsgResp{}, nil
}

// MarkMsgsAsRead 标记消息为已读。
func (h *messageHandler) MarkMsgsAsRead(ctx context.Context, req *pb.MarkMsgsAsReadReq) (*pb.MarkMsgsAsReadResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.MarkMsgsAsRead(ctx, req.ConversationId, userID, req.Seq); err != nil {
		logger.Error(ctx, "mark messages read failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.MarkMsgsAsReadResp{}, nil
}

// GetMaxSeq 获取会话最大/最小 seq。
func (h *messageHandler) GetMaxSeq(ctx context.Context, req *pb.GetMaxSeqReq) (*pb.GetMaxSeqResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.GetMaxSeq(ctx, userID, req.GetConversationIds())
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	maxSeqs := make(map[string]int64, len(m))
	minSeqs := make(map[string]int64, len(m))
	for id, b := range m {
		maxSeqs[id] = b.MaxSeq
		minSeqs[id] = b.MinSeq
	}
	return &pb.GetMaxSeqResp{MaxSeqs: maxSeqs, MinSeqs: minSeqs}, nil
}

// GetConversationsHasReadAndMaxSeq 获取会话已读 seq 与最大 seq。
func (h *messageHandler) GetConversationsHasReadAndMaxSeq(ctx context.Context, req *pb.GetConversationsHasReadAndMaxSeqReq) (*pb.GetConversationsHasReadAndMaxSeqResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	m, err := h.svc.GetConversationsHasReadAndMaxSeq(ctx, userID, req.GetConversationIds())
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	out := make(map[string]*pb.Seqs, len(m))
	for id, p := range m {
		out[id] = &pb.Seqs{MaxSeq: p.MaxSeq, HasReadSeq: p.HasReadSeq, MaxSeqTime: p.MaxSeqTime}
	}
	return &pb.GetConversationsHasReadAndMaxSeqResp{Seqs: out}, nil
}

// DeleteMsgs 删除消息。
func (h *messageHandler) DeleteMsgs(ctx context.Context, req *pb.DeleteMsgsReq) (*pb.DeleteMsgsResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	if err := h.svc.DeleteMsgs(ctx, userID, req.ConversationId, req.Seqs); err != nil {
		logger.Error(ctx, "delete messages failed", "error", err)
		return nil, appErrorToStatus(err)
	}
	return &pb.DeleteMsgsResp{}, nil
}

// GetActiveConversation 获取活跃会话摘要（max_seq + last_time）。
func (h *messageHandler) GetActiveConversation(ctx context.Context, req *pb.GetActiveConversationReq) (*pb.GetActiveConversationResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	list, err := h.svc.GetActiveConversation(ctx, userID, req.GetConversationIds(), req.GetLimit())
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	out := make([]*pb.ActiveConversation, 0, len(list))
	for _, c := range list {
		out = append(out, &pb.ActiveConversation{
			ConversationId: c.ConversationID,
			MaxSeq:         c.MaxSeq,
			LastTime:       c.LastTime,
		})
	}
	return &pb.GetActiveConversationResp{Conversations: out}, nil
}

// GetLastMessage 获取每个会话最后一条可见消息。
func (h *messageHandler) GetLastMessage(ctx context.Context, req *pb.GetLastMessageReq) (*pb.GetLastMessageResp, error) {
	userID, err := authenticatedUserID(ctx)
	if err != nil {
		return nil, err
	}
	msgs, err := h.svc.GetLastMessage(ctx, userID, req.GetConversationIds())
	if err != nil {
		return nil, appErrorToStatus(err)
	}
	out := make(map[string]*pb.MsgData, len(msgs))
	for id, m := range msgs {
		msg := m
		out[id] = messageToProto(&msg)
	}
	return &pb.GetLastMessageResp{Msgs: out}, nil
}
