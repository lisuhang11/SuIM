// Package main 提供 conversation gRPC 服务的入口，负责会话管理模块的初始化并启动服务。
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"

	"conversation/internal/client"
	"conversation/internal/config"
	"conversation/internal/database"
	"conversation/internal/handler"
	"conversation/internal/middleware"
	"conversation/internal/repository"
	"conversation/internal/service"

	pb "SuIM/proto/conversationpb"
	"SuIM/pkg/discovery"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	configPath := flag.String("config", "etc/conversation.yaml", "config file path")
	flag.Parse()
	cfg := config.LoadFromFile(*configPath)

	// 注册到 etcd 服务发现。
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("conversation", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(fmt.Sprintf("failed to register with etcd: %v", err))
	}
	defer registry.Deregister()

	db := database.MustOpen(cfg)

	// 组合根：将仓库注入到服务层。
	conversationRepo := repository.NewConversationRepository(db)

	userConn, err := grpc.NewClient(
		discovery.TargetURL("user"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to user service: %v", err))
	}
	defer userConn.Close()
	authenticator := client.NewUserAuthenticator(userConn)

	msgConn, err := grpc.NewClient(
		discovery.TargetURL("message"),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
	)
	if err != nil {
		panic(fmt.Sprintf("failed to connect to message service: %v", err))
	}
	defer msgConn.Close()
	msgClient := client.NewMessageClient(msgConn)
	conversationSvc := service.NewConversationService(conversationRepo, msgClient)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor(authenticator)),
	)
	pb.RegisterConversationServer(grpcSvr, handler.NewConversationHandler(conversationSvc))
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
