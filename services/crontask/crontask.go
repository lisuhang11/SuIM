package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	svcconfig "github.com/lisuhang11/SuIM/services/crontask/internal/config"
	cronpkg "github.com/lisuhang11/SuIM/services/crontask/internal/cron"
	"github.com/lisuhang11/SuIM/services/crontask/internal/redisclient"
)

func main() {
	cfgPath := flag.String("c", "services/crontask/etc/crontask.yaml", "config file path")
	flag.Parse()
	if err := run(*cfgPath); err != nil {
		slog.Error("crontask service stopped", "err", err)
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

	dial := func(name string) (*grpc.ClientConn, error) {
		return reg.Dial(ctx, name,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
		)
	}
	msgConn, err := dial("msg")
	if err != nil {
		return fmt.Errorf("dial msg: %w", err)
	}
	defer msgConn.Close()
	thirdConn, err := dial("third")
	if err != nil {
		return fmt.Errorf("dial third: %w", err)
	}
	defer thirdConn.Close()
	conversationConn, err := dial("conversation")
	if err != nil {
		return fmt.Errorf("dial conversation: %w", err)
	}
	defer conversationConn.Close()

	slog.Info("crontask service starting",
		"cron", cfg.CronTask.CronExecuteTime,
		"retainDays", cfg.CronTask.RetainChatRecords,
		"fileExpireDays", cfg.CronTask.FileExpireTime)

	// Give dialed services a moment when process starts together.
	time.Sleep(100 * time.Millisecond)
	return cronpkg.Start(ctx, &cfg, msgConn, thirdConn, conversationConn, cronpkg.NewRedisLocker(rdb))
}
