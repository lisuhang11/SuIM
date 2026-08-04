package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"SuIM/pkg/discovery"
	pb "SuIM/proto/filepb"
	"fileservice/internal/config"
	"fileservice/internal/database"
	"fileservice/internal/handler"
	"fileservice/internal/middleware"
	"fileservice/internal/repository"
	"fileservice/internal/service"
	"fileservice/internal/storage"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	path := flag.String("config", "etc/file.yaml", "config file")
	flag.Parse()
	cfg := config.LoadFromFile(*path)
	ctx := context.Background()
	discovery.SetEndpoints(cfg.EtcdEndpoints)
	registry, err := discovery.NewRegistry("file", cfg.ServiceAddr, cfg.EtcdEndpoints)
	if err != nil {
		panic(err)
	}
	defer registry.Deregister()
	db := database.MustOpen(cfg)
	store, err := storage.New(ctx, cfg)
	if err != nil {
		panic(fmt.Sprintf("minio: %v", err))
	}
	svc := service.New(repository.New(db), store, cfg)
	go cleanup(ctx, svc, cfg.CleanupInterval)
	server := grpc.NewServer(grpc.UnaryInterceptor(middleware.Unary(cfg.JWTSecret)))
	pb.RegisterFileServiceServer(server, handler.New(svc))
	reflection.Register(server)
	lis, err := net.Listen("tcp", cfg.ServerAddr)
	if err != nil {
		panic(err)
	}
	slog.Info("file service starting", "addr", cfg.ServerAddr, "retention", cfg.FileRetention)
	if err := server.Serve(lis); err != nil {
		panic(err)
	}
}
func cleanup(ctx context.Context, svc *service.Service, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			total := 0
			for {
				n, err := svc.Cleanup(ctx)
				if err != nil {
					slog.Error("file cleanup failed", "error", err)
					break
				}
				total += n
				if n < 100 {
					break
				}
			}
			if total > 0 {
				slog.Info("expired files deleted", "count", total)
			}
		}
	}
}
