// Package etcd 提供基于 etcd 的远程配置监听与热重载能力。
package etcd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"apigateway/internal/config"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// OnConfigChange 是配置变更回调的函数签名。
// newCfg 是变更后的新配置；若发生错误，err 非 nil。
type OnConfigChange func(newCfg *config.GatewayConfig, err error)

// ConfigWatcher 监听 etcd 中指定 key 的配置变更并通知回调。
type ConfigWatcher struct {
	cli      *clientv3.Client
	key      string
	callback OnConfigChange

	mu       sync.RWMutex
	curCfg   *config.GatewayConfig
	cancel   context.CancelFunc
	done     chan struct{}
	isReady  bool
}

// NewConfigWatcher 创建配置监听器并自动开始 watch。
// callback 在初次加载成功及每次变更时被调用。
func NewConfigWatcher(etcdCfg *config.EtcdConfig, fallback *config.GatewayConfig, callback OnConfigChange) (*ConfigWatcher, error) {
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   etcdCfg.Endpoints,
		Username:    etcdCfg.Username,
		Password:    etcdCfg.Password,
		DialTimeout: etcdCfg.DialTimeout,
	})
	if err != nil {
		slog.Warn("[gateway] etcd connection failed, using fallback config", "error", err)
		// etcd 不可用时用回退配置，不阻塞启动。
		w := &ConfigWatcher{
			key:    etcdCfg.ConfigKey,
			curCfg: fallback,
			done:   make(chan struct{}),
		}
		callback(fallback, nil)
		return w, nil
	}

	w := &ConfigWatcher{
		cli:    cli,
		key:    etcdCfg.ConfigKey,
		curCfg: fallback,
		done:   make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), etcdCfg.DialTimeout)
	defer cancel()

	// 初次加载：拉取 etcd 远程配置，不存在时推送本地默认配置作为种子。
	resp, err := cli.Get(ctx, w.key)
	if err != nil || len(resp.Kvs) == 0 {
		slog.Info("[gateway] etcd key not found, seeding with default config", "key", w.key)
		w.seedConfig(cli, etcdCfg.ConfigKey, fallback)
		callback(fallback, nil)
	} else {
		cfg, err := config.FromJSON(resp.Kvs[0].Value)
		if err != nil {
			slog.Error("[gateway] failed to parse etcd config, using fallback", "error", err)
			cfg = fallback
		}
		w.mu.Lock()
		w.curCfg = cfg
		w.mu.Unlock()
		callback(cfg, err)
	}

	// 启动持续 watch。
	watchCtx, watchCancel := context.WithCancel(context.Background())
	w.cancel = watchCancel
	go w.watch(watchCtx)

	w.mu.Lock()
	w.isReady = true
	w.mu.Unlock()
	slog.Info("[gateway] etcd config watcher started", "key", w.key)
	return w, nil
}

// seedConfig 将默认配置写入 etcd 作为种子。
func (w *ConfigWatcher) seedConfig(cli *clientv3.Client, key string, cfg *config.GatewayConfig) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	data, _ := json.MarshalIndent(cfg, "", "  ")
	if _, err := cli.Put(ctx, key, string(data)); err != nil {
		slog.Warn("[gateway] failed to seed etcd config", "key", key, "error", err)
	}
}

// watch 持续监听 key 变更并回调。
func (w *ConfigWatcher) watch(ctx context.Context) {
	defer close(w.done)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		watchChan := w.cli.Watch(ctx, w.key)
		for wr := range watchChan {
			if wr.Err() != nil {
				slog.Error("[gateway] etcd watch error", "error", wr.Err())
				break // 退出内层 for，外层重试
			}
			for _, ev := range wr.Events {
				slog.Info("[gateway] etcd config changed", "type", ev.Type.String(), "key", string(ev.Kv.Key))
				if ev.Type == clientv3.EventTypeDelete {
					// key 被删除时回退到默认配置。
					fallback := config.Default()
					w.mu.Lock()
					w.curCfg = fallback
					w.mu.Unlock()
					w.callback(fallback, nil)
					continue
				}
				cfg, err := config.FromJSON(ev.Kv.Value)
				if err != nil {
					slog.Error("[gateway] failed to parse updated config", "error", err)
					w.callback(w.Current(), fmt.Errorf("parse config: %w", err))
					continue
				}
				w.mu.Lock()
				w.curCfg = cfg
				w.mu.Unlock()
				w.callback(cfg, nil)
			}
		}
		// watch channel 意外断开，等待后重试。
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
}

// Current 返回当前生效的配置（线程安全）。
func (w *ConfigWatcher) Current() *config.GatewayConfig {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.curCfg
}

// Shutdown 优雅关闭 watcher，释放 etcd 客户端连接。
func (w *ConfigWatcher) Shutdown() {
	if w.cancel != nil {
		w.cancel()
	}
	select {
	case <-w.done:
	case <-time.After(3 * time.Second):
	}
	if w.cli != nil {
		w.cli.Close()
	}
}
