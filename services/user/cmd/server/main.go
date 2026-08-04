// Package main 提供 user gRPC 服务的入口，负责初始化并启动服务。
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"user/internal/cache"
	"user/internal/client"
	"user/internal/config"
	"user/internal/database"
	"user/internal/handler"
	"user/internal/repository"
	"user/internal/service"

	"SuIM/pkg/discovery"
	filePB "SuIM/proto/filepb"
	pb "SuIM/proto/userpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "etc/user.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	// 注册到 etcd 服务发现。
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("user", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(fmt.Sprintf("failed to register with etcd: %v", err))
	}
	defer registry.Deregister()

	db := database.MustOpen(cfg)

	rdb, err := cache.NewRedisClient(cache.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	if err != nil {
		slog.Warn("redis unavailable, user info cache disabled", "error", err)
		rdb = nil
	}
	if rdb != nil {
		defer rdb.Close()
	}
	userCache := cache.NewUserInfoCache(rdb, cfg.UserInfoCacheTTL)

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewAuthTokenRepository(db)
	relationConn, err := grpc.NewClient(discovery.TargetURL("relation"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		panic(fmt.Sprintf("failed to connect to relation service: %v", err))
	}
	defer relationConn.Close()
	relationNotifier := client.NewRelationNotifier(relationConn)
	userSvc := service.NewUserService(userRepo, tokenRepo, userCache, cfg, relationNotifier)
	fileConn, err := grpc.NewClient(discovery.TargetURL("file"), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`))
	if err != nil {
		panic(fmt.Sprintf("failed to connect to file service: %v", err))
	}
	defer fileConn.Close()
	fileClient := filePB.NewFileServiceClient(fileConn)

	grpcSvr := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcSvr, handler.NewUserHandler(userSvc, fileClient))
	reflection.Register(grpcSvr) // 启用 grpcurl 等调试工具

	lis, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to listen on %s: %v", cfg.ServerAddr, err))
	}

	slog.Info("gRPC server starting", "addr", cfg.ServerAddr)
	if err := grpcSvr.Serve(lis); err != nil {
		panic(fmt.Sprintf("gRPC server exited: %v", err))
	}
}
