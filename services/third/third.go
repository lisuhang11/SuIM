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

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/services/third/internal/blob"
	svcconfig "github.com/lisuhang11/SuIM/services/third/internal/config"
	"github.com/lisuhang11/SuIM/services/third/internal/logic"
	"github.com/lisuhang11/SuIM/services/third/internal/store"
)

func main() {
	cfgPath := flag.String("c", "services/third/etc/third.yaml", "config file path")
	migratePath := flag.String("migrate", "migrations/third.sql", "sql file applied on startup")
	flag.Parse()

	if err := run(*cfgPath, *migratePath); err != nil {
		slog.Error("third service stopped", "err", err)
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

	rdb, err := store.NewRedis(cfg.Redis)
	if err != nil {
		return err
	}
	st, err := store.Open(ctx, mysqlDSN(cfg.MySQL), cfg.MySQL.MaxOpenConn, cfg.MySQL.MaxIdleConn, rdb)
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

	objects, err := blob.Open(ctx, blob.Config{
		Endpoint:     cfg.MinIO.Endpoint,
		SignEndpoint: cfg.MinIO.SignEndpoint,
		Bucket:       cfg.MinIO.Bucket,
		AccessKey:    cfg.MinIO.AccessKey,
		SecretKey:    cfg.MinIO.SecretKey,
	})
	if err != nil {
		return err
	}
	svc := logic.New(st, st, objects, logic.Options{
		Limits: logic.DefaultLimits,
		Expire: time.Duration(cfg.SignExpireSeconds) * time.Second,
	})

	reg, err := register.New(&cfg.Registry)
	if err != nil {
		return err
	}
	defer reg.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	grpcSrv := grpc.NewServer()
	thirdpb.RegisterThirdServiceServer(grpcSrv, svc)

	id, err := newID()
	if err != nil {
		_ = lis.Close()
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.Port)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{ID: id, Name: "third", Addr: addr})
	cancel()
	if err != nil {
		_ = lis.Close()
		return fmt.Errorf("register third service: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("third service listening", "addr", lis.Addr().String(), "register", addr, "bucket", cfg.MinIO.Bucket)
		errCh <- grpcSrv.Serve(lis)
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
