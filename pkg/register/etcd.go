package register

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"sync"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	etcdresolver "go.etcd.io/etcd/client/v3/naming/resolver"
	"google.golang.org/grpc"
)

const keepAliveRetryDelay = time.Second

var errRegisterStopped = errors.New("registration stopped")

var _ Registry = (*etcdRegistry)(nil)

type etcdRegistry struct {
	client    *clientv3.Client
	ownClient bool
	ttl       int64

	mu      sync.Mutex
	gen     uint64
	leaseID clientv3.LeaseID
	cancel  context.CancelFunc
	key     string
	value   string
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

	// etcd naming 监听的前缀不带前导斜杠，和 Dial 里的 etcd:///services/... 对齐。
	key := fmt.Sprintf("services/%s/%s", url.PathEscape(inst.Name), url.PathEscape(inst.ID))
	valBytes, _ := json.Marshal(map[string]string{"Addr": inst.Addr})
	value := string(valBytes)
	leaseID, err := r.putLeaseLocked(ctx, key, value)
	if err != nil {
		return err
	}

	keepCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	ch, err := r.client.KeepAlive(keepCtx, leaseID)
	if err != nil {
		cancel()
		_, _ = r.client.Revoke(context.Background(), leaseID)
		_, _ = r.client.Delete(context.Background(), key)
		return fmt.Errorf("start keepalive: %w", err)
	}

	r.leaseID = leaseID
	r.key = key
	r.value = value
	r.cancel = cancel
	go r.keepAlive(keepCtx, r.gen, ch)
	return nil
}

// putLeaseLocked grants a lease and writes key. The caller holds r.mu.
func (r *etcdRegistry) putLeaseLocked(ctx context.Context, key, value string) (clientv3.LeaseID, error) {
	lease, err := r.client.Grant(ctx, r.ttl)
	if err != nil {
		return 0, fmt.Errorf("grant lease: %w", err)
	}
	if _, err := r.client.Put(ctx, key, value, clientv3.WithLease(lease.ID)); err != nil {
		_, _ = r.client.Revoke(context.Background(), lease.ID)
		return 0, fmt.Errorf("put key: %w", err)
	}
	return lease.ID, nil
}

// keepAlive renews the lease. etcd closes the keepalive channel when that lease
// can no longer be renewed, so the same key is written again with a new lease.
func (r *etcdRegistry) keepAlive(ctx context.Context, gen uint64, ch <-chan *clientv3.LeaseKeepAliveResponse) {
	for {
		if ctx.Err() != nil {
			return
		}
		if ch == nil {
			r.mu.Lock()
			if r.gen != gen || r.client == nil || r.leaseID == 0 {
				r.mu.Unlock()
				return
			}
			leaseID := r.leaseID
			client := r.client
			r.mu.Unlock()
			var err error
			ch, err = client.KeepAlive(ctx, leaseID)
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				slog.Warn("etcd keepalive stopped", "err", err)
				if err := r.reRegister(ctx, gen); err != nil {
					if errors.Is(err, errRegisterStopped) || ctx.Err() != nil || !sleepCtx(ctx, keepAliveRetryDelay) {
						return
					}
				}
				ch = nil
				continue
			}
		}
		select {
		case <-ctx.Done():
			return
		case resp, ok := <-ch:
			if ok && resp != nil && resp.TTL > 0 {
				continue
			}
			if ctx.Err() != nil {
				return
			}
			slog.Warn("etcd keepalive stopped", "err", errors.New("lease lost"))
			if err := r.reRegister(ctx, gen); err != nil {
				if errors.Is(err, errRegisterStopped) || ctx.Err() != nil || !sleepCtx(ctx, keepAliveRetryDelay) {
					return
				}
			}
			ch = nil
		}
	}
}

// reRegister writes the current key onto a new lease. errRegisterStopped means
// the registration was removed or replaced, and the caller must not try again.
func (r *etcdRegistry) reRegister(ctx context.Context, gen uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ctx.Err() != nil || r.gen != gen || r.client == nil || r.key == "" || r.value == "" {
		return errRegisterStopped
	}
	retryCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	leaseID, err := r.putLeaseLocked(retryCtx, r.key, r.value)
	if err != nil {
		slog.Warn("etcd re-register failed", "key", r.key, "err", err)
		return err
	}
	// The key now belongs to the new lease. The old lease has no key and expires on its own.
	r.leaseID = leaseID
	slog.Info("etcd re-registered", "key", r.key)
	return nil
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func (r *etcdRegistry) Deregister(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.deregisterLocked()
}

func (r *etcdRegistry) deregisterLocked() error {
	r.gen++
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	if r.leaseID == 0 && r.key == "" {
		r.value = ""
		return nil
	}
	bgCtx, cancelBg := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelBg()

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
	r.value = ""
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
