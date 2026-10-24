package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/proto/group"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	gatewaypb "github.com/lisuhang11/SuIM/proto/msggateway"
	"github.com/lisuhang11/SuIM/proto/userpb"
	svcconfig "github.com/lisuhang11/SuIM/services/msggateway/internal/config"
	"github.com/lisuhang11/SuIM/services/msggateway/internal/redisclient"
	"github.com/lisuhang11/SuIM/services/msggateway/internal/rpc"
	"github.com/lisuhang11/SuIM/services/msggateway/internal/ws"
)

func main() {
	cfgPath := flag.String("c", "services/msggateway/etc/msggateway.yaml", "config file path")
	flag.Parse()
	if err := run(*cfgPath); err != nil {
		slog.Error("msggateway stopped", "err", err)
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

	userConn, err := reg.Dial(ctx, "user", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial user service: %w", err)
	}
	defer userConn.Close()
	msgConn, err := reg.Dial(ctx, "msg", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial msg service: %w", err)
	}
	defer msgConn.Close()
	groupConn, err := reg.Dial(ctx, "group", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial group service: %w", err)
	}
	defer groupConn.Close()

	server := ws.New(ws.Options{
		Tokens:           rpc.NewUserClient(userpb.NewUserServiceClient(userConn)),
		Groups:           groups{rpc.NewGroupClient(grouppb.NewGroupServiceClient(groupConn))},
		Messages:         rpc.NewMsgClient(msgpb.NewMsgServiceClient(msgConn)),
		MaxConn:          cfg.MaxConn,
		HandshakeTimeout: time.Duration(cfg.HandshakeTimeoutSec) * time.Second,
	})

	wsLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.WSPort))
	if err != nil {
		return fmt.Errorf("listen websocket: %w", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /msg/send", server.HandleSend)
	mux.HandleFunc("POST /msg/pull", server.HandlePull)
	mux.Handle("/", server)
	wsSrv := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	rpcLis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.RPCPort))
	if err != nil {
		_ = wsLis.Close()
		return fmt.Errorf("listen rpc: %w", err)
	}
	rpcSrv := grpc.NewServer()
	gatewaypb.RegisterMsgGatewayServer(rpcSrv, server)

	id, err := newID()
	if err != nil {
		_ = wsLis.Close()
		_ = rpcLis.Close()
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.RPCPort)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{ID: id, Name: "msggateway", Addr: addr})
	cancel()
	if err != nil {
		_ = wsLis.Close()
		_ = rpcLis.Close()
		return fmt.Errorf("register msggateway: %w", err)
	}

	pushConsumer := mq.NewRedisConsumer(rdb, cfg.Queue.StreamPrefix, mq.TopicToPush, "msggateway")
	pushConsumer.ConsumeNew()

	errCh := make(chan error, 3)
	go func() {
		slog.Info("msggateway websocket listening", "addr", wsLis.Addr().String())
		errCh <- wsSrv.Serve(wsLis)
	}()
	go func() {
		slog.Info("msggateway rpc listening", "addr", rpcLis.Addr().String(), "register", addr)
		errCh <- rpcSrv.Serve(rpcLis)
	}()
	go func() {
		errCh <- pushConsumer.Subscribe(ctx, server.HandlePush)
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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = wsSrv.Shutdown(shutdownCtx)
	rpcSrv.GracefulStop()
	if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, http.ErrServerClosed) {
		return runErr
	}
	return nil
}

type groups struct {
	*rpc.GroupClient
}

func (g groups) Joined(ctx context.Context, userID string) ([]ws.JoinedGroup, error) {
	joined, err := g.GroupClient.Joined(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]ws.JoinedGroup, len(joined))
	for i, item := range joined {
		out[i] = ws.JoinedGroup{ID: item.ID, Super: item.Super}
	}
	return out, nil
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
