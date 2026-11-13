package logic

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const (
	sessionSingle       = 1
	sessionGroup        = 2
	sessionSuper        = 3
	sessionNotification = 4 // sn_ person-to-person; still needs friend/block checks

	roleOwner             = 100
	roleAdmin             = 60
	groupStatusDismissed  = 2
	groupStatusMuted      = 3
	contentGroupDismissed = 1511

	modelReceive      int32 = 0
	modelNotReceive   int32 = 1
	modelReceiveQuiet int32 = 2
	optOfflinePush          = "offlinePush"
)

// 校验一条消息后写入 toRedis。序号由 msgtransfer 分配，这里不落库。
func (s *Server) SendMsg(ctx context.Context, req *msgpb.SendMsgReq) (*msgpb.SendMsgResp, error) {
	msg := req.GetMsgData()
	if msg == nil {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("msg data is required")
	}
	sendID, err := actorUser(ctx, msg.SendId)
	if err != nil {
		return nil, err
	}
	msg.SendId = sendID
	// 1. 校验会话类型和正文。序号不在这里分配。
	switch msg.SessionType {
	case 1, 4:
		if msg.RecvId == "" {
			return nil, errorcode.ErrMsgInvalidArgs.WithMsg("recv id is required")
		}
	case 2, 3:
		if msg.GroupId == "" {
			return nil, errorcode.ErrMsgInvalidArgs.WithMsg("group id is required")
		}
	default:
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("session type is required")
	}
	if len(msg.Content) == 0 {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("content is required")
	}
	// 2. 单聊要求互为好友，任一方拉黑都拒绝。群消息校验成员和禁言。通知不查这些。
	if codeErr := s.authorizeSend(ctx, msg); codeErr != nil {
		return nil, codeErr
	}
	// 3. 发送前 webhook。回调拒绝则中断，beforeMsgModify 可以改正文。
	if err := s.beforeSend(ctx, msg); err != nil {
		return nil, err
	}
	if err := s.beforeModify(ctx, msg); err != nil {
		return nil, err
	}
	// 4. 补齐 id 和时间，投递到 toRedis。
	key := msgconv.QueueKey(msg)
	if key == "" {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("conversation id is empty")
	}
	if msg.ClientMsgId == "" {
		msg.ClientMsgId = newClientMsgID()
	}
	if msg.ServerMsgId == "" {
		msg.ServerMsgId = msgconv.ServerMsgID(key, msg.ClientMsgId)
	}
	now := time.Now().UnixMilli()
	if msg.SendTime == 0 {
		msg.SendTime = now
	}
	if msg.CreateTime == 0 {
		msg.CreateTime = msg.SendTime
	}
	if msg.Status == 0 {
		msg.Status = 1
	}
	body, err := proto.Marshal(msg)
	if err != nil {
		return nil, errorcode.ErrInternal.WithMsg("marshal message")
	}
	if s.publish == nil {
		return nil, errorcode.ErrInternal.WithMsg("queue is not configured")
	}
	if err := s.publish.Send(ctx, key, body); err != nil {
		return nil, errorcode.ErrInternal.WithMsg("queue send failed")
	}
	s.afterSend(ctx, msg)
	return &msgpb.SendMsgResp{
		ServerMsgId: msg.ServerMsgId,
		ClientMsgId: msg.ClientMsgId,
		SendTime:    msg.SendTime,
	}, nil
}

func (s *Server) authorizeSend(ctx context.Context, msg *msgpb.MsgData) error {
	// 已解散的群只放行 1511。其他通知和普通消息都拒绝。
	if msg.SessionType == sessionGroup || msg.SessionType == sessionSuper {
		if err := s.rejectDismissedGroup(ctx, msg); err != nil {
			return err
		}
	}
	// 通知不走好友、拉黑和禁言校验，入群申请会发给非好友的群主。
	if msgconv.IsNotification(msg) {
		return nil
	}
	switch msg.SessionType {
	case sessionGroup, sessionSuper:
		return s.authorizeGroupSend(ctx, msg)
	case sessionSingle, sessionNotification:
		return s.authorizeSingleSend(ctx, msg)
	default:
		return nil
	}
}

func (s *Server) authorizeSingleSend(ctx context.Context, msg *msgpb.MsgData) error {
	if msg.SendId == "" || msg.SendId == msg.RecvId {
		return errorcode.ErrMsgInvalidArgs.WithMsg("cannot message yourself")
	}
	if s.relations == nil {
		return errorcode.ErrInternal.WithMsg("relation service is not configured")
	}
	blockedBySender, blockedByReceiver, err := s.relations.IsBlack(ctx, msg.SendId, msg.RecvId)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check black")
	}
	if blockedBySender || blockedByReceiver {
		return errorcode.ErrMsgBlocked
	}
	senderHasFriend, receiverHasFriend, err := s.relations.IsFriend(ctx, msg.SendId, msg.RecvId)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check friend")
	}
	if !senderHasFriend || !receiverHasFriend {
		return errorcode.ErrMsgNotFriend
	}
	return s.applySingleRecv(ctx, msg)
}

// 按接收方自己的单聊接收设置决定投递。0 或缺行照常送达，1 拒绝，2 送达但关掉离线推送。已读回执在不接收时仍然送达。
func (s *Server) applySingleRecv(ctx context.Context, msg *msgpb.MsgData) error {
	if s.recv == nil || msg == nil {
		return nil
	}
	setting, err := s.recv.RecvOpt(ctx, msg.RecvId, msgconv.ChatConversationID(msg))
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check receive option")
	}
	if !setting.Found || setting.Opt == modelReceive {
		return nil
	}
	switch setting.Opt {
	case modelNotReceive:
		if msg.ContentType == contentHasReadReceipt {
			return nil
		}
		return errorcode.ErrMsgNotReceiving
	case modelReceiveQuiet:
		if msg.Options == nil {
			msg.Options = map[string]bool{}
		}
		msg.Options[optOfflinePush] = false
		return nil
	default:
		return nil
	}
}

func (s *Server) rejectDismissedGroup(ctx context.Context, msg *msgpb.MsgData) error {
	if msg.GetContentType() == contentGroupDismissed || msg.GroupId == "" || s.groups == nil {
		return nil
	}
	speaker, err := s.groups.Speaker(ctx, msg.GroupId, msg.SendId)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check group")
	}
	if speaker.Found && speaker.Status == groupStatusDismissed {
		return errorcode.ErrMsgDismissed
	}
	return nil
}

func (s *Server) authorizeGroupSend(ctx context.Context, msg *msgpb.MsgData) error {
	if msg.SendId == "" || msg.GroupId == "" {
		return errorcode.ErrMsgInvalidArgs.WithMsg("group sender is required")
	}
	if s.groups == nil {
		return errorcode.ErrInternal.WithMsg("group service is not configured")
	}
	speaker, err := s.groups.Speaker(ctx, msg.GroupId, msg.SendId)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check group")
	}
	if !speaker.Found || !speaker.InGroup {
		return errorcode.ErrMsgNotInGroup
	}
	// 群主不受个人禁言和全员禁言限制。管理员在全员禁言时仍可发言。
	if speaker.Role == roleOwner {
		return nil
	}
	now := time.Now().UnixMilli()
	if speaker.MuteEndMs >= now && speaker.MuteEndMs != 0 {
		return errorcode.ErrMsgMutedInGroup
	}
	if speaker.Status == groupStatusMuted && speaker.Role != roleAdmin {
		return errorcode.ErrMsgMutedGroup
	}
	return nil
}
