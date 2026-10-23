package ws

import (
	"context"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/msgconv"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

func (s *Server) dispatch(c *Client, req *Req) error {
	if req.SendID != "" && req.SendID != c.userID {
		return c.reply(req, errArgs, "sendID does not match the connection", nil)
	}
	switch req.ReqIdentifier {
	case reqGetNewestSeq:
		return s.maxSeq(c, req)
	case reqPullMsgBySeqList:
		return s.pullBySeqList(c, req)
	case reqSendMsg:
		return s.sendMsg(c, req)
	case reqPullMsg:
		return s.pullByRange(c, req)
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
	switch msg.SessionType {
	case 1, 4:
		if msg.RecvId == "" {
			return c.reply(req, errArgs, "recv id is required", nil)
		}
	case 2, 3:
		if msg.GroupId == "" {
			return c.reply(req, errArgs, "group id is required", nil)
		}
	default:
		return c.reply(req, errArgs, "session type is required", nil)
	}
	if len(msg.Content) == 0 {
		return c.reply(req, errArgs, "content is required", nil)
	}
	key := msgconv.QueueKey(msg)
	if key == "" {
		return c.reply(req, errArgs, "conversation id is empty", nil)
	}
	if msg.ClientMsgId == "" {
		msg.ClientMsgId = newConnID()
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
		return c.reply(req, errInternal, "marshal message", nil)
	}
	if s.publish == nil {
		return c.reply(req, errInternal, "queue is not configured", nil)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.publish.Send(ctx, key, body); err != nil {
		return c.reply(req, errInternal, "queue send failed", nil)
	}
	resp, err := proto.Marshal(&msgpb.SendMsgResp{
		ServerMsgId: msg.ServerMsgId,
		ClientMsgId: msg.ClientMsgId,
		SendTime:    msg.SendTime,
	})
	if err != nil {
		return c.reply(req, errInternal, "marshal response", nil)
	}
	return c.reply(req, 0, "", resp)
}

func (s *Server) maxSeq(c *Client, req *Req) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids := s.userConversations(ctx, c.userID)
	maxSeq := map[string]int64{}
	var err error
	if s.history != nil {
		maxSeq, err = s.history.MaxSeqs(ctx, ids)
		if err != nil {
			return c.reply(req, errInternal, "query max seq", nil)
		}
	}
	minSeq := map[string]int64{}
	for id, seq := range maxSeq {
		if seq > 0 {
			minSeq[id] = 1
		}
	}
	body, err := proto.Marshal(&msgpb.GetMaxSeqResp{MaxSeqs: maxSeq, MinSeqs: minSeq})
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids := in.ConversationIds
	if len(ids) == 0 {
		ids = s.userConversations(ctx, c.userID)
	}
	out := &msgpb.GetConversationsHasReadAndMaxSeqResp{Seqs: map[string]*msgpb.Seqs{}}
	if s.history != nil {
		state, err := s.history.SeqState(ctx, c.userID, ids)
		if err != nil {
			return c.reply(req, errInternal, "query read seq", nil)
		}
		for id, item := range state {
			out.Seqs[id] = &msgpb.Seqs{MaxSeq: item.MaxSeq, HasReadSeq: item.HasReadSeq}
		}
	}
	body, err := proto.Marshal(out)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out := &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{}, NotificationMsgs: map[string]*msgpb.PullMsgs{}}
	for _, item := range in.SeqRanges {
		if item == nil || item.ConversationId == "" || s.history == nil {
			continue
		}
		msgs, err := s.history.Range(ctx, item.ConversationId, item.Begin, item.End, item.Num)
		if err != nil {
			return c.reply(req, errInternal, "query messages", nil)
		}
		putPull(out, item.ConversationId, &msgpb.PullMsgs{Msgs: msgs, IsEnd: true, EndSeq: item.End})
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out := &msgpb.PushMessages{Msgs: map[string]*msgpb.PullMsgs{}, NotificationMsgs: map[string]*msgpb.PullMsgs{}}
	for _, item := range in.Conversations {
		if item == nil || item.ConversationId == "" || s.history == nil {
			continue
		}
		msgs, err := s.history.BySeqs(ctx, item.ConversationId, item.Seqs)
		if err != nil {
			return c.reply(req, errInternal, "query messages", nil)
		}
		putPull(out, item.ConversationId, &msgpb.PullMsgs{Msgs: msgs, IsEnd: true})
	}
	body, err := proto.Marshal(out)
	if err != nil {
		return c.reply(req, errInternal, "marshal messages", nil)
	}
	return c.reply(req, 0, "", body)
}

func (s *Server) lastMessages(c *Client, req *Req) error {
	in := &msgpb.GetLastMessageReq{}
	if len(req.Data) > 0 {
		if err := proto.Unmarshal(req.Data, in); err != nil {
			return c.reply(req, errArgs, "invalid last message request", nil)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ids := in.ConversationIds
	if len(ids) == 0 {
		ids = s.userConversations(ctx, c.userID)
	}
	out := &msgpb.GetLastMessageResp{Msgs: map[string]*msgpb.MsgData{}}
	for _, id := range ids {
		if s.history == nil || id == "" {
			continue
		}
		msg, err := s.history.Last(ctx, id)
		if err != nil {
			return c.reply(req, errInternal, "query last message", nil)
		}
		if msg != nil {
			out.Msgs[id] = msg
		}
	}
	body, err := proto.Marshal(out)
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

func (s *Server) userConversations(ctx context.Context, userID string) []string {
	seen := map[string]struct{}{}
	var ids []string
	add := func(id string) {
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if s.history != nil {
		found, err := s.history.ConversationIDs(ctx, userID)
		if err == nil {
			for _, id := range found {
				add(id)
			}
		}
	}
	if s.groups != nil {
		joined, err := s.groups.Joined(ctx, userID)
		if err == nil {
			for _, group := range joined {
				add(msgconv.GroupConversationID(group.ID, group.Super))
			}
		}
	}
	return ids
}

func putPull(out *msgpb.PushMessages, conversationID string, pull *msgpb.PullMsgs) {
	if msgconv.IsNotificationConversation(conversationID) {
		out.NotificationMsgs[conversationID] = pull
		return
	}
	out.Msgs[conversationID] = pull
}
