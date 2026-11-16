# SuIM Kubernetes Deployment

裸 YAML 部署，用 `kubectl apply -k` 不需要 Helm/Kustomize；`deploy.sh` 按顺序应用全部资源。

## 前置条件

1. Kubernetes 集群 + `ingress-nginx` controller（配置 `ingressClassName: nginx`）。
2. MySQL / Redis / MinIO 可从 Pod 访问。`infra.yml` 默认用 Endpoints 指向宿主机上的 docker-compose 容器，把两处 `EXTERNAL_IP` 改成节点可达 IP；若把这些组件也部署进集群，删掉 `infra.yml`，改用同名 Service（端口 3306/6379/9000）。MySQL 需要预先建好 `suim_user`、`suim_group`、`suim_relation`、`suim_conversation`、`suim_msg`、`suim_crontask` 六库。

## 部署步骤

```bash
# 1. 构建并推送镜像（REGISTRY/TAG 按需修改）
REGISTRY=your-registry TAG=v1 ./scripts/build-images.sh

# 2. 修改镜像地址：把 apps/*.yml 里的 suim/<svc>:dev 替换成实际镜像
# 3. 修改 secret.yml 中所有 CHANGE_ME
# 4. 修改 infra.yml 中的 EXTERNAL_IP
# 5. 部署
./deployments/deploy.sh
```

## 配置分层

- `configmap.yml`：12 个服务的 yaml，键名即文件名。`CONFIG_PATH=/config` 让 `config.Load` 从挂载目录读取原路径的同名文件。
- `secret.yml`：连接密码与 token，通过 `SUIMENV_*` 环境变量注入。`pkg/config/env.go` 里维护了可覆盖字段白名单，例如 `SUIMENV_MYSQL_PASSWORD` → `mysql.password`。
- `registry.type: kubernetes` + `k8s.namespace/port` 启用 `pkg/register/k8s.go` 的 DNS 发现。`Service` 名必须与 `reg.Dial` 的名字一致（`user`、`group`、`push`…），且为 headless（`clusterIP: None`），端口必须等于 `k8s.port`。

## 访问

- HTTP API：`api.suim.local`（改为你的 ingress 地址或 hosts 记录）→ apigateway
- WebSocket 与 `/msg/*` HTTP 端点：`ws.suim.local` → msggateway

网关把 WS 和 HTTP 端点都挂在 `/`，所以必须用两个 host 区分，不能只靠 path。临时验证可用：

```bash
kubectl -n suim port-forward svc/apigateway 8089:8089
kubectl -n suim port-forward svc/msggateway-ws 10001:10001
```

## 与本地启动的区别

- 本地：`scripts/start.sh` + `services/<svc>/etc/<svc>.yaml`（127.0.0.1 + etcd 发现）
- 集群：ConfigMap/Secret + k8s DNS 发现；不需要 RBAC（DNS 解析由 CoreDNS 完成，`Register` 是 no-op，靠 readinessProbe 控制就绪 IP 是否进入 DNS）
- `msgtransfer`、`crontask` 不监听端口，不需要 headless Service；crontask 单副本避免定时任务重复

## Prometheus 监控

每个服务暴露独立的 `/metrics` 端点，端口在 `configmap.yml` 的 `metricsPort` 配置（16080–16091）。Pod 上带 `prometheus.io/scrape=true`、`prometheus.io/path=/metrics`、`prometheus.io/port=<端口>` 注解，Prometheus 可用 Kubernetes pod discovery 自动抓取。通用指标包括 `rpc_count{name,path,code}`、`http_count`、`process_*`、`go_*`；业务指标按服务分发（如 push 的 `msg_offline_push_failed_total`、msgtransfer 的 `msg_insert_mysql_*`、msggateway 的 `online_user_num`），与 OpenIM 的指标命名保持一致。

## 限流

`configmap.yml` 每个服务都有 `rateLimiter`，默认关闭：

```yaml
rateLimiter:
  enable: false
  window: 20s
  bucket: 500
  cpuThreshold: 850
```

开启后：

- gRPC 服务在业务 handler 前执行 BBR unary interceptor，拒绝时返回 gRPC `ResourceExhausted`。
- `apigateway` 与 `msggateway` 的 HTTP 入口执行同样的 limiter，拒绝时返回 HTTP 429，并带上 `X-BBR-*`、`X-RateLimit-*`、`Retry-After` 响应头。
- 限流是**进程内**的；多副本时总容量会按副本数放大，需要全局限流时应另接 Redis/网关层。
- `msgtransfer`、`crontask` 没有入站 RPC/HTTP，配置暂不生效。
- 拒绝次数通过 `rate_limit_rejected_total{service,kind}` 暴露到各自 `/metrics`。

`bucket` 是滑动窗口的桶数，不是固定 QPS。默认 `20s/500` 表示每桶 40ms；`cpuThreshold: 850` 表示 CPU 达到 85% 后才按估算容量主动丢弃请求。

## 已知限制

- 镜像 tag 为 `:dev`，正式环境建议固定版本并开启镜像拉取鉴权。
- 未配置 HPA / PDB / NetworkPolicy；如需可在此基础上追加。
- `push` 默认 `enable: dummy`，需要离线推送时在 ConfigMap 的 `push.yaml` 中改 `enable` 并填对应 Secret。
- `rtc` 的 LiveKit agents 默认关闭，脚本路径不在镜像内。
