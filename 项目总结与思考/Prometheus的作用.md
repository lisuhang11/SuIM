# Prometheus 的作用、原理与在 SuIM 中的实践

> 一篇从"为什么需要监控"讲起，覆盖数据模型、Pull 架构、PromQL，再到 SuIM 中如何落地实现、如何配告警的完整笔记。文中所有指标名、代码路径、端口都与仓库里的实现一一对应，可以直接当对照手册用。

## 一、Prometheus 到底解决什么问题

一个分布式 IM 系统跑起来之后，最怕的不是"出错"，而是**"用户说卡，但你不知道卡在哪"**。 Prometheus 解决的就是这一类问题：把系统内部的状态变成可查询、可告警、可画图的时间序列数据。

具体说，监控有四个层次的作用：

| 层次 | 回答的问题 | 典型用法 |
| --- | --- | --- |
| 告警 | "现在是不是出事了？" | 错误率突增、堆积持续扩大、服务不可用 |
| 定位 | "为什么出事？" | 哪个接口、哪个方法、哪台机器、哪个返回码 |
| 容量 | "还能撑多久？" | QPS 趋势、连接数、磁盘/内存增长 |
| 趋势 | "是不是在变坏？" | 慢请求比例、推送延迟、在线人数曲线 |

Prometheus 在这些层次上做对了几件事，使它成为云原生时代的事实标准：

1. **多维数据模型**：一条指标可以有任意数量的 label（标签），能按服务、接口、返回码、用户维度切片聚合。
2. **Pull（拉取）模型**：监控 server 主动去被监控对象上拉 `/metrics`，被监控方只需要暴露一个 HTTP 端点，不需要安装 agent，天然适合容器。
3. **PromQL**：一门专门为时间序列设计的查询语言，可以做速率、分位数、聚合、预测。
4. **不依赖分布式存储**：单机 TSDB 就能扛住大量时序数据，部署简单。
5. **服务发现**：在 Kubernetes 里能自动发现 Pod 并抓取，不需要手工维护目标列表。

## 二、核心概念：数据模型与指标类型

### 2.1 时间序列（Time Series）

Prometheus 存的不是日志，而是**时间序列**：一串按时间排列的 `(timestamp, value)` 点，由"指标名 + 一组 label"唯一确定。

```
rpc_count{name="push", path="/push.PushService/FcmUpdateToken", code="OK"}  12
```

这里：

- `rpc_count` 是指标名（metric name）；
- `{name="push", path="...", code="OK"}` 是标签集，标签不同就是不同的时间序列；
- `12` 是当前采样值。

**label 是 Prometheus 的灵魂**。同样是 `rpc_count`，加上 `name/path/code` 三个维度后，你就能回答"push 服务的 FcmUpdateToken 接口成功了多少次"这种细粒度问题。代价是**label 基数（cardinality）**：label 取值组合越多，序列数越多，存储和查询成本越高。所以**永远不要把 userID、msgID 这种高基数值放进 label**——这是新手最常踩的坑。

### 2.2 四种指标类型

| 类型 | 语义 | 典型例子 | PromQL 常用函数 |
| --- | --- | --- | --- |
| **Counter** | 只增不减的计数器，进程重启归零 | 请求数、错误数、消息处理数 | `rate()`、`increase()` |
| **Gauge** | 可增可减的瞬时值 | 在线人数、内存使用、队列长度 | 直接取值、`avg_over_time()` |
| **Histogram** | 分桶统计观测值分布 | 请求耗时、响应大小 | `histogram_quantile()` |
| **Summary** | 客户端直接计算分位数 | 类似 Histogram，但服务端不可再聚合 | `quantile` |

SuIM 当前实现以 Counter 和 Gauge 为主（对应 OpenIM 的指标类型），gRPC 的耗时分布由 `go-grpc-prometheus` 的 Histogram 提供（我们在实现中选择了更轻量的自研计数器，耗时直方图作为后续可选项）。

**Counter 的关键性质**：它是单调递增的，所以原始值本身意义不大，真正有用的是它的**变化率**：

```promql
# 过去 5 分钟每秒的 RPC 调用速率
rate(rpc_count[5m])

# 按服务名聚合的错误速率（gRPC code != OK）
sum by (name) (rate(rpc_count{code!="OK"}[5m]))
```

进程重启会让 Counter 归零，`rate()` 能自动识别这种重置（reset）并做补偿，这也是必须用 `rate` 而不是直接相减的原因。

### 2.3 暴露格式：Exposition Format

被监控对象通过 HTTP 返回一个纯文本格式的指标：

```
# HELP rpc_count Total number of RPC calls.
# TYPE rpc_count counter
rpc_count{code="OK",name="push",path="/push.PushService/DelUserPushToken"} 3
# HELP go_goroutines Number of goroutines that currently exist.
# TYPE go_goroutines gauge
go_goroutines 19
```

Go 生态里这件事由 `prometheus/client_golang` 的 `promhttp.HandlerFor(registry, ...)` 完成：它把 Registry 里注册的所有 Collector 收集一遍，序列化成上面的文本。SuIM 的 `pkg/prommetrics` 就是基于这个库构建的。

## 三、架构与工作原理

### 3.1 Pull 模型与整体架构

```
                 ┌──────────────────────┐
                 │   Prometheus Server  │
                 │  ├─ Retrieval（拉取） │
                 │  ├─ TSDB（时序存储）  │
                 │  └─ PromQL 引擎       │
                 └──────────┬───────────┘
                            │  HTTP GET /metrics（按 scrape_interval 定时拉取）
                            │
       ┌────────────────────┼────────────────────┐
       ▼                    ▼                    ▼
 ┌───────────┐        ┌───────────┐        ┌───────────┐
 │ user:16080│        │ push:16088│        │ msg:16085 │  ... 12 个服务
 │ /metrics  │        │ /metrics  │        │ /metrics  │
 └───────────┘        └───────────┘        └───────────┘

  Prometheus ──告警规则──> Alertmanager ──> 钉钉/邮件/企业微信
  Prometheus ──PromQL──> Grafana（可视化大盘）
```

**为什么是 Pull 而不是 Push？**

- 被监控方只需暴露一个 HTTP 端点，不依赖 Prometheus SDK 的连接配置，解耦。
- Prometheus 能控制抓取节奏，并通过"上一次抓取是否成功"直接判断目标是否存活（`up` 指标）。
- 容器环境下 IP 随时变，Pull 配合服务发现天然适配；Push 模式下每个实例都要配置 Prometheus 地址。
- 缺点：短生命周期的任务（跑完就退出）可能来不及被抓到，这类场景才需要 Pushgateway。SuIM 的 12 个服务都是常驻进程，不需要 Pushgateway。

### 3.2 核心组件

- **Prometheus Server**：定时从配置的目标拉取指标，存入本地 TSDB，提供 PromQL 查询和告警规则计算。
- **Exporter**：把第三方系统的指标转成 Prometheus 格式（如 node_exporter 暴露机器指标）。SuIM 的服务自己内置了 `/metrics`，不需要额外 exporter。
- **Alertmanager**：接收 Prometheus 触发的告警，负责去重、分组、静默、路由到通知渠道。
- **Pushgateway**：给短任务用的指标中转站（SuIM 未使用）。

### 3.3 服务发现

Prometheus 通过 `scrape_configs` 配置抓取目标。在 Kubernetes 里最常用的是 pod discovery：

```yaml
- job_name: suim
  kubernetes_sd_configs:
    - role: pod
  relabel_configs:
    - action: keep
      source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
      regex: "true"
    - action: replace
      source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port]
      target_label: __port__
```

SuIM 在每个 Pod 上打的三个注解就是给这套机制用的：

```yaml
prometheus.io/scrape: "true"
prometheus.io/path: "/metrics"
prometheus.io/port: "16088"
```

Prometheus 扫到 Pod 后，按注解里的端口和路径去抓，Pod IP 变化也不用改配置。

### 3.4 PromQL 入门

PromQL 是查询时序数据的语言，几个最常用的模式：

```promql
# 1. 瞬时值：当前在线人数
online_user_num

# 2. 速率：push 服务每秒离线推送失败数
rate(msg_offline_push_failed_total[5m])

# 3. 按标签过滤 + 聚合：所有服务过去 5 分钟的非 OK RPC 速率
sum by (name) (rate(rpc_count{code!="OK"}[5m]))

# 4. 比例：成功数 / (成功数 + 失败数)
rate(single_chat_msg_process_success_total[5m])
  / (rate(single_chat_msg_process_success_total[5m]) + rate(single_chat_msg_process_failed_total[5m]))

# 5. 预测：按当前趋势，4 小时后磁盘会不会满（需要 node_exporter）
predict_linear(node_filesystem_free_bytes[6h], 4*3600) < 0
```

`[5m]` 是时间范围选择器，`rate` 只能用在 Counter 上。理解 Counter + rate 这组搭配，就理解了 Prometheus 80% 的日常使用。

## 四、OpenIM 的 metrics 设计（参考来源）

OpenIM 在 `pkg/common/prommetrics/` 里定义了 **17 个自定义指标**，设计上三个关键点值得借鉴：

1. **按服务分发，而不是全量注册**：每个服务启动时通过 `GetGrpcCusMetrics(registerName, discovery)` 只拿自己模块的 collector，注入独立的 Registry，通过独立的 `/metrics` 端口暴露。例如 push 服务只暴露 `msg_offline_push_failed_total`、`msg_long_time_push_total`，不会看到 user 的注册数。
2. **通用指标 + 业务指标分层**：`rpc_count{name,path,code}`、`api_count`、`http_count` 是所有服务都有的通用指标；业务指标按服务归属（online_user_num / user_register_total / 消息处理成败等）。
3. **服务发现集成**：metrics 地址通过 discovery key（`prometheus_discovery/...`，TTL=300）注册，配合 `/prometheus_discovery/*` API 实现动态 target。

SuIM 借鉴了前两点；第三点我们用 Kubernetes 的 Pod 注解 + Prometheus 原生服务发现替代，避免改动 `pkg/register` 的元数据编码（那会带来覆盖业务服务注册的风险）。

## 五、SuIM 中的实现

### 5.1 整体设计

SuIM 没有 OpenIM 那样的统一启动器（`startrpc`），12 个服务各自在 `services/<name>/<name>.go` 里手动构建 gRPC/HTTP server。因此实现时遵循三条原则：

- **自管理**：`prommetrics.Start()` 内部监听 ctx，取消时自动优雅关闭，各服务入口只需一行调用，不需要改 errCh 容量或退出逻辑。
- **独立 Registry**：`ForService(name)` 给每个服务返回一个只含自身指标的 Registry，与 OpenIM 的分发模型同构。
- **命名对齐**：指标名、label 名尽量与 OpenIM 一致，迁移 Grafana 面板和告警规则时几乎不用改。

### 5.2 代码结构

```
pkg/prommetrics/
├── metrics.go       # 指标定义、ForService 分发、Start 自管理 server
├── interceptor.go   # gRPC UnaryServerInterceptor + HTTPMiddleware
└── metrics_test.go  # 单测：分发、拦截器、/metrics 端到端
```

**指标定义**（`metrics.go`）用 `prometheus.NewCounter` / `NewGauge` 声明，通用指标用 `CounterVec` 带 label：

```go
var RPCCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
    Name: "rpc_count",
    Help: "Total number of RPC calls.",
}, []string{"name", "path", "code"})
```

**按服务分发**：

```go
var serviceCollectors = map[string][]prometheus.Collector{
    "user":        {UserRegisterCounter},
    "msggateway":  {OnlineUserGauge},
    "msg":         {SingleChatMsgSuccess, SingleChatMsgFailed, GroupChatMsgSuccess, GroupChatMsgFailed},
    "msgtransfer": {MsgInsertRedisSuccess, MsgInsertRedisFailed, MsgInsertMySQLSuccess, MsgInsertMySQLFailed, SeqSetFailed},
    "push":        {MsgOfflinePushFailed, MsgLongTimePush},
    "apigateway":  {UserLoginCounter, APICounter, HTTPCounter},
}

func ForService(name string) *prometheus.Registry {
    reg := prometheus.NewRegistry()
    for _, c := range baseCollectors { reg.MustRegister(c) } // process/go
    reg.MustRegister(RPCCounter)
    for _, c := range serviceCollectors[name] { reg.MustRegister(c) }
    return reg
}
```

**自管理 server**：

```go
func Start(ctx context.Context, serviceName, listenIP string, port int) *Server {
    if port <= 0 { return nil }
    lis, _ := net.Listen("tcp", fmt.Sprintf("%s:%d", listenIP, port))
    mux := http.NewServeMux()
    mux.Handle(MetricsPath, promhttp.HandlerFor(ForService(serviceName), promhttp.HandlerOpts{}))
    srv := &Server{listener: lis, httpServer: &http.Server{Handler: mux}}
    go func() { <-ctx.Done(); srv.httpServer.Shutdown(...) }() // ctx 取消自动关闭
    go func() { srv.httpServer.Serve(lis) }()
    return srv
}
```

注意 `port <= 0` 时不启动——这让所有服务都能用同一份代码，未配置 metricsPort 时零开销、无副作用。

### 5.3 完整指标清单

| 服务 | 指标名 | 类型 | 含义 |
| --- | --- | --- | --- |
| 通用 | `rpc_count{name,path,code}` | Counter | 每个 gRPC 方法按返回码计数 |
| 通用 | `http_count{path,method,status}` | Counter | HTTP 请求计数（apigateway） |
| 通用 | `api_count{path,method,code}` | Counter | API 业务码计数（apigateway） |
| 通用 | `process_*` / `go_*` | — | 进程与 Go runtime 指标 |
| user | `user_register_total` | Counter | 注册成功数 |
| user | `user_login_total` | Counter | 登录成功数（apigateway 侧也暴露） |
| msg | `single_chat_msg_process_success_total` / `..._failed_total` | Counter | 单聊消息处理成败 |
| msg | `group_chat_msg_process_success_total` / `..._failed_total` | Counter | 群聊消息处理成败 |
| msgtransfer | `msg_insert_redis_success_total` / `..._failed_total` | Counter | 写 Redis 批次成败 |
| msgtransfer | `msg_insert_mysql_success_total` / `..._failed_total` | Counter | 写 MySQL 成败 |
| msgtransfer | `seq_set_failed_total` | Counter | seq 设置失败 |
| msggateway | `online_user_num` | Gauge | 本实例在线用户数 |
| push | `msg_offline_push_failed_total` | Counter | 离线推送失败 |
| push | `msg_long_time_push_total` | Counter | 处理超过 10 秒的慢推送 |

> 与 OpenIM 的差异：OpenIM 的 msgtransfer 指标是 `msg_insert_mongo_*`，SuIM 用 MySQL，所以改为 `msg_insert_mysql_*`；SuIM 没有独立 auth 服务，登录指标放在 apigateway 侧。

### 5.4 12 个服务的接入方式

每个服务的 `Config` 增加了 `MetricsPort int \`yaml:"metricsPort"\`` 字段。入口处统一一行：

```go
prommetrics.Start(ctx, "push", cfg.ListenIP, cfg.MetricsPort)
grpcSrv := grpc.NewServer(grpc.ChainUnaryInterceptor(
    mw.UnaryRecover,
    rpcctx.ServerInterceptor,
    prommetrics.UnaryServerInterceptor("push"),  // ← RPC 计数
))
```

对于不同形态的服务：

- **8 个纯 gRPC 服务**（user/group/third/relation/conversation/msg/push/rtc）：追加 metrics 拦截器到 `ChainUnaryInterceptor`。
- **msggateway**：gRPC 侧追加拦截器；WS/HTTP 侧不用改，metrics 是独立监听。
- **apigateway**：没有 gRPC server，用 `prommetrics.HTTPMiddleware(...)` 包住业务 handler 计数 HTTP 请求。
- **msgtransfer / crontask**：没有监听端口，但也启动独立的 metrics listener（监听地址用 `0.0.0.0`）。

端口分配（ConfigMap 中 `metricsPort`）：

```
user 16080  group 16081  third 16082  relation 16083
conversation 16084  msg 16085  msggateway 16086  msgtransfer 16087
push 16088  rtc 16089  apigateway 16090  crontask 16091
```

### 5.5 业务埋点

**gRPC 拦截器**（`interceptor.go`）自动记录每个 RPC 的返回码，无需业务代码参与：

```go
func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
        resp, err := handler(ctx, req)
        RPCCall(serviceName, info.FullMethod, int(status.Code(err)))
        return resp, err
    }
}
```

用 `status.Code(err)` 取 gRPC 规范返回码（OK / InvalidArgument / Internal / ...），与 OpenIM 用数字 code 的思路一致，只是语义更标准。

**业务埋点**则放在关键路径上：

- `services/user/internal/logic/register.go`、`login.go`：注册/登录成功时 `Inc()`。
- `services/msg/internal/logic/send_msg.go`：`SendMsg` 用命名返回 + defer，按 `SessionType` 决定计入单聊还是群聊、成功还是失败，一处覆盖所有 `return` 分支。
- `services/push/internal/logic/handle_push.go`：`dispatch` 失败时 `MsgOfflinePushFailed.Inc()`。
- `services/msgtransfer/internal/logic/mysql_handler.go` / `redis_handler.go`：写库成功/失败分别计数（Redis 用 defer + 命名返回覆盖所有错误路径）。
- `services/msggateway/internal/ws/online.go`：在定时续期 `renewOnline` 里 `Set(float64(len(s.hub.Snapshot())))`，刷新本实例在线用户数。

### 5.6 Kubernetes 集成

`deployments/apps/*.yml` 中每个 Deployment 的 Pod 模板加了注解，Service 加了 metrics 端口：

```yaml
template:
  metadata:
    annotations:
      prometheus.io/scrape: "true"
      prometheus.io/path: "/metrics"
      prometheus.io/port: "16088"   # 各服务不同
spec:
  containers:
    - ports:
        - { name: metrics, containerPort: 16088 }
```

这样任何支持标准 prometheus.io 注解的服务发现配置（Helm chart 里的 kube-prometheus-stack、自建 Prometheus 的 `kubernetes_sd_configs`）都能自动抓取，不需要为每个服务单独写 `scrape_config`。

本地开发时直接访问 `http://<ip>:<metricsPort>/metrics` 即可验证。实际验证过启动 push 服务后：

```
go_goroutines 19
msg_long_time_push_total 0
msg_offline_push_failed_total 0
process_cpu_seconds_total 0
```

### 5.7 测试保障

`pkg/prommetrics/metrics_test.go` 覆盖了四个关键点：

1. `ForService("push")` 只注册 push 的指标，不含 user 的指标；
2. gRPC 拦截器确实增加 `rpc_count`；
3. HTTP middleware 确实增加 `http_count`；
4. `Start()` 端到端起一个 `/metrics` 端点，`curl` 能拿到指标，ctx 取消后能正常 shutdown。

有一个容易踩的细节：**空的 `CounterVec` 在 `Gather()` 时不会出现**（没有任何 label 实例就没有时间序列），所以测试里验证注册情况要检查那些确定有值的指标（如 `go_goroutines`），而不是 `rpc_count`。

## 六、实战：告警与排查

有了指标之后，真正发挥价值的是告警规则和排查手册。几个贴合 SuIM 的例子：

```promql
# 1. 任意服务 gRPC 错误率超过 5%（5 分钟窗口）
sum by (name) (rate(rpc_count{code!="OK"}[5m]))
  / sum by (name) (rate(rpc_count[5m])) > 0.05

# 2. 离线推送失败持续 10 分钟（每分钟 > 10 次）
rate(msg_offline_push_failed_total[10m]) * 60 > 10

# 3. 消息处理失败突增
increase(msg_insert_mysql_failed_total[5m]) > 50

# 4. 服务实例消失（Prometheus 抓取失败）
up{job="suim"} == 0

# 5. 在线人数异常归零（对比 1 小时前）
online_user_num == 0 and online_user_num offset 1h > 100
```

排查思路一般是：**先看 `up` 确认服务活着 → 看 `rpc_count` 定位哪个服务/方法/返回码出问题 → 看业务指标确认影响面（消息失败数、推送失败数）→ 看 `process_*`/`go_*` 排除资源问题**。这条链路就是 Prometheus 从"告警"到"定位"的完整价值。

## 七、常见坑与注意事项

1. **label 基数爆炸**：不要把 userID、msgID、conversationID 放进 label；`rpc_count` 的 `path` 是 gRPC 方法名（有限集合），`code` 是 gRPC 规范码（有限集合），都是安全的。
2. **Counter 要用 rate 看**：直接看 Counter 原始值只能看累计增长；重启归零后 `rate()` 会自动补偿。
3. **Gauge 的语义**：`online_user_num` 是当前实例的值，多副本下要 `sum()` 才是全系统在线数；而且它是定时刷新的，不是实时精确值。
4. **空 CounterVec 不出现在 Gather 结果里**：写测试或调试时别被这个现象误导。
5. **避免重复计数**：`msgtransfer` 的 Redis 批次埋点最初用 defer 写成了"无条件成功计数"，后来改为命名返回 + 判断 `err != nil` 才分别计入成败。埋点代码本身也要测试。
6. **依赖版本**：添加 `prometheus/client_golang` 时如果用太低版本（如 v1.20.5）会连锁降级 etcd 等关键依赖；SuIM 直接复用 go.mod 里已有的 v1.23.2，避免污染依赖树。
7. **服务发现的取舍**：OpenIM 把 metrics target 写进 etcd discovery key；SuIM 用 K8s Pod 注解 + 原生服务发现，更简单，但要求 Prometheus 有对应的 `kubernetes_sd_configs` 配置。

## 八、总结

Prometheus 的本质是**把系统状态建模成带标签的时间序列，通过 Pull 统一采集，再用 PromQL 查询和告警**。理解 Counter/Gauge、label 基数、Pull 模型这三点，就理解了它大半。

SuIM 的实现以 OpenIM 为蓝本但做了适配：17 个指标按服务分发、每服务独立的 `/metrics`、gRPC 拦截器自动记录 `rpc_count`、业务关键路径手动埋点、K8s 注解驱动的自动发现。整套方案代码集中在 `pkg/prommetrics/`，各服务入口只需一行 `Start()` 加一个拦截器，对现有架构的侵入极小。

下一步可以补的方向：Histogram 耗时分布（P99 延迟）、etcd/Redis/MySQL 的 exporter、Grafana 大盘、以及基于 `up` + 错误率的完整告警规则文件。
