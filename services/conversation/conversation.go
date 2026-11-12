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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/lisuhang11/SuIM/pkg/config"
	"github.com/lisuhang11/SuIM/pkg/mw"
	"github.com/lisuhang11/SuIM/pkg/register"
	"github.com/lisuhang11/SuIM/pkg/rpcctx"
	conversationpb "github.com/lisuhang11/SuIM/proto/conversationpb"
	msgpb "github.com/lisuhang11/SuIM/proto/msg"
	svcconfig "github.com/lisuhang11/SuIM/services/conversation/internal/config"
	"github.com/lisuhang11/SuIM/services/conversation/internal/logic"
	"github.com/lisuhang11/SuIM/services/conversation/internal/store"
)

func main() {
	cfgPath := flag.String("c", "services/conversation/etc/conversation.yaml", "config file path")
	migratePath := flag.String("migrate", "migrations/conversation.sql", "sql file applied on startup")
	flag.Parse()

	if err := run(*cfgPath, *migratePath); err != nil {
		slog.Error("conversation service stopped", "err", err)
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

	reg, err := register.New(&cfg.Registry)
	if err != nil {
		return err
	}
	defer reg.Close()

	msgConn, err := reg.Dial(ctx, "msg",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(rpcctx.ClientInterceptor),
	)
	if err != nil {
		return fmt.Errorf("dial msg service: %w", err)
	}
	defer msgConn.Close()

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.ListenIP, cfg.Port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	srv := grpc.NewServer(grpc.ChainUnaryInterceptor(mw.UnaryRecover, rpcctx.ServerInterceptor))
	messages := msgClient{msgpb.NewMsgServiceClient(msgConn)}
	conversation := logic.New(st, messages)
	conversation.UseSeq(messages)
	conversation.UseHistory(messages)
	conversationpb.RegisterConversationServiceServer(srv, conversation)

	id, err := newID()
	if err != nil {
		return err
	}
	addr := fmt.Sprintf("%s:%d", cfg.RegisterIP, cfg.Port)
	regCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	err = reg.Register(regCtx, &register.ServiceInstance{
		ID:   id,
		Name: "conversation",
		Addr: addr,
	})
	cancel()
	if err != nil {
		_ = lis.Close()
		return fmt.Errorf("register conversation service: %w", err)
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("conversation service listening", "addr", lis.Addr().String(), "register", addr)
		errCh <- srv.Serve(lis)
	}()

	select {
	case <-ctx.Done():
		offCtx, offCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer offCancel()
		_ = reg.Deregister(offCtx)
		srv.GracefulStop()
		return nil
	case err := <-errCh:
		offCtx, offCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer offCancel()
		_ = reg.Deregister(offCtx)
		return err
	}
}

type msgClient struct {
	client msgpb.MsgServiceClient
}

func (c msgClient) SendNotification(ctx context.Context, msg *msgpb.MsgData) error {
	_, err := c.client.SendMsg(ctx, &msgpb.SendMsgReq{MsgData: msg})
	return err
}

func (c msgClient) SetUserMaxSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.client.SetUserConversationMaxSeq(ctx, &msgpb.SetUserConversationMaxSeqReq{
		ConversationId: conversationID,
		UserIds:        userIDs,
		MaxSeq:         seq,
	})
	return err
}

func (c msgClient) SetUserMinSeq(ctx context.Context, conversationID string, userIDs []string, seq int64) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.client.SetUserConversationMinSeq(ctx, &msgpb.SetUserConversationMinSeqReq{
		ConversationId: conversationID,
		UserIds:        userIDs,
		MinSeq:         seq,
	})
	return err
}

func (c msgClient) LastSeqByTime(ctx context.Context, conversationID string, timeMs int64) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	resp, err := c.client.GetLastMessageSeqByTime(ctx, &msgpb.GetLastMessageSeqByTimeReq{
		ConversationId: conversationID,
		Time:           timeMs,
	})
	if err != nil {
		return 0, err
	}
	return resp.GetSeq(), nil
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
