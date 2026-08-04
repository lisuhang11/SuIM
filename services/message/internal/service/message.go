// Package service 实现消息领域的业务逻辑。
package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"message/internal/client"
	apperrors "message/internal/errors"
	"message/internal/logger"
	msgnotif "message/internal/notification"
	"message/internal/repository"
	"message/internal/types"
	"message/internal/types/interfaces"

	"SuIM/pkg/discovery"
	pkgnotif "SuIM/pkg/notification"
	messagepb "SuIM/proto/messagepb"
	"SuIM/proto/msggatewaypb"
	pb "SuIM/proto/pushpb"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"gorm.io/gorm"
)

// 哨兵错误从 repository 包别名引用，使 errors.Is 跨层工作（WeKnora 模式）。
var (
	ErrRevokePermission = repository.ErrRevokePermission
	ErrRevokeExpired    = repository.ErrRevokeExpired
	ErrMessageNotFound  = gorm.ErrRecordNotFound
)

// blackChecker 判断 sendID 是否在 recvID 黑名单中（OpenIM IsBlack 语义）。
type blackChecker interface {
	IsBlockedByPeer(ctx context.Context, sendID, recvID string) (bool, error)
}

// friendChecker 判断双方是否互为好友。
type friendChecker interface {
	IsMutualFriend(ctx context.Context, user1, user2 string) (bool, error)
}

// groupMemberResolver 解析群成员列表（对齐 OpenIM GetGroupMemberIDs）。
type groupMemberResolver interface {
	GetGroupMemberUserIDs(ctx context.Context, groupID string) ([]string, error)
}

// tipNotifier 撤回/已读 tip 推送（对齐 OpenIM NotificationSender）。
type tipNotifier interface {
	RevokeNotification(ctx context.Context, revokerUserID string, recvIDs []string, sessionType int32, tips pkgnotif.RevokeMsgTips)
	HasReadReceiptNotification(ctx context.Context, readerUserID, recvID string, sessionType int32, tips pkgnotif.MarkAsReadTips)
}

// messageService 消息服务实现。
type messageService struct {
	repo             interfaces.MessageRepository
	pushClient       pb.PushMsgServiceClient
	msgGatewayClient msggatewaypb.MsgGatewayClient
	blackChecker     blackChecker
	friendChecker    friendChecker
	groupMembers     groupMemberResolver
	tips             tipNotifier
}

// NewMessageService 创建消息服务实例。
// push / msggateway / relation / group 地址通过 etcd 服务发现自动解析。
func NewMessageService(repo interfaces.MessageRepository) interfaces.MessageService {
	svc := &messageService{repo: repo}
	pushConn, err := grpc.NewClient(
		discovery.TargetURL("push"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		logger.Warn(context.Background(), "failed to connect push service via etcd, offline push disabled",
			"error", err)
	} else {
		svc.pushClient = pb.NewPushMsgServiceClient(pushConn)
	}

	gwConn, err := grpc.NewClient(
		discovery.TargetURL("msggateway"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		logger.Warn(context.Background(), "failed to connect msggateway via etcd, online push disabled",
			"error", err)
	} else {
		svc.msgGatewayClient = msggatewaypb.NewMsgGatewayClient(gwConn)
	}

	relConn, err := grpc.NewClient(
		discovery.TargetURL("relation"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		logger.Warn(context.Background(), "failed to connect relation service via etcd, blacklist send check disabled",
			"error", err)
	} else {
		checker := client.NewBlackChecker(relConn)
		svc.blackChecker = checker
		svc.friendChecker = checker
	}

	groupConn, err := grpc.NewClient(
		discovery.TargetURL("group"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		logger.Warn(context.Background(), "failed to connect group service via etcd, group member resolve disabled",
			"error", err)
	} else {
		svc.groupMembers = client.NewGroupMemberResolver(groupConn)
	}

	// tip 推送复用 msggateway OnlinePush（与 relation 好友 tip 同一管道）。
	if svc.msgGatewayClient != nil {
		gw := svc.msgGatewayClient
		svc.tips = msgnotif.NewMessageNotificationSender(func(ctx context.Context, recvID string, msg *messagepb.MsgData) error {
			_, err := gw.OnlinePushMsg(ctx, &msggatewaypb.OnlinePushMsgReq{
				PushToUserId: recvID,
				MsgData:      msg,
			})
			return err
		})
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

	// 单聊：必须互为好友，且接收方未拉黑发送方。
	if err := s.verifySingleChatFriend(ctx, msg); err != nil {
		return nil, err
	}
	if err := s.verifySingleChatBlacklist(ctx, msg); err != nil {
		return nil, err
	}

	// 幂等检查：如果 client_msg_id 已存在，直接返回已存储的消息。
	existing, err := s.repo.GetBySenderClientMsgIDs(ctx, msg.SendID, []string{msg.ClientMsgID})
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

	// 单聊兜底：未显式传 recv_user_ids 时用 recv_id 推进游标并在线推送。
	if len(msg.RecvUserIDs) == 0 && msg.RecvID != "" {
		msg.RecvUserIDs = []string{msg.RecvID}
	}
	// 群聊：对齐 OpenIM，服务端解析成员列表（客户端可不传 recv_user_ids）。
	if err := s.resolveGroupRecipients(ctx, msg); err != nil {
		return nil, err
	}

	// msg.RecvUserIDs 不持久化，仅用于推进接收方游标。
	if err := s.repo.SendMessage(ctx, msg, msg.RecvUserIDs); err != nil {
		// 唯一约束解决并发重试竞态；冲突时返回已经成功落库的消息。
		existing, lookupErr := s.repo.GetBySenderClientMsgIDs(ctx, msg.SendID, []string{msg.ClientMsgID})
		if lookupErr == nil && len(existing) > 0 {
			return &existing[0], nil
		}
		return nil, apperrors.NewInternalError("failed to persist message").WithDetails(err)
	}

	// 异步推送：在线走 msggateway，离线走 push stub；失败不影响落库成功。
	// 复制入站鉴权 metadata，供下游 push/msggateway 鉴权。
	pushCtx := detachAuthContext(ctx)
	go s.dispatchPush(pushCtx, msg)
	return msg, nil
}

// detachAuthContext 从请求 ctx 复制 authorization metadata 到可独立取消的 context。
func detachAuthContext(ctx context.Context) context.Context {
	out := context.Background()
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		out = metadata.NewOutgoingContext(out, md.Copy())
	} else if md, ok := metadata.FromOutgoingContext(ctx); ok {
		out = metadata.NewOutgoingContext(out, md.Copy())
	}
	return out
}

// resolveGroupRecipients 群聊时用 group.GetGroupMemberUserIDs 填充 RecvUserIDs（排除发送方）。
func (s *messageService) resolveGroupRecipients(ctx context.Context, msg *types.Message) error {
	if msg.SessionType != types.SessionTypeGroup {
		return nil
	}
	groupID := strings.TrimSpace(msg.GroupID)
	if groupID == "" {
		groupID = parseGroupIDFromConversation(msg.ConversationID)
		msg.GroupID = groupID
	}
	if groupID == "" {
		return apperrors.NewInvalidMessageError("group_id is required for group chat")
	}
	// 必须以服务端成员列表为准；解析器不可用则失败关闭，避免伪造收件人。
	if s.groupMembers == nil {
		return apperrors.NewInternalError("group member resolver unavailable")
	}
	ids, err := s.groupMembers.GetGroupMemberUserIDs(ctx, groupID)
	if err != nil {
		return apperrors.NewInternalError("failed to resolve group members").WithDetails(err)
	}
	isMember := false
	recv := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if id == msg.SendID {
			isMember = true
			continue
		}
		recv = append(recv, id)
	}
	if !isMember {
		return apperrors.NewNotGroupMemberError()
	}
	msg.RecvUserIDs = recv
	return nil
}

func parseGroupIDFromConversation(conversationID string) string {
	conversationID = strings.TrimSpace(conversationID)
	if strings.HasPrefix(conversationID, "gid_") {
		return strings.TrimPrefix(conversationID, "gid_")
	}
	return ""
}

// verifySingleChatFriend 单聊发送前检查双方是否互为好友。
// 群聊、通知类消息、缺少接收方时跳过；未注入 checker 时拒绝（失败关闭）。
func (s *messageService) verifySingleChatFriend(ctx context.Context, msg *types.Message) error {
	if msg.SessionType != types.SessionTypeSingle {
		return nil
	}
	if msg.RecvID == "" {
		return nil
	}
	if msg.ContentType >= types.ContentTypeNotificationBegin &&
		msg.ContentType <= types.ContentTypeNotificationEnd {
		return nil
	}
	if s.friendChecker == nil {
		return apperrors.NewInternalError("friend checker unavailable")
	}
	ok, err := s.friendChecker.IsMutualFriend(ctx, msg.SendID, msg.RecvID)
	if err != nil {
		return apperrors.NewInternalError("failed to check friendship").WithDetails(err)
	}
	if !ok {
		return apperrors.NewNotFriendError()
	}
	return nil
}

// verifySingleChatBlacklist 单聊发送前检查：RecvID 是否拉黑了 SendID。
// 群聊、通知类消息、缺少接收方或未注入 checker 时跳过。
func (s *messageService) verifySingleChatBlacklist(ctx context.Context, msg *types.Message) error {
	if s.blackChecker == nil {
		return nil
	}
	if msg.SessionType != types.SessionTypeSingle {
		return nil
	}
	if msg.RecvID == "" {
		return nil
	}
	if msg.ContentType >= types.ContentTypeNotificationBegin &&
		msg.ContentType <= types.ContentTypeNotificationEnd {
		return nil
	}
	blocked, err := s.blackChecker.IsBlockedByPeer(ctx, msg.SendID, msg.RecvID)
	if err != nil {
		return apperrors.NewInternalError("failed to check blacklist").WithDetails(err)
	}
	if blocked {
		return apperrors.NewBlockedByPeerError()
	}
	return nil
}

// dispatchPush 向接收方做在线推送，并尝试离线推送。
func (s *messageService) dispatchPush(parent context.Context, msg *types.Message) {
	ctx, cancel := context.WithTimeout(parent, 5*time.Second)
	defer cancel()

	sdkMsg := toMsgData(msg)
	for _, userID := range msg.RecvUserIDs {
		if userID == "" || userID == msg.SendID {
			continue
		}
		if s.msgGatewayClient == nil {
			break
		}
		if _, err := s.msgGatewayClient.OnlinePushMsg(ctx, &msggatewaypb.OnlinePushMsgReq{
			MsgData:      sdkMsg,
			PushToUserId: userID,
		}); err != nil {
			logger.Warn(ctx, "online push failed",
				"conversation_id", msg.ConversationID,
				"recv_user_id", userID,
				"error", err,
			)
		}
	}

	if s.pushClient == nil || len(msg.RecvUserIDs) == 0 {
		return
	}
	if _, err := s.pushClient.PushMsg(ctx, &pb.PushMsgReq{
		MsgData:        sdkMsg,
		ConversationId: msg.ConversationID,
		UserIds:        msg.RecvUserIDs,
	}); err != nil {
		logger.Warn(ctx, "offline push failed",
			"conversation_id", msg.ConversationID,
			"user_count", len(msg.RecvUserIDs),
			"error", err,
		)
	}
}

// GetHistoryMessages 返回一页历史消息和分页前的匹配行数。
func (s *messageService) GetHistoryMessages(ctx context.Context, userID, conversationID string, anchorSeq int64, limit, order int) ([]types.Message, int64, error) {
	if userID == "" || conversationID == "" {
		return nil, 0, apperrors.NewValidationError("user_id and conversation_id are required")
	}
	msgs, matched, err := s.repo.GetHistory(ctx, userID, conversationID, anchorSeq, limit, order)
	if err != nil {
		return nil, 0, apperrors.NewInternalError("failed to load history").WithDetails(err)
	}
	return msgs, matched, nil
}

// GetMessagesBySeq 按 seq 获取消息。
func (s *messageService) GetMessagesBySeq(ctx context.Context, userID, conversationID string, seqs []int64) ([]types.Message, error) {
	if userID == "" || conversationID == "" {
		return nil, apperrors.NewValidationError("user_id and conversation_id are required")
	}
	msgs, err := s.repo.GetBySeqs(ctx, userID, conversationID, seqs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load messages by seq").WithDetails(err)
	}
	return msgs, nil
}

// GetMessagesByClientMsgIDs 按客户端消息 ID 获取消息。
func (s *messageService) GetMessagesByClientMsgIDs(ctx context.Context, userID string, clientMsgIDs []string) ([]types.Message, error) {
	if userID == "" {
		return nil, apperrors.NewValidationError("user_id is required")
	}
	msgs, err := s.repo.GetByClientMsgIDs(ctx, userID, clientMsgIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load messages by client_msg_id").WithDetails(err)
	}
	return msgs, nil
}

// RevokeMsg 撤回消息：仅发送者可撤回，消息不存在则返回未找到错误；成功后推送 2101 tip。
func (s *messageService) RevokeMsg(ctx context.Context, conversationID, clientMsgID, sendID string, revokeRole int32, revokeNickname string) error {
	msg, err := s.repo.Revoke(ctx, conversationID, clientMsgID, sendID, revokeRole, revokeNickname)
	if err != nil {
		if errors.Is(err, ErrRevokePermission) {
			return apperrors.NewRevokePermissionError()
		}
		if errors.Is(err, ErrRevokeExpired) {
			return apperrors.NewRevokeExpiredError()
		}
		if errors.Is(err, ErrMessageNotFound) {
			return apperrors.NewMessageNotFoundError()
		}
		return apperrors.NewInternalError("failed to revoke message").WithDetails(err)
	}
	s.dispatchRevokeTip(detachAuthContext(ctx), sendID, msg)
	return nil
}

// MarkMsgsAsRead 推进指定用户的已读游标，并推送 2200 已读 tip。
func (s *messageService) MarkMsgsAsRead(ctx context.Context, conversationID, userID string, seq int64) error {
	if userID == "" || conversationID == "" || seq < 0 {
		return apperrors.NewValidationError("valid user_id, conversation_id and seq are required")
	}
	if err := s.repo.SetReadSeq(ctx, userID, conversationID, seq); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return apperrors.NewValidationError("user cannot access conversation")
		}
		return apperrors.NewInternalError("failed to advance read sequence").WithDetails(err)
	}
	s.dispatchReadTip(detachAuthContext(ctx), conversationID, userID, seq)
	return nil
}

// dispatchRevokeTip 对齐 OpenIM：撤回后向会话参与方推送 RevokeMsgTips。
func (s *messageService) dispatchRevokeTip(ctx context.Context, revokerID string, msg *types.Message) {
	if s.tips == nil || msg == nil {
		return
	}
	tips := pkgnotif.RevokeMsgTips{
		RevokerUserID:  revokerID,
		ClientMsgID:    msg.ClientMsgID,
		RevokeTime:     msg.RevokeTime,
		SessionType:    int32(msg.SessionType),
		Seq:            msg.Seq,
		ConversationID: msg.ConversationID,
	}
	recvIDs := s.revokeTipRecipients(ctx, msg)
	s.tips.RevokeNotification(ctx, revokerID, recvIDs, int32(msg.SessionType), tips)
}

// revokeTipRecipients 单聊推双方（多端）；群聊推全体成员。
func (s *messageService) revokeTipRecipients(ctx context.Context, msg *types.Message) []string {
	if msg.SessionType == types.SessionTypeGroup {
		groupID := strings.TrimSpace(msg.GroupID)
		if groupID == "" {
			groupID = parseGroupIDFromConversation(msg.ConversationID)
		}
		if groupID != "" && s.groupMembers != nil {
			if ids, err := s.groupMembers.GetGroupMemberUserIDs(ctx, groupID); err == nil && len(ids) > 0 {
				return ids
			}
		}
	}
	seen := map[string]struct{}{}
	var out []string
	for _, id := range []string{msg.SendID, msg.RecvID} {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// dispatchReadTip 单聊 tip→对端；群聊 tip→自己（多端未读同步）。
func (s *messageService) dispatchReadTip(ctx context.Context, conversationID, userID string, hasReadSeq int64) {
	if s.tips == nil {
		return
	}
	sessionType := int32(types.SessionTypeSingle)
	recvID := ""
	msgs, err := s.repo.GetBySeqs(ctx, userID, conversationID, []int64{hasReadSeq})
	if err == nil && len(msgs) > 0 {
		m := msgs[0]
		sessionType = int32(m.SessionType)
		if m.SessionType == types.SessionTypeGroup {
			recvID = userID
		} else if m.SendID == userID {
			recvID = m.RecvID
		} else {
			recvID = m.SendID
		}
	} else {
		// 兜底：用会话最后一条消息推断对端
		last, err := s.repo.GetLastMessage(ctx, userID, []string{conversationID})
		if err == nil {
			if m, ok := last[conversationID]; ok {
				sessionType = int32(m.SessionType)
				if m.SessionType == types.SessionTypeGroup {
					recvID = userID
				} else if m.SendID == userID {
					recvID = m.RecvID
				} else {
					recvID = m.SendID
				}
			}
		}
	}
	if recvID == "" {
		return
	}
	tips := pkgnotif.MarkAsReadTips{
		MarkAsReadUserID: userID,
		ConversationID:   conversationID,
		HasReadSeq:       hasReadSeq,
		Seqs:             []int64{hasReadSeq},
	}
	s.tips.HasReadReceiptNotification(ctx, userID, recvID, sessionType, tips)
}

// DeleteMsgs 仅对指定用户隐藏消息。
func (s *messageService) DeleteMsgs(ctx context.Context, userID, conversationID string, seqs []int64) error {
	if userID == "" || conversationID == "" {
		return apperrors.NewValidationError("user_id and conversation_id are required")
	}
	if err := s.repo.DeleteForUser(ctx, userID, conversationID, seqs); err != nil {
		return apperrors.NewInternalError("failed to delete messages").WithDetails(err)
	}
	return nil
}

// GetMaxSeq 返回用户在指定会话中的 max/min seq；conversationIDs 为空则返回全部。
func (s *messageService) GetMaxSeq(ctx context.Context, userID string, conversationIDs []string) (map[string]types.SeqBounds, error) {
	if userID == "" {
		return nil, apperrors.NewValidationError("user_id is required")
	}
	rows, err := s.repo.ListSeqUser(ctx, userID, conversationIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list seq_user").WithDetails(err)
	}
	out := make(map[string]types.SeqBounds, len(rows))
	for _, row := range rows {
		out[row.ConversationID] = types.SeqBounds{MaxSeq: row.MaxSeq, MinSeq: row.MinSeq}
	}
	return out, nil
}

// GetConversationsHasReadAndMaxSeq 返回用户在指定会话中的 max_seq、已读 seq 与 max_seq_time。
func (s *messageService) GetConversationsHasReadAndMaxSeq(ctx context.Context, userID string, conversationIDs []string) (map[string]types.SeqPair, error) {
	if userID == "" {
		return nil, apperrors.NewValidationError("user_id is required")
	}
	rows, err := s.repo.ListSeqUser(ctx, userID, conversationIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list seq_user").WithDetails(err)
	}
	convSeqs := make(map[string]int64, len(rows))
	for _, row := range rows {
		if row.MaxSeq > 0 {
			convSeqs[row.ConversationID] = row.MaxSeq
		}
	}
	sendTimes, err := s.repo.MapSendTimeByConvSeq(ctx, convSeqs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load max_seq send_time").WithDetails(err)
	}
	out := make(map[string]types.SeqPair, len(rows))
	for _, row := range rows {
		out[row.ConversationID] = types.SeqPair{
			MaxSeq:     row.MaxSeq,
			HasReadSeq: row.ReadSeq,
			MaxSeqTime: sendTimes[row.ConversationID],
		}
	}
	return out, nil
}

// GetActiveConversation 返回指定会话的 max_seq + last_time（对齐 OpenIM）。
func (s *messageService) GetActiveConversation(ctx context.Context, userID string, conversationIDs []string, limit int64) ([]types.ActiveConversation, error) {
	if userID == "" {
		return nil, apperrors.NewValidationError("user_id is required")
	}
	if len(conversationIDs) == 0 {
		return nil, nil
	}
	rows, err := s.repo.ListSeqUser(ctx, userID, conversationIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to list seq_user").WithDetails(err)
	}
	convSeqs := make(map[string]int64, len(rows))
	for _, row := range rows {
		if row.MaxSeq > 0 {
			convSeqs[row.ConversationID] = row.MaxSeq
		}
	}
	sendTimes, err := s.repo.MapSendTimeByConvSeq(ctx, convSeqs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to load max_seq send_time").WithDetails(err)
	}
	out := make([]types.ActiveConversation, 0, len(rows))
	for _, row := range rows {
		out = append(out, types.ActiveConversation{
			ConversationID: row.ConversationID,
			MaxSeq:         row.MaxSeq,
			LastTime:       sendTimes[row.ConversationID],
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LastTime > out[j].LastTime
	})
	if limit > 0 && int64(len(out)) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetLastMessage 返回每个会话对用户可见的最后一条消息。
func (s *messageService) GetLastMessage(ctx context.Context, userID string, conversationIDs []string) (map[string]types.Message, error) {
	if userID == "" {
		return nil, apperrors.NewValidationError("user_id is required")
	}
	if len(conversationIDs) == 0 {
		return map[string]types.Message{}, nil
	}
	msgs, err := s.repo.GetLastMessage(ctx, userID, conversationIDs)
	if err != nil {
		return nil, apperrors.NewInternalError("failed to get last messages").WithDetails(err)
	}
	return msgs, nil
}

// toMsgData 将消息领域模型转换为 protobuf MsgData。
func toMsgData(msg *types.Message) *messagepb.MsgData {
	sdkMsg := &messagepb.MsgData{
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
		CreateTime:       msg.CreateTime,
		Status:           int32(msg.Status),
		Ex:               msg.Ex,
		SenderPlatformId: int32(msg.SenderPlatformID),
		SenderNickname:   msg.SenderNickname,
		SenderFaceUrl:    msg.SenderFaceURL,
		Options:          msg.Options,
		AttachedInfo:     msg.AttachedInfo,
	}
	if msg.AtUserIDList != "" {
		var list []string
		if err := json.Unmarshal([]byte(msg.AtUserIDList), &list); err == nil {
			sdkMsg.AtUserIdList = list
		}
	}
	return sdkMsg
}
