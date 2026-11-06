package ws

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/errorcode"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
)

// CallSignals is the rtc service. The gateway authenticates the socket, then forwards frame 1004.
type CallSignals interface {
	Assemble(ctx context.Context, userID, connID string, in *msgpb.CallSignal) (*msgpb.CallSignal, error)
}

func (s *Server) signal(c *Client, req *Req) error {
	if s == nil || s.signals == nil {
		return c.reply(req, errArgs, "signal messages are not supported", nil)
	}
	in := &msgpb.CallSignal{}
	if len(req.Data) == 0 || proto.Unmarshal(req.Data, in) != nil {
		return replyCall(c, req, errorcode.ErrCallInvalid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := s.signals.Assemble(ctx, c.userID, c.id, in)
	if err != nil {
		return replyCall(c, req, err)
	}
	if out == nil {
		return replyCall(c, req, errorcode.ErrInternal.WithMsg("empty rtc response"))
	}
	body, err := proto.Marshal(out)
	if err != nil {
		return c.reply(req, errInternal, "marshal call signal", nil)
	}
	return c.reply(req, 0, "", body)
}

func replyCall(c *Client, req *Req, err error) error {
	if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
		return c.reply(req, errInternal, "rtc service is unavailable", nil)
	}
	code := errInternal
	text := "call signaling"
	if biz, ok := errorcode.CodeOf(err); ok {
		code = int(biz)
		text = status.Convert(err).Message()
	}
	return c.reply(req, code, text, nil)
}

// WatchCall delivers signals published by the rtc service on this process and on other gateways.
func (s *Server) WatchCall(ctx context.Context) {
	if s == nil || s.calls == nil {
		return
	}
	s.calls.Subscribe(ctx, s.deliverCall)
}

func (s *Server) deliverCall(ev call.Event) {
	body, err := proto.Marshal(signalProto(call.Signal{
		Op: ev.Op, CallID: ev.CallID, CallerID: ev.Caller, CalleeID: ev.Callee, Media: ev.Media, Reason: ev.Reason,
		URL: ev.URL, Token: ev.Token,
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

func signalProto(sig call.Signal) *msgpb.CallSignal {
	return &msgpb.CallSignal{
		Op:       callOp(sig.Op),
		CallId:   sig.CallID,
		Media:    callMedia(sig.Media),
		CallerId: sig.CallerID,
		CalleeId: sig.CalleeID,
		Reason:   sig.Reason,
		Url:      sig.URL,
		Token:    sig.Token,
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
