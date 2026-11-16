package main

import (
	"context"
	"crypto/rand"
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

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/mw"
	"github.com/lisuhang11/SuIM/pkg/online"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	pushpb "github.com/lisuhang11/SuIM/proto/push"
	svcconfig "github.com/lisuhang11/SuIM/services/push/internal/config"
	"github.com/lisuhang11/SuIM/services/push/internal/logic"
	"github.com/lisuhang11/SuIM/services/push/internal/offlinepush"
	"github.com/lisuhang11/SuIM/services/push/internal/rpc"
	"github.com/lisuhang11/SuIM/services/push/internal/store"
)

func main() {
	cfgPath := flag.String("c", "services/push/etc/push.yaml", "config file path")
	flag.Parse()
	if err := run(*cfgPath); err != nil {
		slog.Error("push service stopped", "err", err)
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

	rdb, err := store.NewRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer rdb.Close()

	tokens := store.NewTokenStore(rdb)
	pusher, err := offlinepush.NewPusher(&cfg.Push, tokens)
	if err != nil {
		return fmt.Errorf("create offline pusher: %w", err)
	}

	reg, err := register.New(&cfg.Registry)
	if err != nil {
		return err
	}
	defer reg.Close()

	groupConn, err := reg.Dial(ctx, "group",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial group service: %w", err)
	}
	defer groupConn.Close()

	svc := logic.New(rpc.NewGroupClient(groupConn), online.New(rdb), pusher, tokens)
	if cfg.Webhooks.URL != "" {
		client := webhook.NewWebhookClient(cfg.Webhooks.URL)
		defer client.Close()
		svc.UseHooks(logic.Hooks{Client: client, BeforeOfflinePush: cfg.Webhooks.BeforeOfflinePush})
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	grpcSrv := grpc.NewServer(grpc.ChainUnaryInterceptor(mw.UnaryRecover, rpcctx.ServerInterceptor))
	pushpb.RegisterPushServiceServer(grpcSrv, svc)

	id, err := newID()
	if err != nil {
		_ = lis.Close()
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.Port)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{ID: id, Name: "push", Addr: addr})
	cancel()
	if err != nil {
		_ = lis.Close()
		return fmt.Errorf("register push service: %w", err)
	}

	consumer := mq.NewRedisConsumer(rdb, cfg.Queue.StreamPrefix, mq.TopicToPush, cfg.Queue.ConsumerGroup)
	consumer.ConsumeNew()
	errCh := make(chan error, 2)
	go func() {
		slog.Info("push service listening", "addr", lis.Addr().String(), "register", addr, "provider", cfg.Push.Enable)
		errCh <- grpcSrv.Serve(lis)
	}()
	go func() {
		slog.Info("begin consume toPush", "group", cfg.Queue.ConsumerGroup)
		errCh <- consumer.Subscribe(ctx, svc.HandlePush)
	}()

	select {
	case <-ctx.Done():
		offCtx, offCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer offCancel()
		_ = reg.Deregister(offCtx)
		grpcSrv.GracefulStop()
		return nil
	case err := <-errCh:
		offCtx, offCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer offCancel()
		_ = reg.Deregister(offCtx)
		return err
	}
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
