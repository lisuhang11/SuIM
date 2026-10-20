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
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	"github.com/lisuhang11/SuIM/proto/userpb"
	svcconfig "github.com/lisuhang11/SuIM/services/relation/internal/config"
	"github.com/lisuhang11/SuIM/services/relation/internal/logic"
	"github.com/lisuhang11/SuIM/services/relation/internal/store"
	"github.com/lisuhang11/SuIM/services/relation/internal/userclient"
)

func main() {
	cfgPath := flag.String("c", "services/relation/etc/relation.yaml", "config file path")
	migratePath := flag.String("migrate", "migrations/relation.sql", "sql file applied on startup")
	flag.Parse()

	if err := run(*cfgPath, *migratePath); err != nil {
		slog.Error("relation service stopped", "err", err)
		os.Exit(1)
	}
}

func run(cfgPath, migratePath string) error {
	var cfg svcconfig.Config
	if err := config.Load(cfgPath, &cfg); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, mysqlDSN(cfg.MySQL), cfg.MySQL.MaxOpenConn, cfg.MySQL.MaxIdleConn)
	if err != nil {
		return err
	}
	defer st.Close()

	script, err := os.ReadFile(migratePath)
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		return err
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

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	var hooks *webhook.Client
	if cfg.Webhooks.URL != "" {
		hooks = webhook.NewWebhookClient(cfg.Webhooks.URL)
		defer hooks.Close()
	}

	srv := grpc.NewServer()
	relationpb.RegisterRelationServiceServer(srv, logic.New(st, userclient.New(userpb.NewUserServiceClient(userConn)), logic.Hooks{
		Client:           hooks,
		BeforeAddBlack:   cfg.Webhooks.BeforeAddBlack,
		AfterRemoveBlack: cfg.Webhooks.AfterRemoveBlack,
	}))

	id, err := newID()
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.Port)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{
		ID:   id,
		Name: "relation",
		Addr: addr,
	})
	cancel()
	if err != nil {
		_ = lis.Close()
		return fmt.Errorf("register relation service: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("relation service listening", "addr", lis.Addr().String(), "register", addr)
		errCh <- srv.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		offCtx, offCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer offCancel()
		_ = reg.Deregister(offCtx)
		srv.GracefulStop()
		return nil
	case err := <-errCh:
		offCtx, offCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer offCancel()
		_ = reg.Deregister(offCtx)
		return err
	}
}

func mysqlDSN(cfg config.MySQL) string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4&loc=UTC&timeout=5s",
		cfg.Username, cfg.Password, cfg.Address[0], cfg.Database)
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
