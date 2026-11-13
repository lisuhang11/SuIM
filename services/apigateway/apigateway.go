package main

import (
	"context"
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
	"github.com/redis/go-redis/v9"
	"github.com/lisuhang11/SuIM/pkg/mw"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	"github.com/lisuhang11/SuIM/proto/group"
	"github.com/lisuhang11/SuIM/proto/relation"
	thirdpb "github.com/lisuhang11/SuIM/proto/third"
	"github.com/lisuhang11/SuIM/proto/user"
	"github.com/lisuhang11/SuIM/services/apigateway/internal/api"
	svcconfig "github.com/lisuhang11/SuIM/services/apigateway/internal/config"
)

func main() {
	cfgPath := flag.String("c", "services/apigateway/etc/apigateway.yaml", "config file path")
	flag.Parse()

	if err := run(*cfgPath); err != nil {
		slog.Error("api gateway stopped", "err", err)
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
	conversationConn, err := reg.Dial(ctx, "conversation",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial conversation service: %w", err)
	}
	defer conversationConn.Close()
	thirdConn, err := reg.Dial(ctx, "third",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial third service: %w", err)
	}
	defer thirdConn.Close()

	apiSrv := api.New(
		userpb.NewUserServiceClient(userConn),
		relationpb.NewRelationServiceClient(relationConn),
		grouppb.NewGroupServiceClient(groupConn),
		conversationpb.NewConversationServiceClient(conversationConn),
		thirdpb.NewThirdServiceClient(thirdConn),
	)
	apiSrv.UsePreviewTools(cfg.VoicePreview.Python, cfg.VoicePreview.Script)
	if len(cfg.Redis.Address) > 0 {
		rdb := redis.NewClient(&redis.Options{
			Addr:       cfg.Redis.Address[0],
			Username:   cfg.Redis.Username,
			Password:   cfg.Redis.Password,
			DB:         cfg.Redis.DB,
			MaxRetries: cfg.Redis.MaxRetry,
			PoolSize:   cfg.Redis.PoolSize,
		})
		defer rdb.Close()
		apiSrv.UseRedis(rdb)
	}

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	srv := &http.Server{
		Handler:           mw.GzipHandler(cfg.CompressionLevel, apiSrv.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("api gateway listening", "addr", lis.Addr().String())
		errCh <- srv.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		offCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(offCtx)
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
