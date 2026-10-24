package logic

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/pkg/msgconv"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const sessionSingle = 1

// SendMsg checks one message and publishes it to toRedis. msgtransfer assigns the seq.
func (s *Server) SendMsg(ctx context.Context, req *msgpb.SendMsgReq) (*msgpb.SendMsgResp, error) {
	msg := req.GetMsgData()
	if msg == nil {
		return nil, errorcode.ErrMsgInvalidArgs.WithMsg("msg data is required")
	}
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
	// 2. 单聊要求互为好友，任一方拉黑都拒绝。群消息和通知不在这里查关系。
	if codeErr := s.authorizeSend(ctx, msg); codeErr != nil {
		return nil, codeErr
	}
	// 3. 补齐 id 和时间，投递到 toRedis。
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
	return &msgpb.SendMsgResp{
		ServerMsgId: msg.ServerMsgId,
		ClientMsgId: msg.ClientMsgId,
		SendTime:    msg.SendTime,
	}, nil
}

func (s *Server) authorizeSend(ctx context.Context, msg *msgpb.MsgData) error {
	if msg.SessionType != sessionSingle {
		return nil
	}
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
	return nil
}
