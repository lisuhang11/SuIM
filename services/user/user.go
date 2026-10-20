package main

import (
	"context"
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
	"github.com/lisuhang11/SuIM/proto/userpb"
	svcconfig "github.com/lisuhang11/SuIM/services/user/internal/config"
	"github.com/lisuhang11/SuIM/services/user/internal/logic"
	"github.com/lisuhang11/SuIM/services/user/internal/store"
	"github.com/lisuhang11/SuIM/services/user/internal/token"
)

func main() {
	cfgPath := flag.String("c", "services/user/etc/user.yaml", "config file path")
	migratePath := flag.String("migrate", "migrations/user.sql", "sql file applied on startup")
	flag.Parse()

	if err := run(*cfgPath, *migratePath); err != nil {
		slog.Error("user service stopped", "err", err)
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

	reg, err := register.New(&cfg.Registry)
	if err != nil {
		return err
	}
	defer reg.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	tokenCache, stopCache := logic.NewTokenCache(ctx, rdb, cfg.LocalCache)
	defer stopCache()

	srv := grpc.NewServer()
	userpb.RegisterUserServiceServer(srv, logic.New(st, token.NewIssuer(cfg.JWT), tokenCache))

	id, err := logic.NewID()
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.Port)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{
		ID:   id,
		Name: "user",
		Addr: addr,
	})
	cancel()
	if err != nil {
		_ = lis.Close()
		return fmt.Errorf("register user service: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("user service listening", "addr", lis.Addr().String(), "register", addr)
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
