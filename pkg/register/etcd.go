package register

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	etcdresolver "go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
)

var _ Registry = (*etcdRegistry)(nil)

type etcdRegistry struct {
	client    *clientv3.Client
	ownClient bool
	ttl       int64

	mu          sync.Mutex
	leaseID     clientv3.LeaseID
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse
	cancel      context.CancelFunc
	key         string
}

func NewEtcdRegistry(cfg *Config) (Registry, error) {
	ttl := cfg.Etcd.TTL
	if ttl <= 0 {
		ttl = 10
	}
	timeout := time.Duration(cfg.Etcd.DialTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Etcd.Endpoints,
		DialTimeout: timeout,
		Username:    cfg.Etcd.Username,
		Password:    cfg.Etcd.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("new etcd client: %w", err)
	}
	return &etcdRegistry{client: cli, ownClient: true, ttl: int64(ttl)}, nil
}

func (r *etcdRegistry) Register(ctx context.Context, inst *ServiceInstance) error {
	if inst == nil || inst.Name == "" || inst.ID == "" || inst.Addr == "" {
		return fmt.Errorf("invalid instance: name/id/addr required")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.deregisterLocked(); err != nil {
		return fmt.Errorf("cleanup previous registration: %w", err)
	}

	lease, err := r.client.Grant(ctx, r.ttl)
	if err != nil {
		return fmt.Errorf("grant lease: %w", err)
	}

	// etcd naming 监听的前缀不带前导斜杠，和 Dial 里的 etcd:///services/... 对齐。
	key := fmt.Sprintf("services/%s/%s", url.PathEscape(inst.Name), url.PathEscape(inst.ID))
	valBytes, _ := json.Marshal(map[string]string{"Addr": inst.Addr})
	if _, err := r.client.Put(ctx, key, string(valBytes), clientv3.WithLease(lease.ID)); err != nil {
		_, _ = r.client.Revoke(context.Background(), lease.ID)
		return fmt.Errorf("put key: %w", err)
	}

	keepCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	ch, err := r.client.KeepAlive(keepCtx, lease.ID)
	if err != nil {
		cancel()
		_, _ = r.client.Revoke(context.Background(), lease.ID)
		_, _ = r.client.Delete(context.Background(), key)
		return fmt.Errorf("start keepalive: %w", err)
	}

	r.leaseID = lease.ID
	r.key = key
	r.keepAliveCh = ch
	r.cancel = cancel

	go r.consume(keepCtx, ch)
	return nil
}

func (r *etcdRegistry) consume(ctx context.Context, ch <-chan *clientv3.LeaseKeepAliveResponse) {
	for {
		select {
		case resp, ok := <-ch:
			if !ok {
				// TODO: 租约丢失，通知上层
				return
			}
			if resp != nil && resp.TTL <= 0 {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (r *etcdRegistry) Deregister(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deregisterLocked()
}

func (r *etcdRegistry) deregisterLocked() error {
	if r.leaseID == 0 && r.key == "" {
		return nil
	}
	bgCtx, cancelBg := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelBg()

	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}

	var firstErr error
	if r.leaseID != 0 {
		if _, err := r.client.Revoke(bgCtx, r.leaseID); err != nil {
			firstErr = fmt.Errorf("revoke lease: %w", err)
		}
	}
	if r.key != "" {
		if _, err := r.client.Delete(bgCtx, r.key); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("delete key: %w", err)
		}
	}
	r.leaseID = 0
	r.key = ""
	r.keepAliveCh = nil
	return firstErr
}

func (r *etcdRegistry) List(ctx context.Context, service string) ([]ServiceInstance, error) {
	if service == "" {
		return nil, fmt.Errorf("service name is required")
	}
	prefix := fmt.Sprintf("services/%s/", url.PathEscape(service))
	resp, err := r.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", service, err)
	}
	out := make([]ServiceInstance, 0, len(resp.Kvs))
	for _, kv := range resp.Kvs {
		var meta map[string]string
		if err := json.Unmarshal(kv.Value, &meta); err != nil {
			return nil, fmt.Errorf("decode %s: %w", kv.Key, err)
		}
		addr := strings.TrimSpace(meta["Addr"])
		if addr == "" {
			continue
		}
		id := strings.TrimPrefix(string(kv.Key), prefix)
		out = append(out, ServiceInstance{ID: id, Name: service, Addr: addr})
	}
	return out, nil
}

func (r *etcdRegistry) Dial(ctx context.Context, service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	builder, err := etcdresolver.NewBuilder(r.client)
	if err != nil {
		return nil, fmt.Errorf("new etcd resolver builder: %w", err)
	}
	defaultOpts := []grpc.DialOption{
		grpc.WithResolvers(builder),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`),
		grpc.WithDefaultCallOptions(grpc.MaxCallSendMsgSize(20 * 1024 * 1024)),
	}
	target := "etcd:///services/" + url.PathEscape(service)
	return grpc.NewClient(target, append(defaultOpts, opts...)...)
}

func (r *etcdRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var err error
	if r.leaseID != 0 || r.key != "" {
		err = r.deregisterLocked()
	}
	if r.ownClient && r.client != nil {
		if cerr := r.client.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}
	return err
}
