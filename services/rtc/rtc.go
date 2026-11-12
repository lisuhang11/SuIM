package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lisuhang11/SuIM/pkg/call"
	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/mw"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	grouppb "github.com/lisuhang11/SuIM/proto/group"
	relationpb "github.com/lisuhang11/SuIM/proto/relation"
	rtcpb "github.com/lisuhang11/SuIM/proto/rtc"
	svcconfig "github.com/lisuhang11/SuIM/services/rtc/internal/config"
	"github.com/lisuhang11/SuIM/services/rtc/internal/logic"
	"github.com/lisuhang11/SuIM/services/rtc/internal/redisclient"
	"github.com/lisuhang11/SuIM/services/rtc/internal/rpc"
)

func main() {
	cfgPath := flag.String("c", "services/rtc/etc/rtc.yaml", "config file path")
	flag.Parse()
	if err := run(*cfgPath); err != nil {
		slog.Error("rtc service stopped", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath string) error {
	var cfg svcconfig.Config
	if err := config.Load(cfgPath, &cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rdb, err := redisclient.New(cfg.Redis)
	if err != nil {
		return err
	}
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}

	reg, err := register.New(&cfg.Registry)
	if err != nil {
		return err
	}
	defer reg.Close()

	relationConn, err := reg.Dial(ctx, "relation",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial relation service: %w", err)
	}
	defer relationConn.Close()

	groupConn, err := reg.Dial(ctx, "group",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial group service: %w", err)
	}
	defer groupConn.Close()

	service := logic.New(call.New(rdb), rpc.NewRelationClient(relationpb.NewRelationServiceClient(relationConn)), cfg.LiveKit)
	service.UseGroups(rpc.NewGroupClient(grouppb.NewGroupServiceClient(groupConn)))
	if cfg.Agents.Enabled {
		stream := cfg.Agents.Stream
		if stream == "" {
			stream = "suim:agent:dispatch"
		}
		service.UseAgents(cfg.Agents, logic.NewRedisAgentDispatcher(rdb, stream))
		slog.Info("livekit agents dispatch enabled", "stream", stream, "bots", len(cfg.Agents.BotUserIDs))
	}
	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(mw.UnaryRecover, rpcctx.ServerInterceptor))
	rtcpb.RegisterRtcServiceServer(srv, service)

	id, err := newID()
	if err != nil {
		_ = lis.Close()
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.Port)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{ID: id, Name: "rtc", Addr: addr})
	cancel()
	if err != nil {
		_ = lis.Close()
		return fmt.Errorf("register rtc service: %w", err)
	}

	go service.Sweep(ctx)
	errCh := make(chan error, 1)
	go func() {
		slog.Info("rtc service listening", "addr", lis.Addr().String(), "register", addr)
		errCh <- srv.Serve(lis)
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
		stop()
	}
	offCtx, offCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer offCancel()
	_ = reg.Deregister(offCtx)
	srv.GracefulStop()
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return runErr
	}
	return nil
}

func newID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}
