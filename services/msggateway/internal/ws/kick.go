package ws

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
)

// GatewayPeers lists every msggateway address, including this process.
type GatewayPeers interface {
	Addrs(ctx context.Context) ([]string, error)
}

// GatewayKick closes one user platform on a single msggateway address.
type GatewayKick interface {
	Kick(ctx context.Context, addr, userID, token string, platformID int32) error
}

type platformReplacer interface {
	ReplacePlatform(ctx context.Context, token string) error
}

// KickPlatform closes sockets this process still holds for the user and platform.
// The token must be a live login for that same user and platform.
func (s *Server) KickPlatform(ctx context.Context, req *gatewaypb.KickPlatformReq) (*gatewaypb.KickPlatformResp, error) {
	if s == nil || s.tokens == nil || req == nil || req.GetUserId() == "" || req.GetPlatformId() <= 0 || req.GetToken() == "" {
		return nil, status.Error(codes.InvalidArgument, "user, platform, and token are required")
	}
	userID, platformID, err := s.tokens.Validate(ctx, req.GetToken())
	if err != nil || userID != req.GetUserId() || platformID != req.GetPlatformId() {
		return nil, status.Error(codes.PermissionDenied, "token does not match")
	}
	for _, old := range s.hub.Platform(userID, int(platformID)) {
		old.replaced(req.GetToken())
	}
	return &gatewaypb.KickPlatformResp{}, nil
}

// claimPlatform tells every other gateway to drop this user and platform, and
// revokes the older tokens on that platform. The shared online record is not
// cleared by those remote closes.
func (s *Server) claimPlatform(client *Client) {
	if s == nil || client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if replacer, ok := s.tokens.(platformReplacer); ok && client.token != "" {
		if err := replacer.ReplacePlatform(ctx, client.token); err != nil {
			slog.Warn("replace platform session", "err", err, "userID", client.userID, "platformID", client.platformID)
		}
	}
	if s.peers == nil || s.kick == nil {
		return
	}
	addrs, err := s.peers.Addrs(ctx)
	if err != nil {
		slog.Warn("list gateways", "err", err)
		return
	}
	var wg sync.WaitGroup
	for _, addr := range addrs {
		if addr == "" || addr == s.selfAddr {
			continue
		}
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			if err := s.kick.Kick(ctx, addr, client.userID, client.token, int32(client.platformID)); err != nil {
				slog.Warn("kick other gateway", "err", err, "addr", addr, "userID", client.userID)
			}
		}(addr)
	}
	wg.Wait()
}
