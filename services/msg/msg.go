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

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/proto/group"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	"github.com/lisuhang11/SuIM/proto/relationpb"
	svcconfig "github.com/lisuhang11/SuIM/services/msg/internal/config"
	"github.com/lisuhang11/SuIM/services/msg/internal/logic"
	"github.com/lisuhang11/SuIM/services/msg/internal/redisclient"
	"github.com/lisuhang11/SuIM/services/msg/internal/rpc"
	"github.com/lisuhang11/SuIM/services/msg/internal/store"
)

func main() {
	cfgPath := flag.String("c", "services/msg/etc/msg.yaml", "config file path")
	migratePath := flag.String("migrate", "migrations/msg.sql", "sql file applied on startup")
	flag.Parse()
	if err := run(*cfgPath, *migratePath); err != nil {
		slog.Error("msg service stopped", "err", err)
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

	relationConn, err := reg.Dial(ctx, "relation", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial relation service: %w", err)
	}
	defer relationConn.Close()
	groupConn, err := reg.Dial(ctx, "group", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("dial group service: %w", err)
	}
	defer groupConn.Close()

	publisher := mq.NewRedisProducer(rdb, cfg.Queue.StreamPrefix, mq.TopicToRedis, cfg.Queue.MaxLen)
	srv := logic.New(
		st,
		publisher,
		rpc.NewRelationClient(relationpb.NewRelationServiceClient(relationConn)),
		rpc.NewGroupClient(grouppb.NewGroupServiceClient(groupConn)),
	)

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	grpcSrv := grpc.NewServer()
	msgpb.RegisterMsgServiceServer(grpcSrv, srv)

	id, err := newID()
	if err != nil {
		_ = lis.Close()
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.Port)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{ID: id, Name: "msg", Addr: addr})
	cancel()
	if err != nil {
		_ = lis.Close()
		return fmt.Errorf("register msg service: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("msg service listening", "addr", lis.Addr().String(), "register", addr)
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
