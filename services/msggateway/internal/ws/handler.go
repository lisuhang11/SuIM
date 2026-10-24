package ws

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

const sessionSingle = 1

func (s *Server) dispatch(c *Client, req *Req) error {
	if req.SendID != "" && req.SendID != c.userID {
		return c.reply(req, errArgs, "sendID does not match the connection", nil)
	}
	switch req.ReqIdentifier {
	case reqGetNewestSeq:
		return s.maxSeq(c, req)
	case reqPullMsgByRange:
		return s.pullByRange(c, req)
	case reqSendMsg:
		return s.sendMsg(c, req)
	case reqPullMsgBySeqList:
		return s.pullBySeqList(c, req)
	case reqGetConvMaxReadSeq:
		return s.readAndMaxSeq(c, req)
	case reqPullConvLastMsg:
		return s.lastMessages(c, req)
	case reqLogoutMsg:
		if err := c.reply(req, 0, "", nil); err != nil {
			return err
		}
		return errLogout
	case reqSetBackground:
		return s.background(c, req)
	case reqSubUserOnline:
		return s.subscribe(c, req)
	case reqSendSignalMsg:
		return c.reply(req, errArgs, "signal messages are not supported", nil)
	default:
		return c.reply(req, errArgs, "unsupported request", nil)
	}
}

func (s *Server) sendMsg(c *Client, req *Req) error {
	msg := &msgpb.MsgData{}
	if err := proto.Unmarshal(req.Data, msg); err != nil {
		return c.reply(req, errArgs, "invalid message", nil)
	}
	msg.SendId = c.userID
	msg.SenderPlatformId = int32(c.platformID)
	if s.messages == nil {
		return c.reply(req, errInternal, "msg service is not configured", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.messages.SendMsg(ctx, msg)
	if err != nil {
		return replyErr(c, req, err)
	}
	body, err := proto.Marshal(resp)
	if err != nil {
		return c.reply(req, errInternal, "marshal response", nil)
	}
	return c.reply(req, 0, "", body)
}

func (s *Server) maxSeq(c *Client, req *Req) error {
	if s.messages == nil {
		return c.reply(req, errInternal, "msg service is not configured", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.messages.GetMaxSeq(ctx, c.userID)
	if err != nil {
		return replyErr(c, req, err)
	}
	body, err := proto.Marshal(resp)
	if err != nil {
		return c.reply(req, errInternal, "marshal max seq", nil)
	}
	return c.reply(req, 0, "", body)
}

func (s *Server) readAndMaxSeq(c *Client, req *Req) error {
	in := &msgpb.GetConversationsHasReadAndMaxSeqReq{}
	if len(req.Data) > 0 {
		if err := proto.Unmarshal(req.Data, in); err != nil {
			return c.reply(req, errArgs, "invalid seq request", nil)
		}
	}
	in.UserId = c.userID
	if s.messages == nil {
		return c.reply(req, errInternal, "msg service is not configured", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.messages.GetConversationsHasReadAndMaxSeq(ctx, in)
	if err != nil {
		return replyErr(c, req, err)
	}
	body, err := proto.Marshal(resp)
	if err != nil {
		return c.reply(req, errInternal, "marshal read seq", nil)
	}
	return c.reply(req, 0, "", body)
}

func (s *Server) pullByRange(c *Client, req *Req) error {
	in := &msgpb.PullMessageBySeqsReq{}
	if err := proto.Unmarshal(req.Data, in); err != nil {
		return c.reply(req, errArgs, "invalid pull request", nil)
	}
	in.UserId = c.userID
	if s.messages == nil {
		return c.reply(req, errInternal, "msg service is not configured", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := s.messages.PullMessageBySeqs(ctx, in)
	if err != nil {
		return replyErr(c, req, err)
	}
	body, err := proto.Marshal(out)
	if err != nil {
		return c.reply(req, errInternal, "marshal messages", nil)
	}
	return c.reply(req, 0, "", body)
}

func (s *Server) pullBySeqList(c *Client, req *Req) error {
	in := &msgpb.GetSeqMessageReq{}
	if err := proto.Unmarshal(req.Data, in); err != nil {
		return c.reply(req, errArgs, "invalid seq list", nil)
	}
	in.UserId = c.userID
	if s.messages == nil {
		return c.reply(req, errInternal, "msg service is not configured", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := s.messages.GetSeqMessage(ctx, in)
	if err != nil {
		return replyErr(c, req, err)
	}
	body, err := proto.Marshal(out)
	if err != nil {
		return c.reply(req, errInternal, "marshal messages", nil)
	}
	return c.reply(req, 0, "", body)
}

func replyErr(c *Client, req *Req, err error) error {
	code, text := fromServiceErr(err)
	return c.reply(req, code, text, nil)
}

func fromServiceErr(err error) (int, string) {
	if code, ok := errorcode.CodeOf(err); ok {
		switch code {
		case errorcode.MsgNotFriend, errorcode.MsgBlocked, errorcode.MsgInvalidArgs, errorcode.NoPermission:
			return int(code), status.Convert(err).Message()
		}
	}
	st, ok := status.FromError(err)
	if !ok {
		return errInternal, "msg service"
	}
	switch st.Code() {
	case codes.InvalidArgument:
		return errArgs, st.Message()
	case codes.PermissionDenied:
		return int(errorcode.NoPermission), st.Message()
	case codes.FailedPrecondition:
		if code, ok := errorcode.CodeOf(err); ok {
			return int(code), st.Message()
		}
	}
	return errInternal, st.Message()
}

func (s *Server) lastMessages(c *Client, req *Req) error {
	in := &msgpb.GetLastMessageReq{}
	if len(req.Data) > 0 {
		if err := proto.Unmarshal(req.Data, in); err != nil {
			return c.reply(req, errArgs, "invalid last message request", nil)
		}
	}
	in.UserId = c.userID
	if s.messages == nil {
		return c.reply(req, errInternal, "msg service is not configured", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	resp, err := s.messages.GetLastMessage(ctx, in)
	if err != nil {
		return replyErr(c, req, err)
	}
	body, err := proto.Marshal(resp)
	if err != nil {
		return c.reply(req, errInternal, "marshal last message", nil)
	}
	return c.reply(req, 0, "", body)
}

func (s *Server) background(c *Client, req *Req) error {
	in := &msgpb.SetAppBackgroundStatusReq{}
	if len(req.Data) > 0 {
		if err := proto.Unmarshal(req.Data, in); err != nil {
			return c.reply(req, errArgs, "invalid background request", nil)
		}
	}
	c.background = in.IsBackground
	return c.reply(req, 0, "", nil)
}

func (s *Server) subscribe(c *Client, req *Req) error {
	in := &msgpb.SubUserOnlineStatus{}
	if len(req.Data) > 0 {
		if err := proto.Unmarshal(req.Data, in); err != nil {
			return c.reply(req, errArgs, "invalid subscribe request", nil)
		}
	}
	s.hub.SetSubs(c, in.SubscribeUserId, in.UnsubscribeUserId)
	tips := &msgpb.SubUserOnlineStatusTips{}
	for _, userID := range in.SubscribeUserId {
		tips.Subscribers = append(tips.Subscribers, &msgpb.SubUserOnlineStatusElem{
			UserId:            userID,
			OnlinePlatformIds: s.hub.Platforms(userID),
		})
	}
	body, err := proto.Marshal(tips)
	if err != nil {
		return c.reply(req, errInternal, "marshal online status", nil)
	}
	return c.reply(req, 0, "", body)
}
