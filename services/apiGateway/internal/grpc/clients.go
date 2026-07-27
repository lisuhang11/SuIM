// Package grpc 管理到后端微服务的 gRPC 客户端连接池，支持动态热切换。
package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	"apigateway/internal/config"

	pbUser "SuIM/proto/userpb"
	pbRel "SuIM/proto/relationpb"
	pbGroup "SuIM/proto/grouppb"
	pbConv "SuIM/proto/conversationpb"
	pbMsg "SuIM/proto/messagepb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Clients 持有所有后端 gRPC 客户端，对外只暴露接口。
type Clients struct {
	User         pbUser.UserServiceClient
	Relation     pbRel.RelationServiceClient
	Group        pbGroup.GroupServiceClient
	Conversation pbConv.ConversationClient
	Message      pbMsg.MessageClient

	mu     sync.RWMutex
	conns  map[string]*grpc.ClientConn
}

// NewClients 根据配置创建所有 gRPC 客户端连接。
func NewClients(cfg *config.GatewayConfig) (*Clients, error) {
	c := &Clients{
		conns: make(map[string]*grpc.ClientConn),
	}
	if err := c.connect(cfg); err != nil {
		return nil, err
	}
	return c, nil
}

// connect 根据 backend 列表建立连接并创建客户端。
func (c *Clients) connect(cfg *config.GatewayConfig) error {
	svcMap := map[string]*grpc.ClientConn{}

	for _, backend := range cfg.Backends {
		if backend.Addr == "" {
			slog.Warn("[gateway] skipping backend with empty addr", "name", backend.Name)
			continue
		}
		conn, err := grpc.NewClient(backend.Addr,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
		)
		if err != nil {
			return fmt.Errorf("dial %s(%s): %w", backend.Name, backend.Addr, err)
		}
		svcMap[backend.Name] = conn
		slog.Info("[gateway] gRPC client connected", "service", backend.Name, "addr", backend.Addr)
	}

	// 逐个创建客户端，缺失的服务用 nil（路由守卫会报 503）。
	c.mu.Lock()
	defer c.mu.Unlock()

	// user
	if conn, ok := svcMap["user"]; ok {
		c.User = pbUser.NewUserServiceClient(conn)
		c.conns["user"] = conn
	}
	// relation
	if conn, ok := svcMap["relation"]; ok {
		c.Relation = pbRel.NewRelationServiceClient(conn)
		c.conns["relation"] = conn
	}
	// group
	if conn, ok := svcMap["group"]; ok {
		c.Group = pbGroup.NewGroupServiceClient(conn)
		c.conns["group"] = conn
	}
	// conversation
	if conn, ok := svcMap["conversation"]; ok {
		c.Conversation = pbConv.NewConversationClient(conn)
		c.conns["conversation"] = conn
	}
	// message
	if conn, ok := svcMap["message"]; ok {
		c.Message = pbMsg.NewMessageClient(conn)
		c.conns["message"] = conn
	}

	return nil
}

// Reload 根据新配置热切换后端连接（先建新连接，再断开旧连接）。
func (c *Clients) Reload(ctx context.Context, newCfg *config.GatewayConfig) error {
	newClients := &Clients{
		conns: make(map[string]*grpc.ClientConn),
	}
	if err := newClients.connect(newCfg); err != nil {
		return err
	}

	// 原子替换。
	c.mu.Lock()
	oldConns := c.conns
	c.User = newClients.User
	c.Relation = newClients.Relation
	c.Group = newClients.Group
	c.Conversation = newClients.Conversation
	c.Message = newClients.Message
	c.conns = newClients.conns
	c.mu.Unlock()

	// 异步关闭旧连接，给 in-flight 请求缓冲时间。
	go func() {
		for name, conn := range oldConns {
			slog.Info("[gateway] closing old gRPC connection", "service", name)
			conn.Close()
		}
	}()

	slog.Info("[gateway] gRPC clients reloaded")
	return nil
}

// Close 关闭所有 gRPC 连接。
func (c *Clients) Close() {
	c.mu.RLock()
	defer c.mu.RUnlock()
	for name, conn := range c.conns {
		slog.Info("[gateway] closing gRPC connection", "service", name)
		conn.Close()
	}
}
