package ws

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// Relations is the friend and blacklist check used before a call rings.
type Relations interface {
	IsFriend(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
	IsBlack(ctx context.Context, user1, user2 string) (inUser1, inUser2 bool, err error)
}

func (s *Server) signal(c *Client, req *Req) error {
	if s == nil || s.calls == nil {
		return c.reply(req, errArgs, "signal messages are not supported", nil)
	}
	in := &msgpb.CallSignal{}
	if len(req.Data) == 0 || proto.Unmarshal(req.Data, in) != nil {
		return replyCall(c, req, errorcode.ErrCallInvalid, call.Signal{})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	sig, err := s.applySignal(ctx, c, in)
	if err != nil {
		return replyCall(c, req, err, sig)
	}
	s.fanout(ctx, c, sig)
	return replyCall(c, req, nil, sig)
}

func (s *Server) applySignal(ctx context.Context, c *Client, in *msgpb.CallSignal) (call.Signal, error) {
	switch in.GetOp() {
	case msgpb.CallOp_CALL_INVITE:
		if err := s.authorizeCall(ctx, c.userID, in.GetPeerId()); err != nil {
			return call.Signal{}, err
		}
		return s.calls.Invite(ctx, c.userID, in.GetPeerId(), mediaName(in.GetMedia()))
	case msgpb.CallOp_CALL_CANCEL:
		return s.calls.Cancel(ctx, c.userID, in.GetCallId())
	case msgpb.CallOp_CALL_ACCEPT:
		return s.calls.Accept(ctx, c.userID, in.GetCallId())
	case msgpb.CallOp_CALL_REJECT:
		return s.calls.Reject(ctx, c.userID, in.GetCallId())
	case msgpb.CallOp_CALL_HANGUP:
		return s.calls.Hangup(ctx, c.userID, in.GetCallId())
	default:
		return call.Signal{}, errorcode.ErrCallInvalid
	}
}

func (s *Server) authorizeCall(ctx context.Context, caller, callee string) error {
	if s.relations == nil {
		return errorcode.ErrInternal.WithMsg("relation service is not configured")
	}
	blockedByCaller, blockedByCallee, err := s.relations.IsBlack(ctx, caller, callee)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check black")
	}
	if blockedByCaller || blockedByCallee {
		return errorcode.ErrMsgBlocked
	}
	callerHasFriend, calleeHasFriend, err := s.relations.IsFriend(ctx, caller, callee)
	if err != nil {
		return errorcode.ErrInternal.WithMsg("check friend")
	}
	if !callerHasFriend || !calleeHasFriend {
		return errorcode.ErrMsgNotFriend
	}
	return nil
}

func (s *Server) fanout(ctx context.Context, actor *Client, sig call.Signal) {
	switch sig.Op {
	case call.OpInvite:
		s.publishCall(ctx, sig, call.OpRing, []string{sig.CalleeID}, "")
	case call.OpCancel:
		s.publishCall(ctx, sig, call.OpCancel, []string{sig.CalleeID}, "")
	case call.OpReject:
		s.publishCall(ctx, sig, call.OpReject, []string{sig.CallerID}, "")
	case call.OpAccept:
		s.publishCall(ctx, sig, call.OpAccept, []string{sig.CallerID}, "")
		s.publishCall(ctx, sig, call.OpAnswered, []string{sig.CalleeID}, actor.id)
	case call.OpHangup:
		peer := sig.CallerID
		if actor.userID == sig.CallerID {
			peer = sig.CalleeID
		}
		s.publishCall(ctx, sig, call.OpHangup, []string{peer}, "")
	}
}

func (s *Server) publishCall(ctx context.Context, sig call.Signal, op string, users []string, except string) {
	out := sig
	out.Op = op
	if err := s.calls.Publish(ctx, call.Event{
		Users:  users,
		Except: except,
		Op:     out.Op,
		CallID: out.CallID,
		Caller: out.CallerID,
		Callee: out.CalleeID,
		Media:  out.Media,
		Reason: out.Reason,
	}); err != nil {
		slog.Warn("publish call signal", "err", err, "op", op, "callID", sig.CallID)
	}
}

// WatchCall delivers signals published by this process and by other gateways.
func (s *Server) WatchCall(ctx context.Context) {
	if s == nil || s.calls == nil {
		return
	}
	s.calls.Subscribe(ctx, s.deliverCall)
}

// SweepCalls ends ringing calls that nobody answered.
func (s *Server) SweepCalls(ctx context.Context) {
	if s == nil || s.calls == nil {
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ended, err := s.calls.Expire(ctx, time.Now())
			if err != nil {
				slog.Warn("expire calls", "err", err)
				continue
			}
			for _, sig := range ended {
				s.publishCall(ctx, sig, call.OpTimeout, []string{sig.CallerID, sig.CalleeID}, "")
			}
		}
	}
}

func (s *Server) deliverCall(ev call.Event) {
	body, err := proto.Marshal(signalProto(call.Signal{
		Op: ev.Op, CallID: ev.CallID, CallerID: ev.Caller, CalleeID: ev.Callee, Media: ev.Media, Reason: ev.Reason,
	}))
	if err != nil {
		slog.Warn("marshal call signal", "err", err)
		return
	}
	for _, userID := range ev.Users {
		for _, client := range s.hub.Clients(userID) {
			if client == nil || client.id == ev.Except {
				continue
			}
			_ = client.write(Resp{ReqIdentifier: reqCallSignal, Data: body})
		}
	}
}

func replyCall(c *Client, req *Req, err error, sig call.Signal) error {
	if err == nil {
		body, marshalErr := proto.Marshal(signalProto(sig))
		if marshalErr != nil {
			return c.reply(req, errInternal, "marshal call signal", nil)
		}
		return c.reply(req, 0, "", body)
	}
	code := errInternal
	text := "call signaling"
	if biz, ok := errorcode.CodeOf(err); ok {
		code = int(biz)
		text = err.Error()
	}
	var body []byte
	if sig.CallID != "" {
		body, _ = proto.Marshal(signalProto(sig))
	}
	return c.reply(req, code, text, body)
}

func signalProto(sig call.Signal) *msgpb.CallSignal {
	return &msgpb.CallSignal{
		Op:       callOp(sig.Op),
		CallId:   sig.CallID,
		Media:    callMedia(sig.Media),
		CallerId: sig.CallerID,
		CalleeId: sig.CalleeID,
		Reason:   sig.Reason,
	}
}

func callOp(op string) msgpb.CallOp {
	switch op {
	case call.OpInvite:
		return msgpb.CallOp_CALL_INVITE
	case call.OpCancel:
		return msgpb.CallOp_CALL_CANCEL
	case call.OpAccept:
		return msgpb.CallOp_CALL_ACCEPT
	case call.OpReject:
		return msgpb.CallOp_CALL_REJECT
	case call.OpHangup:
		return msgpb.CallOp_CALL_HANGUP
	case call.OpRing:
		return msgpb.CallOp_CALL_RING
	case call.OpTimeout:
		return msgpb.CallOp_CALL_TIMEOUT
	case call.OpBusy:
		return msgpb.CallOp_CALL_BUSY
	case call.OpAnswered:
		return msgpb.CallOp_CALL_ANSWERED
	default:
		return msgpb.CallOp_CALL_OP_NONE
	}
}

func callMedia(media string) msgpb.CallMedia {
	switch media {
	case call.MediaAudio:
		return msgpb.CallMedia_CALL_AUDIO
	case call.MediaVideo:
		return msgpb.CallMedia_CALL_VIDEO
	default:
		return msgpb.CallMedia_CALL_MEDIA_NONE
	}
}

func mediaName(media msgpb.CallMedia) string {
	switch media {
	case msgpb.CallMedia_CALL_AUDIO:
		return call.MediaAudio
	case msgpb.CallMedia_CALL_VIDEO:
		return call.MediaVideo
	default:
		return ""
	}
}
