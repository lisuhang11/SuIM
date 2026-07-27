// Package main 提供 conversation gRPC 服务的入口，负责会话管理模块的初始化并启动服务。
package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"conversation/internal/config"
	"conversation/internal/database"
	"conversation/internal/handler"
	"conversation/internal/middleware"
	"conversation/internal/repository"
	"conversation/internal/service"

	pb "SuIM/proto/conversationpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 初始化结构化 JSON 日志。
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	db := database.MustOpen(cfg)

	// 组合根：将仓库注入到服务层。
	conversationRepo := repository.NewConversationRepository(db)
	conversationSvc := service.NewConversationService(conversationRepo)

	grpcSvr := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.UnaryServerInterceptor()),
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
