package ws

import (
	"context"
	"fmt"
	"sync"
	"testing"

	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
)

type scriptTokens struct {
	replaced []string
}

func (s *scriptTokens) Validate(_ context.Context, token string) (string, int32, error) {
	switch token {
	case "new", "old", "same":
		return "a", 5, nil
	default:
		return "", 0, fmt.Errorf("invalid token")
	}
}

func (s *scriptTokens) ReplacePlatform(_ context.Context, token string) error {
	s.replaced = append(s.replaced, token)
	return nil
}

type scriptPeers struct {
	addrs []string
}

func (s scriptPeers) Addrs(context.Context) ([]string, error) { return s.addrs, nil }

type scriptKick struct {
	mu    sync.Mutex
	addrs []string
}

func (s *scriptKick) Kick(_ context.Context, addr, _, _ string, _ int32) error {
	s.mu.Lock()
	s.addrs = append(s.addrs, addr)
	s.mu.Unlock()
	return nil
}

func TestKickPlatformDifferentTokenSendsKick(t *testing.T) {
	server := New(Options{Tokens: &scriptTokens{}})
	old := &Client{userID: "a", platformID: 5, token: "old", encoder: jsonEncoder{}, conn: &memSocket{}, server: server}
	server.hub.Add(old)
	if _, err := server.KickPlatform(context.Background(), &gatewaypb.KickPlatformReq{UserId: "a", PlatformId: 5, Token: "new"}); err != nil {
		t.Fatal(err)
	}
	if !old.kicked.Load() || !old.closed.Load() || !old.keepOnline.Load() {
		t.Fatalf("kicked=%v closed=%v keep=%v", old.kicked.Load(), old.closed.Load(), old.keepOnline.Load())
	}
	if got := decodeResp(t, old); got.ReqIdentifier != reqKickOnlineMsg {
		t.Fatalf("frame %d", got.ReqIdentifier)
	}
	if left := server.hub.Platform("a", 5); len(left) != 0 {
		t.Fatalf("still connected %d", len(left))
	}
}

func TestKickPlatformSameTokenDoesNotNotify(t *testing.T) {
	server := New(Options{Tokens: &scriptTokens{}})
	old := &Client{userID: "a", platformID: 5, token: "same", encoder: jsonEncoder{}, conn: &memSocket{}, server: server}
	server.hub.Add(old)
	if _, err := server.KickPlatform(context.Background(), &gatewaypb.KickPlatformReq{UserId: "a", PlatformId: 5, Token: "same"}); err != nil {
		t.Fatal(err)
	}
	if old.kicked.Load() || !old.closed.Load() || !old.keepOnline.Load() {
		t.Fatalf("kicked=%v closed=%v keep=%v", old.kicked.Load(), old.closed.Load(), old.keepOnline.Load())
	}
	sock := old.conn.(*memSocket)
	sock.mu.Lock()
	n := len(sock.writes)
	sock.mu.Unlock()
	if n != 0 {
		t.Fatalf("writes %d", n)
	}
}

func TestKickPlatformRejectsOtherToken(t *testing.T) {
	server := New(Options{Tokens: &scriptTokens{}})
	if _, err := server.KickPlatform(context.Background(), &gatewaypb.KickPlatformReq{UserId: "a", PlatformId: 5, Token: "nope"}); err == nil {
		t.Fatal("expected rejection")
	}
}

func TestClaimPlatformSkipsSelf(t *testing.T) {
	tokens := &scriptTokens{}
	kick := &scriptKick{}
	server := New(Options{
		Tokens:   tokens,
		Gateways: scriptPeers{addrs: []string{"127.0.0.1:1", "127.0.0.1:2"}},
		Kick:     kick,
		SelfAddr: "127.0.0.1:1",
	})
	server.claimPlatform(&Client{userID: "a", platformID: 5, token: "new"})
	if len(tokens.replaced) != 1 || tokens.replaced[0] != "new" {
		t.Fatalf("replaced %+v", tokens.replaced)
	}
	kick.mu.Lock()
	got := append([]string(nil), kick.addrs...)
	kick.mu.Unlock()
	if len(got) != 1 || got[0] != "127.0.0.1:2" {
		t.Fatalf("kicked %+v", got)
	}
}
