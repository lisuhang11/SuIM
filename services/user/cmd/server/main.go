// Package main is the entry point for the user gRPC service.
package main

import (
	"fmt"
	"log/slog"
	"net"
	"os"

	"user/internal/config"
	"user/internal/database"
	"user/internal/handler"
	"user/internal/repository"
	"user/internal/service"

	pb "user/proto/userpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// Structured JSON logger.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	db := database.MustOpen(cfg)

	userRepo := repository.NewUserRepository(db)
	tokenRepo := repository.NewAuthTokenRepository(db)
	userSvc := service.NewUserService(userRepo, tokenRepo)

	grpcSvr := grpc.NewServer()
	pb.RegisterUserServiceServer(grpcSvr, handler.NewUserHandler(userSvc))
	reflection.Register(grpcSvr) // enable grpcurl / debugging

	lis, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		panic(fmt.Sprintf("failed to listen on %s: %v", cfg.ServerAddr, err))
	}

	slog.Info("gRPC server starting", "addr", cfg.ServerAddr)
	if err := grpcSvr.Serve(lis); err != nil {
		panic(fmt.Sprintf("gRPC server exited: %v", err))
	}
}
