// Package discovery 提供基于 etcd 的服务注册与发现。
package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdEndpoints 是全局 etcd 端点配置，在服务启动时由 main 函数通过 SetEndpoints 初始化。
var (
	etcdEndpoints   []string
	etcdEndpointsMu sync.RWMutex
)

// SetEndpoints 设置全局 etcd 端点列表，供 registry 和 resolver 共用。
func SetEndpoints(endpoints []string) {
	etcdEndpointsMu.Lock()
	defer etcdEndpointsMu.Unlock()
	etcdEndpoints = endpoints
}

// getEndpoints 返回已配置的 etcd 端点。
func getEndpoints() []string {
	etcdEndpointsMu.RLock()
	defer etcdEndpointsMu.RUnlock()
	return etcdEndpoints
}

const (
	defaultLeaseTTL    = 15 // 租约 TTL（秒）
	defaultDialTimeout = 5 * time.Second
)

// ServiceInfo 注册到 etcd 的服务信息。
type ServiceInfo struct {
	Addr string `json:"addr"` // "host:port"
}

// Registry 管理 etcd 服务注册。
type Registry struct {
	client    *clientv3.Client
	leaseID   clientv3.LeaseID
	key       string
	closeCh   chan struct{}
	serviceName string
}

// NewRegistry 创建服务注册器并建立 etcd 连接。
// serviceName: 服务名，如 "user"、"group"。
// serviceAddr: 服务地址，如 "user:8080"。
// endpoints: etcd 端点列表，如 ["127.0.0.1:2379"]。
func NewRegistry(serviceName, serviceAddr string, endpoints []string) (*Registry, error) {
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("etcd endpoints not configured")
	}

	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: defaultDialTimeout,
	})
	if err != nil {
		return nil, fmt.Errorf("connect etcd: %w", err)
	}

	key := fmt.Sprintf("/suim/services/%s/%s", serviceName, generateInstanceID())

	r := &Registry{
		client:      cli,
		key:         key,
		closeCh:     make(chan struct{}),
		serviceName: serviceName,
	}

	// 注册并启动 keepalive。
	if err := r.register(serviceAddr); err != nil {
		cli.Close()
		return nil, fmt.Errorf("register service: %w", err)
	}

	slog.Info("[discovery] service registered",
		"service", serviceName,
		"key", key,
		"addr", serviceAddr,
	)

	go r.keepAlive()

	return r, nil
}

// register 向 etcd 写入服务信息并创建租约。
func (r *Registry) register(addr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()

	grant, err := r.client.Grant(ctx, defaultLeaseTTL)
	if err != nil {
		return fmt.Errorf("grant lease: %w", err)
	}

	r.leaseID = grant.ID

	info := ServiceInfo{Addr: addr}
	val, err := json.Marshal(info)
	if err != nil {
		return fmt.Errorf("marshal service info: %w", err)
	}

	_, err = r.client.Put(ctx, r.key, string(val), clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("put key: %w", err)
	}

	return nil
}

// keepAlive 维持租约续期。
func (r *Registry) keepAlive() {
	ch, err := r.client.KeepAlive(context.Background(), r.leaseID)
	if err != nil {
		slog.Error("[discovery] keepalive failed", "service", r.serviceName, "error", err)
		return
	}

	for {
		select {
		case ka, ok := <-ch:
			if !ok {
				slog.Warn("[discovery] keepalive channel closed, re-registering",
					"service", r.serviceName)
				// 尝试重新注册。
				time.Sleep(time.Second)
				continue
			}
			if ka != nil {
				slog.Debug("[discovery] lease renewed",
					"service", r.serviceName,
					"ttl", ka.TTL,
				)
			}
		case <-r.closeCh:
			return
		}
	}
}

// Deregister 从 etcd 注销服务。
func (r *Registry) Deregister() {
	close(r.closeCh)
	ctx, cancel := context.WithTimeout(context.Background(), defaultDialTimeout)
	defer cancel()

	if _, err := r.client.Revoke(ctx, r.leaseID); err != nil {
		slog.Error("[discovery] revoke lease failed", "service", r.serviceName, "error", err)
	}

	if err := r.client.Close(); err != nil {
		slog.Error("[discovery] close etcd client failed", "service", r.serviceName, "error", err)
	}

	slog.Info("[discovery] service deregistered", "service", r.serviceName)
}

// generateInstanceID 生成实例唯一标识。
func generateInstanceID() string {
	hostname, _ := os.Hostname()
	pid := os.Getpid()
	return fmt.Sprintf("%s-%d-%d", hostname, pid, time.Now().UnixNano())
}
