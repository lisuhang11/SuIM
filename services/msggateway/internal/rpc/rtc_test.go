package rpc

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
)

func TestAssembleForwardsCaller(t *testing.T) {
	const room = "room-1"
	capture := &captureRtc{room: room}
	lis := bufconn.Listen(1 << 20)
	srv := grpc.NewServer(grpc.UnaryInterceptor(rpcctx.ServerInterceptor))
	rtcpb.RegisterRtcServiceServer(srv, capture)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)

	conn, err := grpc.NewClient("passthrough:///rtc",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	out, err := NewRtcClient(rtcpb.NewRtcServiceClient(conn)).Assemble(context.Background(), "caller-1", "conn-1", &msgpb.CallSignal{
		Op: msgpb.CallOp_CALL_INVITE, PeerId: "callee-1", Media: msgpb.CallMedia_CALL_AUDIO,
	})
	if err != nil {
		t.Fatal(err)
	}
	if capture.user != "caller-1" || capture.conn != "conn-1" || capture.invitee != "callee-1" || capture.media != "audio" {
		t.Fatalf("user %s conn %s invitee %s media %s", capture.user, capture.conn, capture.invitee, capture.media)
	}
	if out.GetCallId() != room || out.GetCallerId() != "caller-1" || out.GetCalleeId() != "callee-1" || out.GetToken() != "" {
		t.Fatalf("reply %+v", out)
	}
}

func TestAssembleAcceptRejectsEmptyRoom(t *testing.T) {
	out, err := signalReply("callee-1", &msgpb.CallSignal{Op: msgpb.CallOp_CALL_ACCEPT, CallId: "room", Media: msgpb.CallMedia_CALL_AUDIO},
		&rtcpb.SignalResp{Payload: &rtcpb.SignalResp_Accept{Accept: &rtcpb.SignalAcceptResp{LiveUrl: "wss://x", Token: "t"}}})
	if err == nil || out != nil {
		t.Fatalf("out=%+v err=%v", out, err)
	}
}

type captureRtc struct {
	rtcpb.UnimplementedRtcServiceServer
	room    string
	user    string
	conn    string
	invitee string
	media   string
}

func (c *captureRtc) SignalMessageAssemble(ctx context.Context, req *rtcpb.SignalMessageAssembleReq) (*rtcpb.SignalMessageAssembleResp, error) {
	c.user = rpcctx.UserID(ctx)
	c.conn = req.GetConnId()
	c.invitee = req.GetSignalReq().GetInvite().GetInvitation().GetInviteeUserId()
	c.media = req.GetSignalReq().GetInvite().GetInvitation().GetMediaType()
	return &rtcpb.SignalMessageAssembleResp{SignalResp: &rtcpb.SignalResp{Payload: &rtcpb.SignalResp_Invite{
		Invite: &rtcpb.SignalInviteResp{RoomId: c.room},
	}}}, nil
}
