package userclient

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/lisuhang11/SuIM/pkg/errorcode"
	"github.com/lisuhang11/SuIM/proto/userpb"
)

type stubUsers struct {
	userpb.UnimplementedUserServiceServer
	users map[string]*userpb.UserInfo
}

func (s stubUsers) GetUsersByIDs(context.Context, *userpb.GetUsersByIDsReq) (*userpb.GetUsersByIDsResp, error) {
	return &userpb.GetUsersByIDsResp{Users: s.users}, nil
}

func TestCheckUsers(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	gs := grpc.NewServer()
	userpb.RegisterUserServiceServer(gs, stubUsers{users: map[string]*userpb.UserInfo{
		"ada": {UserId: "ada"},
	}})
	go gs.Serve(lis)
	t.Cleanup(gs.Stop)

	conn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	c := New(userpb.NewUserServiceClient(conn))
	if err := c.CheckUsers(context.Background(), []string{"ada", "ada"}); err != nil {
		t.Fatal(err)
	}
	err = c.CheckUsers(context.Background(), []string{"ada", "missing"})
	got, ok := errorcode.CodeOf(err)
	if !ok || got != errorcode.RelationUserNotFound || status.Code(err) != codes.NotFound {
		t.Fatalf("missing user code=%v err=%v", status.Code(err), err)
	}
}
