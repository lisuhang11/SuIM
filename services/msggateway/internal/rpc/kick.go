package rpc

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	_ "google.golang.org/grpc/resolver/passthrough"

	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
)

// GatewayKick asks one msggateway process to close its local sockets.
type GatewayKick struct{}

func (GatewayKick) Kick(ctx context.Context, addr, userID, token string, platformID int32) error {
	conn, err := grpc.NewClient("passthrough:///"+addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial gateway %s: %w", addr, err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err = gatewaypb.NewMsgGatewayClient(conn).KickPlatform(ctx, &gatewaypb.KickPlatformReq{
		UserId:     userID,
		PlatformId: platformID,
		Token:      token,
	})
	if err != nil {
		return fmt.Errorf("kick %s: %w", addr, err)
	}
	return nil
}
