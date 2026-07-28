// Package main 是 push gRPC 服务的入口。
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"push/internal/config"
	"push/internal/database"
	"push/internal/handler"
	"push/internal/middleware"
	"push/internal/repository"
	"push/internal/service"

	pb "SuIM/proto/pushpb"
	"SuIM/pkg/discovery"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "etc/push.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	// 注册到 etcd 服务发现。
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("push", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(fmt.Sprintf("failed to register with etcd: %v", err))
	}
	defer registry.Deregister()

	db := database.MustOpen(context.Background(), cfg)

	// 依赖注入：将 repository 注入到 service。
	pushRepo := repository.NewPushRepository(db)
	pushSvc := service.NewPushService(pushRepo)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor()),
	)
	pb.RegisterPushMsgServiceServer(grpcSvr, handler.NewPushHandler(pushSvc))
	reflection.Register(grpcSvr) // 启用 grpcurl 调试支持

	lis, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to listen on %s: %v", cfg.ServerAddr, err))
	}

	slog.Info("push gRPC server starting", "addr", cfg.ServerAddr)
	if err := grpcSvr.Serve(lis); err != nil {
		panic(fmt.Sprintf("push gRPC server exited: %v", err))
	}
}
