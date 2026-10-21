package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"

	_ "github.com/go-sql-driver/mysql"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/mq"
	"github.com/lisuhang11/SuIM/pkg/webhook"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/cache"
	svcconfig "github.com/lisuhang11/SuIM/services/msgtransfer/internal/config"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/logic"
	"github.com/lisuhang11/SuIM/services/msgtransfer/internal/store"
)

func main() {
	cfgPath := flag.String("c", "services/msgtransfer/etc/msgtransfer.yaml", "config file path")
	migratePath := flag.String("migrate", "migrations/msg.sql", "sql file applied on startup")
	flag.Parse()
	if err := run(*cfgPath, *migratePath); err != nil {
		slog.Error("msgtransfer stopped", "err", err)
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
		return err
	}
	if err := st.Migrate(ctx, string(script)); err != nil {
		return err
	}

	rdb, err := cache.NewRedis(cfg.Redis)
	if err != nil {
		return err
	}
	defer rdb.Close()
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}

	msgCache := cache.New(rdb, cfg.MsgCacheTTL(), st.MaxSeq)
	toMySQL := mq.NewRedisProducer(rdb, cfg.Queue.StreamPrefix, mq.TopicToMySQL, cfg.Queue.MaxLen)
	toPush := mq.NewRedisProducer(rdb, cfg.Queue.StreamPrefix, mq.TopicToPush, cfg.Queue.MaxLen)
	transfer := logic.NewTransfer(msgCache, st, toMySQL, toPush)

	hooks := webhook.NewWebhookClient(cfg.Webhooks.URL)
	defer hooks.Close()

	redisHandler := logic.NewRedisHandler(transfer, cfg.Queue.BatchSize, cfg.Queue.Workers, cfg.BatchInterval())
	redisHandler.Start(ctx)
	defer redisHandler.Close()
	mysqlHandler := logic.NewMySQLHandler(st, hooks, cfg.Webhooks.AfterMsgSaveDB)

	redisConsumer := mq.NewRedisConsumer(rdb, cfg.Queue.StreamPrefix, mq.TopicToRedis, mq.TopicToRedis)
	mysqlConsumer := mq.NewRedisConsumer(rdb, cfg.Queue.StreamPrefix, mq.TopicToMySQL, mq.TopicToMySQL)

	slog.Info("msgtransfer started", "toRedis", mq.TopicToRedis, "toMysql", mq.TopicToMySQL, "toPush", mq.TopicToPush)

	var wg sync.WaitGroup
	errCh := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		errCh <- redisConsumer.Subscribe(ctx, redisHandler.Handle)
	}()
	go func() {
		defer wg.Done()
		errCh <- mysqlConsumer.Subscribe(ctx, mysqlHandler.Handle)
	}()

	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-errCh:
		stop()
	}
	wg.Wait()
	if runErr == nil {
		select {
		case runErr = <-errCh:
		default:
		}
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		return runErr
	}
	return nil
}

func mysqlDSN(cfg config.MySQL) string {
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&charset=utf8mb4&loc=UTC&timeout=5s",
		cfg.Username, cfg.Password, cfg.Address[0], cfg.Database)
}
