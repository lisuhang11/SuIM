// Package service 实现消息领域的业务逻辑。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	apperrors "message/internal/errors"
	"message/internal/logger"
	"message/internal/repository"
	"message/internal/types"
	"message/internal/types/interfaces"

	pb "SuIM/proto/pushpb"
	sdkws "SuIM/proto/sdkws"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"gorm.io/gorm"
)

// 哨兵错误从 repository 包别名引用，使 errors.Is 跨层工作（WeKnora 模式）。
var (
	ErrRevokePermission = repository.ErrRevokePermission
	ErrMessageNotFound  = gorm.ErrRecordNotFound
)

// messageService 消息服务实现。
type messageService struct {
	repo       interfaces.MessageRepository
	pushClient pb.PushMsgServiceClient
}

// NewMessageService 创建消息服务实例。
// pushAddr 为 push 服务地址，为空字符串则不启用离线推送。
func NewMessageService(repo interfaces.MessageRepository, pushAddr string) interfaces.MessageService {
	svc := &messageService{repo: repo}
	if pushAddr != "" {
		conn, err := grpc.NewClient(pushAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			logger.Warn(context.Background(), "failed to dial push service, offline push disabled",
				"push_addr", pushAddr, "error", err)
		} else {
			svc.pushClient = pb.NewPushMsgServiceClient(conn)
		}
	}
	return svc
}

// SendMsg 校验并发送消息：基于 client_msg_id 幂等，
// 填充服务端字段（server_msg_id、send_time、status），
// 持久化并为每个接收方推进 seq_user 游标。
func (s *messageService) SendMsg(ctx context.Context, msg *types.Message) (*types.Message, error) {
	if msg.ConversationID == "" || msg.SendID == "" {
		return nil, apperrors.NewInvalidMessageError("conversation_id and send_id are required")
	}
	if msg.ClientMsgID == "" {
		return nil, apperrors.NewInvalidMessageError("client_msg_id is required for idempotency")
	}

	// 幂等检查：如果 client_msg_id 已存在，直接返回已存储的消息。
	existing, err := s.repo.GetByClientMsgIDs(ctx, []string{msg.ClientMsgID})
	if err != nil {
		return nil, apperrors.NewInternalError("failed to check message idempotency").WithDetails(err)
	}
	if len(existing) > 0 {
		return &existing[0], nil
	}

	// 填充服务端字段。
	if msg.ServerMsgID == "" {
		msg.ServerMsgID = uuid.New().String()
	}
	now := time.Now().UnixMilli()
	if msg.SendTime == 0 {
		msg.SendTime = now
	}
	if msg.CreateTime == 0 {
		msg.CreateTime = now
	}
	if msg.Status == 0 {
		msg.Status = types.MsgStatusNormal
	}

	// msg.RecvUserIDs 不持久化，仅用于推进接收方游标。
	if err := s.repo.SendMessage(ctx, msg, msg.RecvUserIDs); err != nil {
		return nil, apperrors.NewInternalError("failed to persist message").WithDetails(err)
	}

	// 异步离线推送：消息已持久化，推送失败不影响主流程。
	if s.pushClient != nil {
		req := buildPushRequest(msg)
		go func() {
			if _, err := s.pushClient.PushMsg(context.Background(), req); err != nil {
				logger.Warn(context.Background(), "offline push failed",
					"conversation_id", msg.ConversationID,
					"user_count", len(msg.RecvUserIDs),
					"error", err,
				)
			}
		}()
	}
	return msg, nil
}

// GetHistoryMessages 返回一页历史消息和分页前的匹配行数。
func (s *messageService) GetHistoryMessages(ctx context.Context, conversationID string, anchorSeq int64, limit, order int) ([]types.Message, int64, error) {
	if conversationID == "" {
		return nil, 0, apperrors.NewValidationError("conversation_id is required")
	}
	msgs, matched, err := s.repo.GetHistory(ctx, conversationID, anchorSeq, limit, order)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to load history").WithDetails(err)
	}
	return msgs, matched, nil
}

// GetMessagesBySeq 按 seq 获取消息。
func (s *messageService) GetMessagesBySeq(ctx context.Context, conversationID string, seqs []int64) ([]types.Message, error) {
	msgs, err := s.repo.GetBySeqs(ctx, conversationID, seqs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load messages by seq").WithDetails(err)
	}
	return msgs, nil
}

// GetMessagesByClientMsgIDs 按客户端消息 ID 获取消息。
func (s *messageService) GetMessagesByClientMsgIDs(ctx context.Context, clientMsgIDs []string) ([]types.Message, error) {
	msgs, err := s.repo.GetByClientMsgIDs(ctx, clientMsgIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load messages by client_msg_id").WithDetails(err)
	}
	return msgs, nil
}

// RevokeMsg 撤回消息：仅发送者可撤回，消息不存在则返回未找到错误。
func (s *messageService) RevokeMsg(ctx context.Context, conversationID, clientMsgID, sendID string, revokeRole int32, revokeNickname string) error {
	err := s.repo.Revoke(ctx, conversationID, clientMsgID, sendID, revokeRole, revokeNickname)
	if err != nil {
		if errors.Is(err, ErrRevokePermission) {
			return apperrors.NewRevokePermissionError()
		}
		if errors.Is(err, ErrMessageNotFound) {
			return apperrors.NewMessageNotFoundError()
		}
		return apperrors.NewInternalError("failed to revoke message").WithDetails(err)
	}
	return nil
}

// MarkMsgsAsRead 标记已读，同时尽最大努力同步会话服务的 min_seq。
func (s *messageService) MarkMsgsAsRead(ctx context.Context, conversationID, userID string, seq int64) error {
	if err := s.repo.MarkMessagesRead(ctx, conversationID, seq); err != nil {
		return apperrors.NewInternalError("failed to mark messages read").WithDetails(err)
	}
	if err := s.repo.SetConversationMinSeq(ctx, conversationID, userID, seq); err != nil {
		// 尽最大努力兼容性操作；失败记录日志但不影响已读确认。
		logger.Warn(ctx, "failed to advance conversation read cursor", "error", err)
	}
	return nil
}

// DeleteMsgs 删除消息。
func (s *messageService) DeleteMsgs(ctx context.Context, conversationID string, seqs []int64) error {
	if err := s.repo.Delete(ctx, conversationID, seqs); err != nil {
		return apperrors.NewInternalError("failed to delete messages").WithDetails(err)
	}
	return nil
}

// buildPushRequest 将消息领域模型转换为 push 服务请求。
func buildPushRequest(msg *types.Message) *pb.PushMsgReq {
	sdkMsg := &sdkws.MsgData{
		ClientMsgId:      msg.ClientMsgID,
		ServerMsgId:      msg.ServerMsgID,
		ConversationId:   msg.ConversationID,
		SendId:           msg.SendID,
		RecvId:           msg.RecvID,
		GroupId:          msg.GroupID,
		SessionType:      int32(msg.SessionType),
		MsgFrom:          int32(msg.MsgFrom),
		ContentType:      int32(msg.ContentType),
		Content:          msg.Content,
		Seq:              msg.Seq,
		SendTime:         msg.SendTime,
		Status:           int32(msg.Status),
		Ex:               msg.Ex,
		SenderPlatformId: int32(msg.SenderPlatformID),
		SenderNickname:   msg.SenderNickname,
		SenderFaceUrl:    msg.SenderFaceURL,
		Options:          msg.Options,
		AttachedInfo:     msg.AttachedInfo,
	}

	// 解析 @用户列表。
	if msg.AtUserIDList != "" {
		var list []string
		if err := json.Unmarshal([]byte(msg.AtUserIDList), &list); err == nil {
			sdkMsg.AtUserIdList = list
		}
	}

	return &pb.PushMsgReq{
		MsgData:        sdkMsg,
		ConversationId: msg.ConversationID,
		UserIds:        msg.RecvUserIDs,
	}
}
