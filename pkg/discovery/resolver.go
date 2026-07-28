package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/resolver"
)

const scheme = "etcd"

func init() {
	resolver.Register(&etcdResolverBuilder{})
}

// etcdResolverBuilder 实现 gRPC resolver.Builder 接口。
type etcdResolverBuilder struct{}

func (b *etcdResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// target.URL.Host 为服务名，如 "etcd:///user"
	serviceName := strings.TrimPrefix(target.URL.Path, "/")
	if serviceName == "" {
		return nil, fmt.Errorf("etcd resolver: service name is empty in target %q", target.URL)
	}

	endpoints := getEndpoints()
	if len(endpoints) == 0 {
		return nil, fmt.Errorf("etcd resolver: etcd endpoints not configured, call discovery.SetEndpoints first")
	}

	r := &etcdResolver{
		cc:          cc,
		serviceName: serviceName,
		endpoints:   endpoints,
		closeCh:     make(chan struct{}),
	}

	go r.watch()
	return r, nil
}

func (b *etcdResolverBuilder) Scheme() string {
	return scheme
}

type etcdResolver struct {
	cc          resolver.ClientConn
	serviceName string
	endpoints   []string
	closeCh     chan struct{}
	mu          sync.Mutex
	client      *clientv3.Client
}

func (r *etcdResolver) ResolveNow(opts resolver.ResolveNowOptions) {
	// etcd watch 已持续更新，无需额外触发。
}

func (r *etcdResolver) Close() {
	close(r.closeCh)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.client != nil {
		r.client.Close()
	}
}

func (r *etcdResolver) watch() {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   r.endpoints,
		DialTimeout: defaultDialTimeout,
	})
	if err != nil {
		slog.Error("[discovery] etcd resolver connect failed",
			"service", r.serviceName, "error", err)
		return
	}

	r.mu.Lock()
	r.client = cli
	r.mu.Unlock()

	prefix := fmt.Sprintf("/suim/services/%s/", r.serviceName)

	// 先获取当前所有实例。
	if err := r.updateAddrs(cli, prefix); err != nil {
		slog.Error("[discovery] initial address fetch failed",
			"service", r.serviceName, "error", err)
	}

	// 启动 watch。
	watchCh := cli.Watch(context.Background(), prefix, clientv3.WithPrefix())
	isFirst := true

	for {
		select {
		case wresp, ok := <-watchCh:
			if !ok {
				slog.Warn("[discovery] watch channel closed, reconnecting",
					"service", r.serviceName)
				// 重连。
				time.Sleep(2 * time.Second)
				newCli, err := clientv3.New(clientv3.Config{
					Endpoints:   r.endpoints,
					DialTimeout: defaultDialTimeout,
				})
				if err != nil {
					continue
				}
				r.mu.Lock()
				if r.client != nil {
					r.client.Close()
				}
				r.client = newCli
				r.mu.Unlock()
				cli = newCli

				if err := r.updateAddrs(cli, prefix); err != nil {
					slog.Error("[discovery] reconnect fetch failed",
						"service", r.serviceName, "error", err)
				}
				watchCh = cli.Watch(context.Background(), prefix, clientv3.WithPrefix())
				continue
			}

			if wresp.Err() != nil {
				slog.Error("[discovery] watch error",
					"service", r.serviceName, "error", wresp.Err())
				continue
			}

			if isFirst {
				// 首次 watch 事件不更新（已在 updateAddrs 中获取）。
				isFirst = false
				continue
			}

			if err := r.updateAddrs(cli, prefix); err != nil {
				slog.Error("[discovery] update addrs failed",
					"service", r.serviceName, "error", err)
			}

		case <-r.closeCh:
			return
		}
	}
}

func (r *etcdResolver) updateAddrs(cli *clientv3.Client, prefix string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	resp, err := cli.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return fmt.Errorf("get prefix %s: %w", prefix, err)
	}

	addrs := make([]resolver.Address, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var info ServiceInfo
		if err := json.Unmarshal(kv.Value, &info); err != nil {
			slog.Warn("[discovery] unmarshal service info failed",
				"key", string(kv.Key), "error", err)
			continue
		}
		if info.Addr != "" {
			addrs = append(addrs, resolver.Address{Addr: info.Addr})
		}
	}

	slog.Info("[discovery] resolved addresses",
		"service", r.serviceName,
		"count", len(addrs),
	)

	r.cc.UpdateState(resolver.State{Addresses: addrs})
	return nil
}

// TargetURL 构造 etcd 解析器的目标 URL。
// serviceName 为服务名，如 "user"、"group"。
func TargetURL(serviceName string) string {
	return fmt.Sprintf("%s:///%s", scheme, serviceName)
}
