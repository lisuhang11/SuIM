# Kafka 消息堆积问题详解

> 一篇可以直接当排查手册用的长文：从"什么才算堆积"讲起，覆盖根因、定位 SOP、应急止血、参数根治、次生风险与预防体系，命令和配置都能直接复制。

## 一、先把概念说清楚：什么才叫"堆积"

**消息堆积（Message Backlog），在 Kafka 语境里的标准叫法是 Consumer Lag（消费者滞后）**，指的是"已经写进 Topic、但消费组还没消费掉"的消息数量。

### 1.1 计算口径

Kafka 的每个分区都是一条只追加的日志，日志里的每条消息有一个单调递增的编号，叫 **位点（offset）**。堆积量就是两个位点的差：

```
单分区堆积量 = 分区最新位点(Log End Offset) − 消费组已提交位点(Committed Offset)
Topic 堆积量 = 该 Topic 所有分区堆积量之和
消费组堆积量 = 该组订阅的所有 Topic、所有分区堆积量之和
```

命令行里对应的就是 `LAG` 那一列：

```bash
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --describe --group order-consumer-group
```

```
GROUP                 TOPIC        PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG     CONSUMER-ID     HOST         CLIENT-ID
order-consumer-group  order.paid   0          1823401         1840223         16822   consumer-1-...  10.0.1.11    consumer-1
order-consumer-group  order.paid   1          1819988         1838410         18422   consumer-2-...  10.0.1.12    consumer-2
order-consumer-group  order.paid   2          1825611         1841002         15391   consumer-3-...  10.0.1.13    consumer-3
```

三个容易看漏的点：

- **`CONSUMER-ID` 为空**，说明这个分区当前没有任何消费者在消费。这是最该先看的一列。
- **`CURRENT-OFFSET` 是"已提交"的位点，不是"已处理"的位点**。这两者的差别，是后面"假堆积"的根源。
- **`LOG-END-OFFSET` 在高水位（HW）意义上是"所有 ISR 副本都已同步"的位置**，不是 leader 本地最后一条的位置。

### 1.2 Lag 有两个维度，别只盯条数

| 维度                           | 含义                                         | 用途                                         |
| ------------------------------ | -------------------------------------------- | -------------------------------------------- |
| **Offset lag（位点差）**       | 未消费的消息条数                             | 反映"还欠多少活"，是容量和追平时间估算的输入 |
| **Consumer latency（时间差）** | 当前正在处理的消息，其生产时间与现在的时间差 | 反映"业务数据有多旧"，直接对应业务影响       |

**生产上真正该告警的是时间差。** 条数是个绝对量，1 万条对高吞吐 Topic 是 1 秒的事，对低吞吐 Topic 可能是 3 小时的事。同一个阈值套到所有 Topic 上，一定会出现"要么天天误报、要么真出事不报"。

### 1.3 堆积量大 ≠ 出故障了

这是新手最容易踩的判断错误。举个例子：

某 Topic 生产速率 10000 TPS，消费端配置了自动提交位点，`auto.commit.interval.ms=1000`（1 秒提交一次）。那么即使消费能力完全跟得上，监控上看到的堆积量也会**常年在 10000 条上下波动**——因为这 1 秒内生产的消息还没到提交时机。

**判断是不是真故障，看两个信号：**

1. 堆积量是否**持续单调扩大**（而不是围绕某个基线波动）
2. 消费位点是否**在前进**

组合起来就是这张判别表：

| 最近消费时间   | 消费位点   | 堆积量趋势     | 结论                                             |
| -------------- | ---------- | -------------- | ------------------------------------------------ |
| 与当前时间接近 | 持续前进   | 在基线附近波动 | **正常**，不是堆积                               |
| 与当前时间接近 | 持续前进   | 持续扩大       | **消费慢于生产**，查消费耗时                     |
| 很久以前       | 长时间不变 | 持续扩大       | **消费线程卡住**，真故障，最高优先级             |
| 与当前时间接近 | 长时间不变 | 持续扩大       | 可能是 `assign` 模式自管位点未提交，属**假堆积** |

最后一行值得展开：如果消费端用的是 `assign()` 手动指定分区、位点由业务自己管理（比如存在数据库里），那么 Kafka 侧看到的 `CURRENT-OFFSET` 永远不会动，监控上的堆积量会一直涨，但业务其实早就消费完了。这种情况的解法是按 Kafka 的协议把位点提交上去，让监控读数恢复真实。

### 1.4 position 与 committed position

Kafka 消费者内部维护两个位置，混淆它们会得出错误结论：

- **position**：下一次 `poll()` 会拉取的位点。每次 `poll()` 拿到消息后自动前进，**只存在于消费者进程内存里**。
- **committed position**：已经安全提交到 `__consumer_offsets` 的位点。进程重启、分区被重新分配后，**从这里恢复**。

推论：**如果监控系统用 committed offset 算 lag，而应用已经处理完但还没提交，你看到的数字会偏大**；反过来，如果应用提前提交（先提交再处理），数字会很好看，但业务结果根本没落地。所以看到 lag 数字，第一件事是确认这个数字是谁算的、用的是哪个位点。

### 1.5 一个高级坑：`read_committed` 与 LSO

如果消费端配置了 `isolation.level=read_committed`（读已提交，配合生产端事务使用），那么它能读到的上界不是高水位，而是 **LSO（Last Stable Offset）**——第一个"尚未提交或中止的开放事务"的起始位置。

这意味着：**一个长时间不提交的事务生产者，会让所有 `read_committed` 消费者的 lag 持续增长，而消费者本身完全健康、处理能力绰绰有余。**

排查时如果 lag 涨得离谱但消费端一切正常，一定要看：消费者的 `isolation.level` 是什么、生产端有没有开着事务、有没有卡住未提交的事务。另外，`read_uncommitted` 和 `read_committed` 两种模式下算出来的 lag 数值口径不同，**不能直接比较**。

## 二、堆积的本质：一道不等式

抛开所有细节，消息堆积只有一句话：

```
生产速率 > 消费速率，且这个状态持续了足够长的时间
```

堆积量就是这条不等式在时间轴上的积分。要解决问题，就得把"消费速率"拆开：

```
消费速率 ≈ 有效并行度 × 单消费者吞吐

有效并行度 = min(分区数, 消费者实例数 × 每实例消费线程数)
                              ↑ 天花板永远是分区数

单消费者吞吐 = 每次 poll 拉取的条数 / 处理完这批的耗时
```

```
                       ┌──────────── 分区 0 ────────────┐
   Producer ──────►    ├──────────── 分区 1 ────────────┤  ──►  Consumer Group  ──►  下游
  （生产速率）         └──────────── 分区 N ────────────┘      （消费速率）        （DB / HTTP / MQ）
                              ↑                                    ↑
                     并行度的天花板                          三个瓶颈点：
                                                          ① 拉不到（网络/带宽/参数）
                                                          ② 处理慢（业务逻辑/下游）
                                                          ③ 根本没在消费（卡死/Rebalance）
```

这个拆解直接给出了四类根因方向：

| 方向           | 对应不等式的哪一项 | 典型场景                                |
| -------------- | ------------------ | --------------------------------------- |
| **生产端变快** | 分子变大           | 大促、批处理回刷、上游 bug 死循环发消息 |
| **并行度不足** | 分母第一项         | 分区太少、消费者多于分区、热点 key 倾斜 |
| **单条处理慢** | 分母第二项         | 慢 SQL、外部调用无超时、逐条 IO、大消息 |
| **没在消费**   | 分母整体归零       | 消费者卡死、Rebalance 风暴、位点异常    |

第四类最危险，因为它不是"慢"，是"停"。而线上事故里，**"停"和"慢"的处置方式完全不同**：慢要扩容和优化，停要先找到卡点。所以下一章的根因清单里，判断顺序很重要。

## 三、根因清单

按线上实际出现频率排序。前两类占绝大多数，但第三类（Rebalance）造成的事故最惨烈。

### 3.1 消费逻辑慢（出现频率最高）

单条消息处理耗时从毫秒级退化到秒级，是堆积最常见的直接原因。

- **逐条同步 IO**：`for` 循环里一条消息查一次库、写一次库、调一次 HTTP。1000 条消息就是 3000 次网络往返，即使每次只要 5 ms，也要 15 秒。**这是最常见的性能杀手，也是最容易改造的。**
- **下游依赖慢**：慢 SQL、缺索引、大事务、连接池打满、Redis 大 key。表现为消费耗时曲线和数据库慢查询曲线同步上扬。
- **外部调用没设超时**：这是最阴的一种。HTTP 客户端默认无超时或超时几十分钟，一旦对端挂起，消费线程就永久阻塞在那里。`jstack` 一打就能看到线程卡在 `SocketInputStream.read`。**连接池被这些挂起的调用占满后，整个消费组假死。**
- **消息体过大**：反序列化开销、GC 压力、网络传输时间同时上升。
- **同步日志**：每条消息打几行 INFO 日志，日志框架是同步 appender，磁盘 IO 成为瓶颈。
- **JVM Full GC**：一次 STW 停顿几十秒到几分钟，期间所有线程冻结。GC 结束后，两次 `poll()` 的间隔早就超过 `max.poll.interval.ms` 了，直接触发 Rebalance（见 3.3）。

### 3.2 并行度不足

Kafka 有一条铁律：**同一个消费组内，一个分区在同一时刻只能被一个消费者消费。**

由此推出三个结论：

1. **分区数就是消费并行度的天花板。** 分区数是 6，起 20 个消费者实例，也只有 6 个在干活，其余 14 个空跑。
2. **加消费者只在"实例数 < 分区数"时有效。** 这是应急扩容前必须先看的一件事。
3. **分区只能增加，不能减少。** 一旦扩上去就回不来了。

并行度不足的具体形态：

| 形态              | 现象                                 | 处理                                  |
| ----------------- | ------------------------------------ | ------------------------------------- |
| 分区数太少        | 所有消费者 CPU 都很闲，但 lag 一起涨 | 扩分区（需评估顺序性影响）            |
| 消费者多于分区    | 部分消费者长期 0 消息，资源浪费      | 减少消费者数                          |
| **热点 key 倾斜** | **个别分区 lag 巨大，其余分区正常**  | 改 key 设计（见 6.5）                 |
| 分配策略倾斜      | 某个消费者拿到的分区明显多于别人     | 换 `RoundRobin` / `CooperativeSticky` |
| leader 集中       | 某台 broker 磁盘 IO 打满，其余空闲   | 分区副本重分配                        |

**热点 key 倾斜值得单独强调**，因为它的诊断特征极其鲜明：`kafka-consumer-groups.sh --describe` 的输出里，一两个分区的 LAG 是几十万，其他分区是几百。这基本可以断定是数据倾斜——通常是拿商户号、用户 ID、设备号做 key，而某个大客户占了当天流量的三四成，所有消息全压在同一个分区上。

这种情况下**加多少消费者都没用**，因为那个热点分区只能被一个消费者串行处理。

### 3.3 Rebalance 风暴（事故最惨烈的一类）

**Rebalance（重平衡）是消费组内分区与消费者的重新分配过程。** 在使用 eager 协议（传统协议）时，Rebalance 期间**整个消费组的所有消费者都会停止消费**，等待新的分区分配完成。组越大、分区越多，这个停顿越长——几十上百个消费者、上千个分区的组，停顿可能持续几十秒。

停顿期间生产端还在写，于是 lag 跳变式上涨。

**七类触发场景：**

| 触发条件          | 说明                                             | 常见诱因                          |
| ----------------- | ------------------------------------------------ | --------------------------------- |
| 消费者加入        | 扩容、新实例上线                                 | 主动扩容、滚动发布                |
| 消费者离开        | 宕机、正常关闭、被踢出                           | OOM、K8s Pod 被驱逐、优雅停机     |
| 心跳超时          | `session.timeout.ms` 内没收到心跳                | 网络抖动、Full GC、心跳线程被阻塞 |
| **poll 间隔超时** | **`max.poll.interval.ms` 内没再次调用 `poll()`** | **单批处理太慢（最常见）**        |
| 分区数变化        | Topic 新增分区                                   | 手动扩分区                        |
| 订阅关系变化      | `subscribe()` 的 Topic 列表变了                  | 代码变更、配置变更                |
| Coordinator 变更  | 组协调者所在 broker 故障                         | broker 宕机、controller 切换      |

**"假死"机制——理解 Rebalance 事故的关键**

Kafka 0.10.1 之后，消费者内部拆成了两个线程：

```
┌──────────────────────────────────────────────────────┐
│  消费者进程                                          │
│                                                      │
│  心跳线程 ──每 heartbeat.interval.ms──► Coordinator  │
│    判定阈值：session.timeout.ms（进程存活）           │
│                                                      │
│  主线程 ──poll() → 处理业务 → poll() → ...           │
│    判定阈值：max.poll.interval.ms（业务存活）         │
└──────────────────────────────────────────────────────┘
```

问题就出在这两个判定是**独立**的。假设：

- 一次 `poll()` 拉到 500 条消息（`max.poll.records` 默认值）
- 因为下游写库变慢，处理这 500 条花了 6 分钟
- 这 6 分钟里，心跳线程一切正常，日志也在正常刷，进程活得好好的
- 但在 Coordinator 看来：这台机器已经 6 分钟没调用 `poll()` 了，超过 `max.poll.interval.ms`（默认 5 分钟）的死线

于是服务端判定它**失去了消费能力**（占着分区却不拉数据），强制踢出消费组，触发 Rebalance。这就是所谓"假死"——**进程存活，但业务不存活。**

更糟的是，这里会形成一个**正反馈的死亡螺旋**：

```
消息堆积 → 每次 poll 拉满 500 条 → 处理时间变长 → 超过 max.poll.interval.ms
    ↑                                                              ↓
    └──── 消费能力进一步下降 ←── 被踢出组，触发 Rebalance ←──────────┘
```

一旦进入这个循环，所有消费实例会轮流被踢、轮流重启，组一直在 Rebalance，**实际有效消费时间趋近于零**，堆积以生产速率的全速增长。线上见过最惨的案例，就是三台机器轮流拉消息、又轮流被踢出组。

**识别 Rebalance 的日志关键字：**

```
(Re-)joining group
Successfully joined group with generation N
Attempt to heartbeat failed since group is rebalancing
The consumer is no longer part of the group
Consumer clientId=... Exiting since the group coordinator is no longer reachable
org.apache.kafka.clients.consumer.CommitFailedException
The time taken to process the batch of records exceeds the configured max.poll.interval.ms

```

其中 `CommitFailedException` 是最典型的信号：消费者辛辛苦苦处理完一批消息，去提交位点时发现自己早已被踢出组，提交失败——**这批消息会被新的消费者重新消费一遍**，造成重复。

**Rebalance 还会顺带引发两个问题：**

- **消息重复消费**：位点提交滞后于消息处理，Rebalance 后新消费者从上次提交的位置重来。
- **消息丢失**：如果开着自动提交（`enable.auto.commit=true`，默认 `auto.commit.interval.ms=5000`），位点可能在消息还没处理完时就被提交了。时序是：`poll` 到 offset 100–200 → 5 秒后自动提交 offset 200 → 处理到 150 时进程崩了 → 新消费者从 200 开始 → **150–199 的消息永久丢失**。

所以老运维常说一句话：**Kafka 出现消息积压、重复、丢失这三类问题，先看有没有 Rebalance，能解决一大半。**

### 3.4 消费者卡死与毒消息

- **异常后不提交位点，陷入死循环**：某条消息处理时抛异常，代码 catch 之后既不提交位点也不跳过，下一次 `poll()` 又拉到同一条，又抛异常……消费者永远卡在同一个 offset 上，lag 只增不减。**特征是：位点长时间完全不动，但消费者进程活着、心跳正常。**
- **毒消息（poison pill）**：消息体超过消费端的 `max.partition.fetch.bytes`、反序列化失败、字段缺失导致校验不通过、上游 schema 变更导致新旧格式混合。一条毒消息能卡住整个分区。
- **死锁 / 线程池阻塞**：业务代码死锁，或者线程池队列满了用了不合适的拒绝策略。顺带说一句，如果用 `CallerRunsPolicy`（队列满就让 poll 线程自己执行），反而是**好事**——它天然形成背压，让 `poll()` 变慢，从而把压力反馈回去；用 `AbortPolicy` 直接丢任务，等于丢消息。
- **手动提交漏了异常分支**：`commitSync()` 没放在 `finally` 或者只在成功路径上调用，异常路径不提交，重启后大量重复消费。

### 3.5 位点相关的"假堆积"

lag 数字很大，但业务其实没问题。四种情况：

1. **`assign` 模式自管位点**：见 1.3，业务自己存位点，没往 Kafka 提交。
2. **自动提交间隔太长**：`auto.commit.interval.ms` 设成 30 秒，那 lag 常态就至少是 30 秒的生产量。
3. **位点过期**：`__consumer_offsets` 里的位点也有保留期，由 broker 端 `offsets.retention.minutes` 控制（默认 7 天）。**消费组停用超过这个时间，位点就被清掉了**。再次启动时找不到位点，按 `auto.offset.reset` 决定从头（`earliest`）还是从尾（`latest`）开始——`earliest` 会瞬间产生海量重复消费，`latest` 会直接丢掉中间所有消息。
4. **长事务卡住 LSO**：见 1.5。

### 3.6 生产端突增

消费端一切正常，纯粹是进水口开大了。

- 大促、秒杀、直播开播等**可预期**的流量高峰
- 批处理任务、数据回刷、历史数据迁移等**周期性**的大量写入
- 上游 bug：死循环发消息、重试风暴、消息体突然变大（比如把 base64 图片塞进了消息体）
- 新增了一个高流量的生产者

诊断方法：对比 broker 的 `MessagesInPerSec` 指标在 lag 跳变时间点是否有激增；或者用 `kafka-get-offsets.sh` 采样两次各分区的最新位点，算出实际写入速率。

**关键判断：这个突增是临时的还是持续的？** 临时的（比如一次数据回刷），且堆积能在消息保留期内自然追平，那就不需要动手，等着就行。持续的，就按并行度和吞吐的方案扩容。

### 3.7 服务端与基础设施

- **broker 磁盘 IO 饱和**：分区数过多导致每块盘上分区密度过高，吞吐下降。
- **带宽限流**：云上 Kafka 实例有规格化的带宽上限，消费流量打到上限就会被限流。这时消费端怎么调都没用，得升配。
- **broker Full GC / CPU 打满**：GC 停顿期间所有请求排队，消费端表现为 fetch 延迟飙升。
- **网络问题**：消费者跨可用区、跨地域访问 broker；丢包、抖动。
- **冷读放大效应（重要且容易被忽略）**：

正常情况下，消费者紧跟生产进度，读的都是刚写入的数据，**都在操作系统的 page cache 里，属于顺序热读**，Kafka 的性能优势正是建立在这个前提上。

一旦消费者宕机几小时后重启、或者新起了一个消费组从头消费，它要读的是几小时前的数据，**早就被挤出 page cache 了，变成磁盘随机读**。后果是：broker 磁盘 IO 飙升，不仅这个消费者的追平速度极慢，**还会拖垮同一台 broker 上所有其他 Topic 的正常消费**。

这就是为什么"追平堆积"往往比预想的慢得多，也是为什么大规模重放时要做限速和错峰。

## 四、定位 SOP：六步法

原则：**先看清楚堆积长什么样，再决定动手方向。** 最忌讳的是收到告警就重启服务。

### 第一步：看形态

```bash
# 查看消费组各分区的 lag 与成员分配
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --describe --group order-consumer-group

# 查看消费组状态与成员数
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --describe --group order-consumer-group --state --members

```

**间隔 1 分钟执行两次**，对比 `CURRENT-OFFSET` 和 `LOG-END-OFFSET` 的变化，就能算出实际的生产速率和消费速率。这一步的信息量最大，别省。

然后看 lag 曲线的形态，三种形态指向完全不同的根因：

| 曲线形态         | 特征                                           | 最可能的根因                               | 第一动作                                       |
| ---------------- | ---------------------------------------------- | ------------------------------------------ | ---------------------------------------------- |
| **线性匀速增长** | 近似直线斜率上升，短期不会自愈                 | 消费能力持续低于生产能力：下游慢、并发不足 | 量单条处理耗时 + 看并行度配比                  |
| **突发跳变**     | 某时刻突然跳升几千到几万                       | Rebalance、生产端突增、消费者实例宕机重启  | 查同时段的 Rebalance 记录和 `MessagesInPerSec` |
| **高位不归零**   | 长期在某个基数附近波动，消费能力充足也降不下去 | 毒消息、死信堆积、位点提交逻辑有问题       | 抓反复消费的消息样本，查死信队列               |

### 第二步：判断"没在消费"还是"消费得慢"

这一步决定了后续所有动作的方向。

```
位点在前进吗？
├── 完全不动 + CONSUMER-ID 为空     → 消费组根本没起来 / 全部实例挂了 → 查部署和进程存活
├── 完全不动 + CONSUMER-ID 有值     → 消费者卡死 → jstack 抓线程栈，看卡在哪
├── 在动，但增速 < 生产增速         → 吞吐不足 → 进第三步、第四步
└── 在动，且增速 > 生产增速         → 正在追平，估算追平时间即可（见 5.9）

```

抓线程栈：

```bash
jstack <pid> > /tmp/consumer-jstack.txt
grep -A 20 "KafkaListenerEndpointContainer\|org.apache.kafka.clients.consumer" /tmp/consumer-jstack.txt

```

卡在 `SocketInputStream.read` → 外部调用没设超时；卡在数据库驱动 → 慢 SQL 或连接池耗尽；卡在 `BLOCKED` → 锁竞争或死锁。

### 第三步：分区维度看是全局还是局部

回到第一步的输出，看 `LAG` 列在分区间的分布：

- **所有分区 lag 均匀上涨** → 整体消费能力不足，或者下游依赖全局性变慢（数据库、缓存、外部服务）。
- **个别分区 lag 巨大，其余正常** → 数据倾斜（热点 key）、该分区有毒消息、或该分区 leader 所在 broker 异常。

**这一步能避免最昂贵的误判**：把倾斜问题当成能力问题去扩容，扩多少都没用。

### 第四步：量处理耗时

在消费代码里加分段埋点，分别计时：

```
poll() 耗时  →  反序列化耗时  →  业务处理耗时  →  外部写入耗时  →  commit 耗时

```

**看 P99，不要看平均值。** 平均 50 ms 完全不能说明系统健康——如果有 1% 的消息需要几十秒，整个批次的位点提交就会被这条长尾拖住，进而触发 `max.poll.interval.ms` 超时。

同时监控客户端暴露的两个指标：`poll` 调用间隔的平均值和最大值。这两个值一旦接近 `max.poll.interval.ms`，Rebalance 就在眼前了。

### 第五步：查 Rebalance

- **客户端日志**：搜 3.3 节列出的关键字，重点看 `(Re-)joining group` 的频率和 `generation` 号的增长速度。generation 号几分钟就涨一次，说明组一直在抖。
- **Broker 端**：group coordinator 所在 broker 的日志。
- **云控制台**：主流云厂商都提供 Rebalance 详情页，能看到时间、耗时、次数、原因、是否有新成员加入。
- **同时检查**：K8s 事件里有没有 Pod 被驱逐/OOMKilled 的记录；JVM 的 GC 日志里有没有长时间 Full GC。

### 第六步：查服务端与资源

两侧都要看：

**Broker 侧：**

| 指标                        | 说明               | 关注点                       |
| --------------------------- | ------------------ | ---------------------------- |
| `MessagesInPerSec`          | 写入速率           | 判断是否生产端突增           |
| `BytesOutPerSec`            | 出流量             | 是否打到带宽上限被限流       |
| `UnderReplicatedPartitions` | 副本不同步的分区数 | > 0 说明 broker 有压力或故障 |
| `RequestQueueTimeMs`        | 请求排队时间       | 升高说明 broker 处理能力不足 |
| 磁盘使用率 / IO util        | 存储压力           | 建议 80% 告警                |
| JVM GC 时间                 | broker 停顿        | 单次停顿 > 1s 就要查         |

**消费者侧机器：** CPU、内存、网卡流量、JVM 堆使用与 GC 频率、数据库连接池使用率、线程池活跃线程数与队列长度。

## 五、应急止血

**第一原则：先定位，再动手。不要上来就重启消费者。**

重启的代价被严重低估了：它会触发 Rebalance（整组停消费）、会让未提交的位点回退（造成重复消费）、如果是从头消费还会引发冷读拖垮 broker。**大多数情况下，重启会让堆积更严重。**

下面八种手段，按性价比排序。

### 5.1 加消费者实例（最快见效）

**前提：当前实例数 < 分区数。** 先跑一遍 `--describe` 数一下成员数，等于或超过分区数就别加了，加了也是空跑。

扩容后观察 10 分钟，确认 lag 增长率是否下降。**如果扩容无效，说明瓶颈不在并发度，而在单条消息处理耗时**——这时候该去优化处理逻辑，而不是继续加机器。

### 5.2 消费逻辑批量化（改造成本低，收益最大）

把逐条 IO 改成批量 IO。这一项通常能带来 2–3 倍的吞吐提升，比加机器划算得多。

```java
// ❌ 改造前：一条消息查一次库、写一次库，1000 条就是 2000 次 IO
for (ConsumerRecord<String, String> r : records) {
    Order o = orderMapper.selectByNo(parse(r).getOrderNo());
    handle(o);
    flowMapper.insert(buildFlow(o));
}

// ✅ 改造后：一次批量查、内存做映射、一次批量写
List<OrderPaidEvent> events = records.stream().map(this::parse).collect(toList());

List<String> nos = events.stream()
        .map(OrderPaidEvent::getOrderNo).distinct().collect(toList());

Map<String, Order> orderMap = orderMapper.selectByNos(nos).stream()
        .collect(toMap(Order::getOrderNo, identity()));

List<Flow> flows = events.stream()
        .map(e -> buildFlow(e, orderMap.get(e.getOrderNo())))
        .collect(toList());

flowMapper.batchInsertIgnore(flows);   // 一次批量写入

```

同类改造：`IN` 批量查询替代循环单查、`batch insert` 替代逐条 insert、Redis `pipeline` / `mget` 替代逐个 get、下游 HTTP 接口改批量接口。

### 5.3 修下游

- 给所有外部调用**加超时**（连接超时 + 读超时），这是永久性的改进，不只是应急。
- 数据库：加索引、拆大事务、临时扩容只读副本、把非核心查询降级。
- 能缓存的缓存，能异步的异步（比如风控、埋点、通知这类不影响主链路的调用）。

### 5.4 临时调参

不改代码、不扩容就能立刻生效的手段：

```properties
# 减少网络往返，提升单次拉取量（注意 OOM 风险）
fetch.min.bytes=1048576          # 1MB，攒够再返回
fetch.max.bytes=5242880          # 5MB
max.poll.records=2000            # 单次 poll 最多 2000 条

# 如果是 Rebalance 引起的，方向要反过来：调小批次 + 放宽超时
max.poll.records=50
max.poll.interval.ms=600000      # 10 分钟

```

**注意这两组配置是矛盾的**：前者追求吞吐，后者追求稳定。**如果当前的堆积是 Rebalance 引起的，用第一组配置会火上浇油。** 先确认根因再选方向。

### 5.5 生产端限流 / 熔断 / 降级

如果消费端已经榨干还是追不上，就得从进水口想办法：

- 对非核心业务的生产者限流或熔断
- 把生产者的消息临时转发到其他 Topic 分流
- 降级非核心链路（比如暂停实时推荐特征计算，保住订单主链路）

**这是有业务代价的动作，必须和业务方确认后再执行。**

### 5.6 搬运工模式（分区数不够时的杀手锏）

**适用场景**：分区数已经成了硬瓶颈（比如老 Topic 只有 8 个分区，但需要 64 路并行），又不能随便扩分区（怕破坏顺序性、怕影响其他消费组）。

**做法**：写一个**只转发、不做任何业务处理**的临时消费者，把老 Topic 的消息原样搬到一个分区数大得多的新 Topic，然后在新 Topic 上起大量消费者并行处理。

```
老 Topic（8 分区，堆积 500 万条）
        │
        ▼
  临时转发消费者（逻辑只有一行：produce 到新 topic）
  单机吞吐可达数万条/秒，因为它不做任何业务
        │
        ▼
新 Topic（128 分区）
        │
        ▼
  128 个消费者并行处理业务  ──►  下游

```

为什么有效：转发消费者的处理逻辑几乎为零，8 个分区的读取速度足以喂饱下游；而真正的业务处理被摊到了 128 路上。相当于**临时把消费资源扩大了 16 倍**。

**代价与注意事项：**

- **顺序性会被破坏**：老 Topic 里同 key 的消息是有序的，转发到新 Topic 后如果分区数变了，同 key 可能落到不同分区，被不同消费者并行处理。对顺序敏感的业务（如账户余额变更）**不能用这招**，或者必须保证转发时按 key 哈希到新 Topic 的固定分区、且新 Topic 内部也单线程处理同 key。
- 需要临时资源：新 Topic 的磁盘空间、大量临时消费者实例。
- 处理完要拆掉，别留成长期架构。
- 转发消费者本身要做好幂等和位点管理，别在搬运环节丢消息。

### 5.7 跳过非关键消息（有损，谨慎）

**仅适用于日志、埋点、监控这类可丢数据。订单、支付、账务类严禁使用。**

```bash
# ⚠️ 执行前必须先停掉该消费组的所有实例，否则命令会拒绝执行
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --group log-consumer-group --topic app.log \
  --reset-offsets --to-latest --execute

# 或者按时间点重置（跳过某个时间段之前的堆积）
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --group log-consumer-group --topic app.log \
  --reset-offsets --to-datetime 2026-11-14T15:00:00.000 --execute

```

其他可选：`--to-earliest`（从头，会造成海量重复）、`--shift-by N`（相对偏移）、`--to-offset N`（指定位点）、`--by-duration PT30M`（按时长）。

建议先加 `--dry-run` 预览效果，确认无误再换 `--execute`。

### 5.8 临时扩分区

```bash
kafka-topics.sh --bootstrap-server kafka1:9092 \
  --alter --topic order.paid --partitions 16

```

**这不是应急首选，因为它有三个不可逆或有风险的特性：**

1. **只能加，不能减。** 加上去就永久生效，长期看可能带来 broker 分区密度过高、元数据膨胀的问题。
2. **会破坏按 key 的分区路由。** Kafka 默认用 `hash(key) % 分区数` 决定消息进哪个分区。分区数从 8 变成 16 之后，**同一个 key 的新消息会落到和老消息不同的分区**，于是同 key 的消息在两个分区里各自有序、合起来无序。对顺序敏感的业务这是致命的。
3. **扩容本身会触发 Rebalance**，在已经堆积的情况下雪上加霜；而且已有的消费者组要等 Rebalance 之后才会接手新分区。

所以：**分区数应该在容量规划阶段就想清楚，而不是等堆积了再临时加。**

### 5.9 算一下能不能追平（这一步别跳过）

```
追平时间（秒） = 当前堆积量 / (消费速率 − 生产速率)

```

**前提：分母必须为正。** 如果消费速率 ≤ 生产速率，堆积永远不会减少，任何"等着就好"的想法都是错的。

示例（数值为假设，仅演示算法）：当前堆积 500 万条，扩容后消费速率 3000 条/秒，生产速率 1000 条/秒，那么净消化速率 2000 条/秒，追平需要 2500 秒 ≈ 42 分钟。

**然后必须做这个校验：**

```
追平时间  vs  消息保留时长（retention.ms）

```

**如果追平时间超过消息保留时长，那些还没被消费的消息会在被消费之前就被 Kafka 删掉——这是真真切切的数据丢失。** 这时候要立刻改变优先级：

1. 先临时调大 Topic 的 `retention.ms`，同时确认 broker 磁盘容量扛得住
2. 或者立刻用 5.6 的搬运工模式，把堆积的消息复制到一个独立的、保留期更长的 Topic 里"存起来"，先保住数据，再慢慢处理
3. 如果磁盘也不够，考虑分层存储（tiered storage）或临时扩盘

**另一个必须注意的点：追平过程要限速。** 追平时消费速率通常是正常值的数倍，如果不限速，下游数据库会被瞬间打挂，然后消费速率掉下来，堆积反而更严重。做法是在消费端加限流器，把追平速率控制在下游能承受的范围内（比如下游容量的 70%）。

### 5.10 应急决策树

```
收到堆积告警
    │
    ▼
① 跑 kafka-consumer-groups.sh --describe（间隔 1 分钟跑两次）
    │
    ├── CONSUMER-ID 全为空 ─────────────► 消费组没起来，查部署/进程/配置，先恢复消费
    │
    ├── 位点完全不动 + 有成员 ──────────► jstack 抓栈：卡死 or 毒消息
    │                                      ├─ 卡在外部调用 → 加超时、修下游
    │                                      └─ 反复消费同一条 → 毒消息，转死信队列后跳过
    │
    ├── 日志有频繁 Rebalance ───────────► 调小 max.poll.records + 调大 max.poll.interval.ms
    │                                      查 Full GC / Pod 重启 / 单批处理耗时
    │
    ├── 个别分区 lag 巨大 ──────────────► 热点 key 倾斜，改 key 设计（扩容无效）
    │
    └── 所有分区均匀上涨 ───────────────► 吞吐不足
                                           ├─ 实例数 < 分区数？ → 加消费者（最快）
                                           ├─ 逐条 IO？ → 批量化改造（收益最大）
                                           ├─ 实例数已 = 分区数？ → 搬运工模式（5.6）
                                           └─ 都不行 → 生产端限流 + 临时扩分区（评估顺序性）
    │
    ▼
② 算追平时间，校验是否超过 retention → 超过则优先保数据
    │
    ▼
③ 追平过程限速，盯住下游负载

```

## 六、根治与参数调优

### 6.1 三个超时参数：这是所有 Rebalance 问题的核心

| 参数                    | 默认值                               | 作用                                       | 违反后果                                          |
| ----------------------- | ------------------------------------ | ------------------------------------------ | ------------------------------------------------- |
| `heartbeat.interval.ms` | 3000（3 秒）                         | 心跳线程向 Coordinator 发送心跳的间隔      | 太大 → 来不及在 session 超时前发出心跳            |
| `session.timeout.ms`    | 45000（较新版本）/ 10000（早期版本） | Coordinator 多久收不到心跳就判定消费者死亡 | 太小 → GC 或网络抖动就误判；太大 → 真故障发现太慢 |
| `max.poll.interval.ms`  | 300000（5 分钟）                     | 两次 `poll()` 之间允许的最大间隔           | 单批处理超时 → 被踢出组 → Rebalance               |
| `max.poll.records`      | 500                                  | 单次 `poll()` 返回的最大消息条数           | 太大 → 单批处理时间过长                           |

> **关于 `session.timeout.ms` 的默认值**：不同资料说法不一（有说 2.3 起改为 45 秒，Confluent 官方明确是 3.0.0 起从 10 秒提升到 45 秒）。**以你实际使用的客户端版本的官方文档为准**，不要照抄本文或博客里的数字。

**两条必须满足的约束：**

```
① heartbeat.interval.ms  ≤  session.timeout.ms / 3
   （保证在会话超时前至少能发出 3 次心跳，容忍单次丢失）

② max.poll.interval.ms  >  max.poll.records / (单线程每秒处理条数 × 消费线程数)
   （保证一批消息能在超时前处理完并回到下一次 poll）

```

反过来，`max.poll.records` 应该**远小于**：

```
max.poll.records  <<  单线程每秒处理条数 × 消费线程数 × max.poll.interval.ms(秒)

```

**数值算例（假设值，用于演示算法）：**

| 场景   | 单线程吞吐 | 线程数 | `max.poll.records` | 处理一批耗时 | `max.poll.interval.ms` | 结论                           |
| ------ | ---------- | ------ | ------------------ | ------------ | ---------------------- | ------------------------------ |
| A      | 100 条/秒  | 2      | 500                | 2.5 秒       | 300 秒                 | 安全，余量 120 倍              |
| B      | 1 条/秒    | 1      | 500                | 500 秒       | 300 秒                 | **必然被踢**，Rebalance 死循环 |
| B 修正 | 1 条/秒    | 1      | **50**             | 50 秒        | 300 秒                 | 安全                           |

场景 B 就是线上最典型的事故：某条链路单条消息要调一次耗时的外部接口，平时 200 ms，某天对端变慢到 1 秒，500 条一批就要 500 秒，直接超过 5 分钟死线。**平时不报错，下游一抖就雪崩。**

**调参原则（重要）：优先调小 `max.poll.records`，而不是一味调大 `max.poll.interval.ms`。**

理由：`max.poll.interval.ms` 是服务端发现"真正故障的消费者"的兜底机制。如果你把它设成 30 分钟，那么一个真的死锁卡住的消费者，要 30 分钟后才会被发现并接管它持有的分区——这 30 分钟里那些分区完全无人消费。调小批次则没有这个副作用，还能让位点提交更频繁，减少重复消费的量。

**推荐配置模板：**

```properties
# 通用稳妥配置
max.poll.records=200              # 按实际单条耗时反推，宁可小一点
max.poll.interval.ms=300000       # 保持默认 5 分钟
session.timeout.ms=45000
heartbeat.interval.ms=10000       # ≤ session.timeout.ms / 3
enable.auto.commit=false          # 改手动提交

```

Spring Boot 写法：

```yaml
spring:
  kafka:
    consumer:
      max-poll-records: 200
      enable-auto-commit: false
      properties:
        max.poll.interval.ms: 300000
        session.timeout.ms: 45000
        heartbeat.interval.ms: 10000

```

### 6.2 拉取吞吐参数

| 参数                        | 默认值           | 作用                                  | 调优方向                                             |
| --------------------------- | ---------------- | ------------------------------------- | ---------------------------------------------------- |
| `fetch.min.bytes`           | 1                | broker 攒够这么多字节才返回           | 调大（如 1MB）减少请求次数、提升吞吐，代价是增加延迟 |
| `fetch.max.wait.ms`         | 500              | 攒不够 `fetch.min.bytes` 时最多等多久 | 与上一个配合，决定最坏延迟                           |
| `fetch.max.bytes`           | 52428800（50MB） | 单次 fetch 返回的最大字节数           | 调大提升吞吐，注意内存                               |
| `max.partition.fetch.bytes` | 1048576（1MB）   | 单分区单次 fetch 的最大字节数         | **必须 ≥ 单条消息最大长度**，否则大消息永远拉不下来  |

**内存估算：**

```
消费者堆内存占用 ≈ fetch.max.bytes × 消费线程数（并发拉取的分区数）

```

`fetch.max.bytes` 设 50MB、单进程 8 个消费线程，理论峰值就是 400MB 的消息数据在堆里。堆给小了就是 Full GC，再小就是 OOM。**这是"调大拉取参数反而导致堆积更严重"的常见原因**——GC 停顿引发 Rebalance。

`max.partition.fetch.bytes` 有个容易踩的坑：如果生产端发了 2MB 的大消息，而这个参数是默认的 1MB，消费者**永远拉不到这条消息**，表现为该分区 lag 卡在一个固定值不动。broker 端对单条超过限制的消息有特殊处理（会返回它以便消费者能前进），但配置不匹配仍会造成各种问题，最好的做法是从生产端就控制住消息大小。

### 6.3 位点提交与幂等

**核心原则：先处理消息，再提交位点。** 顺序反了就会丢消息。

```java
// ❌ 错误：先提交再处理，中间崩溃就丢消息
consumer.commitSync();
process(records);

// ✅ 正确：处理完这批再提交
while (true) {
    ConsumerRecords<String, String> records = consumer.poll(Duration.ofMillis(1000));
    for (ConsumerRecord<String, String> record : records) {
        processMessage(record);
    }
    consumer.commitSync();     // 整批处理成功后提交
}

```

**提交粒度的三种选择：**

| 方式                | 吞吐                   | 重复消费量                         | 适用                           |
| ------------------- | ---------------------- | ---------------------------------- | ------------------------------ |
| 整批 `commitSync()` | 高                     | 一批（最多 `max.poll.records` 条） | 绝大多数场景                   |
| 逐条提交            | 低（每条一次网络往返） | 单条                               | 单条处理极重、不允许重复的场景 |
| 按分区提交          | 中高                   | 单分区一批                         | 多线程按分区隔离时             |

**异步多线程消费的三个坑（想清楚再上）：**

把 `poll()` 到的消息丢进线程池并行处理，确实能让主线程快速回到下一次 `poll()`，避免 `max.poll.interval.ms` 超时。但它引入了新问题：

1. **自动提交会丢消息**：主线程下一次 `poll()` 触发自动提交，但线程池里的任务还没跑完。此时进程崩溃，这批消息永久丢失。**所以用线程池就必须关掉自动提交。**
2. **子线程提交会乱序覆盖**：线程池里的任务是并发乱序完成的，如果每个子线程完成后各自提交自己的 offset，后完成的小 offset 可能覆盖先完成的大 offset，导致大量重复消费。
3. **背压缺失**：如果线程池用了无界队列，`poll()` 一直很快，消息在内存里无限堆积，最后 OOM。**要用有界队列 + `CallerRunsPolicy`**，让队列满时反压到 poll 线程。

正确的做法是**按分区隔离**：每个分区绑定一个独立的内存队列和单线程处理器，保证分区内顺序；位点提交采用"滑动窗口"——只提交连续处理完成的最小 offset。实现成本不低，非必要不上。

**幂等兜底（推荐，无论如何都要做）：**

不管位点提交得多及时，网络分区、进程被 kill、Rebalance 这些极端场景下，重复消费躲不掉。**Kafka 的 offset 机制天生只保证 at-least-once。** 所以关键业务必须在消费端做幂等：

```java
public void onMessage(ConsumerRecord<String, String> record) {
    String bizId = extractBizId(record.value());   // 业务唯一 ID：订单号、流水号等

    // 用唯一业务 ID 判断是否已处理（Redis setnx 或 DB 唯一索引）
    Boolean first = redis.opsForValue()
            .setIfAbsent("consumed:" + bizId, "1", Duration.ofHours(24));

    if (Boolean.TRUE.equals(first)) {
        doBusinessLogic(record);
    } else {
        log.warn("重复消费，跳过: bizId={}", bizId);
    }
}

```

数据库侧则用唯一索引 + `INSERT IGNORE` / `ON DUPLICATE KEY UPDATE` 兜底，比 Redis 更可靠（不会因为缓存过期而失效）。

### 6.4 降低 Rebalance 的影响

**① 换成 `CooperativeStickyAssignor`（增量式重平衡）**

传统 eager 协议下，Rebalance 会先撤销所有分区再重新分配，整组停摆。协作式（cooperative）协议只迁移必要的分区，其余分区继续消费，中断时间大幅缩短。

**切换必须分两步滚动进行**（直接改会导致组内协议不一致而无法工作）：

```properties
# 第一步：两个 assignor 都配上，当前的放前面。滚动重启所有成员。
# Coordinator 会继续选第一个共同支持的 assignor，行为不变。
partition.assignment.strategy=org.apache.kafka.clients.consumer.RangeAssignor,org.apache.kafka.clients.consumer.CooperativeStickyAssignor

# 第二步：确认所有成员都已升级并重启后，把 cooperative 放到第一位（或只留它），再滚动重启一次。
partition.assignment.strategy=org.apache.kafka.clients.consumer.CooperativeStickyAssignor

```

**② 静态成员（`group.instance.id`）**

Kafka 2.3+ 支持。给每个消费者实例配一个固定的 `group.instance.id`，实例重启时（在 `session.timeout.ms` 内重新加入）Coordinator 认得它，**直接恢复原有的分区分配，不触发 Rebalance**。

这对 K8s 环境特别有用——Pod 重启、滚动发布是家常便饭，每次都触发全组 Rebalance 的代价太大。注意 `session.timeout.ms` 需要设得比 Pod 重启耗时长。

**③ 启动风暴**

批量重启消费者时，实例逐个加入会触发一连串 Rebalance。调大 broker 端的 `group.initial.rebalance.delay.ms`，让 Coordinator 等一等再开始分配，可以把多次 Rebalance 合并成一次。

**④ 订阅关系收敛**

一个消费组订阅的 Topic 越多，Rebalance 涉及的范围越大、耗时越长。建议**一个 Group 只订阅一个 Topic**，最多不超过 5 个。

**⑤ K8s 优雅停机**

- liveness 探针别太激进，避免因为一次 GC 停顿就 kill Pod
- 停机时先停止拉取、处理完手头这批、提交位点、调用 `consumer.close()`（会主动离组，触发一次可控的 Realance），再退出进程
- `terminationGracePeriodSeconds` 要给足时间

### 6.5 分区数与 key 设计

**分区数怎么定：**

```
目标分区数 ≈ 峰值总吞吐 / 单分区可达吞吐  ×  (1 + 冗余系数)

```

单分区吞吐要实测，别猜——它受消息大小、broker 磁盘、副本数、消费端处理逻辑共同影响。

两个方向的代价都要看：

| 分区过少                 | 分区过多                                 |
| ------------------------ | ---------------------------------------- |
| 消费并行度天花板低       | Rebalance 变慢（要分配的分区多）         |
| 单分区吞吐受限于单机     | broker 文件句柄、内存、元数据开销上升    |
| 扩容困难（只能加不能减） | 端到端延迟上升（生产端要等更多副本确认） |
|                          | 分区密度过高导致磁盘吞吐下降             |

云厂商的通用建议：分区数不建议低于 12，单 Topic 不建议超过 100（超过后 Rebalance 明显变慢）；单 broker 承载的分区总数（含副本）也有上限，需要按集群规格核算。

**热点 key 的四种处理：**

| 方案           | 做法                                                | 代价                                      |
| -------------- | --------------------------------------------------- | ----------------------------------------- |
| 加随机后缀     | key 改成 `商户号_随机两位`，把一个热点打散成 100 份 | **丧失同 key 顺序性**，顺序敏感业务禁用   |
| 二级 key       | 用 `商户号 + 订单号后两位` 做 key                   | 同上，但粒度可控                          |
| 热点单独 Topic | 识别出大客户的流量，走独立 Topic + 独立分区         | 需要路由逻辑，运维复杂度上升              |
| 单分区内多线程 | 消费者内部按 key 二次哈希到多个线程                 | 能保证同 key 顺序，但实现复杂、位点提交难 |

**单分区内多线程的正确姿势**（保 key 内顺序）：

```java
// 按 key 哈希到固定的子队列，每个子队列单线程串行处理
int slot = Math.abs(record.key().hashCode()) % QUEUE_COUNT;
queues[slot].submit(record);

```

同一个 key 永远进同一个队列、被同一个线程串行处理，key 内顺序得以保持；不同 key 之间并行。位点提交需要等所有队列里 offset 更小的消息都处理完，实现上要小心。

### 6.6 服务端侧

**① retention 与磁盘容量规划**

**消息保留时长必须大于"最大可接受的故障恢复时间"。** 如果保留 24 小时，而消费端故障可能需要 6 小时才能修复、再 4 小时才能追平，那就只剩 14 小时的余量——一旦遇上周末或大促，就会丢数据。

```bash
# 查看与调整 Topic 保留策略
kafka-configs.sh --bootstrap-server kafka1:9092 \
  --entity-type topics --entity-name order.paid --describe

kafka-configs.sh --bootstrap-server kafka1:9092 \
  --entity-type topics --entity-name order.paid --alter \
  --add-config retention.ms=604800000,segment.ms=1800000

```

**② 理解 Kafka 的删除粒度（很多人误解这一点）**

Kafka **删除的是 segment（日志段），不是单条消息**。规则是：

- 只有**已关闭**的 segment 才可能被删除，**正在写入的活跃 segment 永远不删**
- 判定过期用的是**该 segment 里最新一条消息的时间戳**
- 清理线程按 `log.retention.check.interval.ms`（默认 5 分钟）周期扫描

所以实际保留时间可能比配置的长很多：

```
实际保留时长 ≈ retention.ms + segment 滚动时间(最长 segment.ms，默认 7 天) + 检查间隔(5 分钟) + 删除延迟(1 分钟)

```

**低吞吐 Topic 尤其要注意**：如果流量小到 7 天都填不满一个 1GB 的 segment，那这个 segment 一直不滚动，里面的消息永远不会被删除——即使你配了 `retention.ms=1小时`。解法是同时配置 `segment.ms`（比如 30 分钟）强制滚动。

**③ 时间戳类型**

```properties
message.timestamp.type=LogAppendTime   # 用 broker 时间，而不是客户端时间

```

默认的 `CreateTime` 用生产者本地时间。**如果生产者时钟错乱**（比如设成了未来某个时间），会导致：

- 这条消息所在的整个 segment 都不会被老化删除 → 磁盘异常增长、消息异常堆积
- 或者反过来，消息被提前判定过期删除 → **还没消费就丢了**

用 `LogAppendTime` 可以从根上避免这类问题。

**④ 磁盘水位告警**

磁盘使用率 80% 就该告警。很多云厂商的 Kafka 有"磁盘清理水位"策略：容量达到阈值后会**自动删除旧消息**（哪怕没到保留期），或者**暂停写入**。前者意味着静默丢数据，后者意味着生产端报错。别等磁盘满了才知道。

## 七、堆积的次生风险（别只盯着 lag 数字）

堆积本身只是"慢"，但它会引发一系列更严重的后果：

1. **数据被 retention 删除 → 真丢数据。** 这是最严重的。消费速度长期跟不上 + 保留时间偏短，消息会在被消费之前就过期删除。**而且这个过程是静默的，没有任何报错。** 唯一的防护是监控 lag 时间维度，并保证保留时长有足够余量。
2. **磁盘打满 → broker 不可用。** 堆积的消息占满磁盘，影响的是这台 broker 上**所有** Topic，故障范围从"一个消费组慢"扩大到"整个集群写入失败"。
3. **冷读拖垮 broker。** 消费者从很久以前的位点开始追，page cache 失效，磁盘随机读飙升，连带影响同 broker 上的其他业务。
4. **追平瞬间打挂下游。** 追平时消费速率是平时的数倍，数据库、缓存、外部接口可能扛不住，反而引发新一轮故障。**必须限速追平。**
5. **业务时效性失效，导致错误决策。** 风控拿到的是 10 分钟前的数据、实时大屏显示的是历史状态、推荐系统基于过期特征打分。**错误的实时数据比没有数据更危险**，因为它会让人做出错误的判断。
6. **位点过期后海量重复。** 消费组停用超过 `offsets.retention.minutes`，位点被清，重启后按 `auto.offset.reset=earliest` 从头消费，瞬间产生海量重复流量，可能直接压垮下游。

## 八、预防体系

### 8.1 监控与告警

**告警阈值按"分钟消费量"设，而不是绝对条数。**

```
警告级：lag > 5 分钟的平均消费量
严重级：lag > 30 分钟的平均消费量  →  升级 P1

```

这样一个 10000 TPS 的 Topic 和一个 10 TPS 的 Topic 都能用同一套逻辑，且阈值天然对应业务能感知的延迟。

**双指标监控：**

| 指标         | 计算方式                              | 意义                                 |
| ------------ | ------------------------------------- | ------------------------------------ |
| lag 条数     | `LOG-END-OFFSET − CURRENT-OFFSET`     | 还欠多少活                           |
| **lag 时间** | 当前时间 − 正在处理的消息的生产时间戳 | **业务数据有多旧（更贴近业务影响）** |

**客户端 JMX 指标清单：**

| 指标                                 | 含义                                       | 异常表现                                       |
| ------------------------------------ | ------------------------------------------ | ---------------------------------------------- |
| `records-lag-max`                    | 所有分区中最大的 lag                       | 持续增长                                       |
| `records-lead-min`                   | 最小的"领先量"（位点与最早可用消息的距离） | 接近 0 说明快要读到被删除的边界了              |
| `records-consumed-rate`              | 消费速率（条/秒）                          | 突降说明出问题                                 |
| `records-per-request-avg`            | 每次 fetch 平均条数                        | 长期等于 `max.poll.records` 说明一直在满负荷追 |
| `fetch-latency-avg`                  | fetch 请求延迟                             | 升高说明 broker 或网络有问题                   |
| `commit-rate` / `commit-latency-avg` | 位点提交频率与延迟                         | 提交失败率上升是 Rebalance 前兆                |
| `poll-idle-ratio`                    | poll 线程空闲比例                          | 接近 0 说明主线程一直在忙，快超时了            |

`records-lead-min` 特别值得说：它表示消费者当前位置距离"最早还保留着的消息"有多远。**如果这个值接近 0，说明消费者快要读到 retention 边界了，再慢一点数据就要被删了**——这是数据丢失的直接前兆指标。

### 8.2 容量规划与压测

- 定期用 `kafka-producer-perf-test.sh` 和 `kafka-consumer-perf-test.sh` 摸清单分区、单消费者的实际吞吐上限
- 按业务增长预留分区数，**在还有余量时就扩，不要等堆积了才扩**（因为扩分区有顺序性风险）
- 消费端的容量规划要按"峰值生产速率 × 1.5"来算，而不是平均值

```bash
# 生产压测
kafka-producer-perf-test.sh --topic perf-test --num-records 1000000 \
  --record-size 1024 --throughput 20000 \
  --producer-props bootstrap.servers=kafka1:9092

# 消费压测
kafka-consumer-perf-test.sh --bootstrap-server kafka1:9092 \
  --topic perf-test --messages 1000000 --fetch-size 1048576

```

### 8.3 代码规范红线

把这几条写进团队的编码规范并做 Code Review 检查：

- ✅ 所有外部调用（HTTP、RPC、DB）**必须设超时**，且超时要短（秒级）
- ✅ 消费逻辑**禁止逐条 IO**，一律批量化
- ✅ 消费逻辑里**禁止长事务、禁止重计算**，重活拆到异步链路
- ✅ 处理失败的消息**必须进死信队列**，不能原地无限重试
- ✅ 单条消息处理**必须有超时熔断**（比如超过 30 秒直接失败落死信），不能阻塞整个消费线程
- ✅ 关键业务消费端**必须做幂等**
- ✅ `enable.auto.commit` 生产环境设 `false`

### 8.4 流程与演练

- **变更管控**：分区数、消息 key 规则、消费者核心参数的变更走评审，评估顺序性和容量影响
- **死信队列必须有人看**：配告警，定期清理。死信堆积是"lag 永远不归零"的典型原因
- **对账机制**：每小时比对"生产端发送数"与"业务侧成功处理数"，差值超阈值告警。这是发现静默丢数据的最后一道防线
- **应急演练**：定期做堆积演练——注入慢消费、模拟 Pod 重启、验证告警是否触发、验证 SOP 是否跑得通

## 九、一页纸速查表

| 现象                                      | 最可能的原因                   | 第一动作                                  |
| ----------------------------------------- | ------------------------------ | ----------------------------------------- |
| `CONSUMER-ID` 全为空                      | 消费组没启动 / 全部实例挂了    | 查部署、进程、配置、鉴权                  |
| 位点长时间不动，进程活着                  | 消费线程卡死（外部调用无超时） | `jstack` 抓线程栈                         |
| 位点不动，日志反复报同一条消息异常        | 毒消息                         | 转死信队列，跳过该条                      |
| 个别分区 lag 巨大，其余正常               | 热点 key 数据倾斜              | 改 key 设计（加消费者无效）               |
| 所有分区 lag 均匀线性上涨                 | 整体消费能力不足               | 先看实例数 vs 分区数，再批量化改造        |
| lag 突发跳变，日志有 `(Re-)joining group` | Rebalance                      | 调小 `max.poll.records`，查 GC / Pod 重启 |
| 日志频繁 `CommitFailedException`          | poll 间隔超时被踢出组          | 同上，重点查单批处理耗时                  |
| lag 长期在某个基数不归零                  | 死信堆积 / 位点提交逻辑缺陷    | 查死信队列，检查 commit 是否在 finally    |
| lag 涨但消费端 CPU 空闲、无异常           | 长事务卡 LSO / 服务端限流      | 查 `isolation.level` 与开放事务、查带宽   |
| 消费速率突然下降，broker 磁盘 IO 飙升     | 冷读（从很旧的位点开始追）     | 限速追平，错峰，必要时用独立集群重放      |
| `records-lead-min` 接近 0                 | 即将触及 retention 边界        | **立刻**扩 `retention.ms` 或搬运落盘      |

## 十、常见误区

**误区 1：消费者越多越好。**
错。上限是分区数，超出的实例完全空跑，还增加 Rebalance 的规模和耗时、加重 Coordinator 负担。合理做法是消费者数 = 分区数，或留 10%–20% 冗余应对实例下线。

**误区 2：堆积了就重启消费者。**
错，而且往往让情况更糟。重启触发 Rebalance（整组停消费）、位点回退（重复消费）、可能引发冷读（拖垮 broker）。

**误区 3：调大 `max.poll.interval.ms` 就能解决 Rebalance。**
治标不治本，还有副作用。真正的故障消费者要更久才被发现，期间它持有的分区无人消费。优先调小 `max.poll.records`。

**误区 4：lag 为 0 就没问题。**
不一定。lag 为 0 但存在大量重复消费（位点提交了、业务失败重来了），或者 lag 是用 committed offset 算的、业务其实还没处理完，都会掩盖真实问题。要同时看重复率、处理成功率、lag 时间。

**误区 5：分区可以随时加。**
只能加不能减，而且会改变 `hash(key) % 分区数` 的路由结果，**破坏同 key 消息的顺序性**。这是容量规划阶段就该想清楚的事。

**误区 6：自动提交省心。**
默认的 `enable.auto.commit=true` 会带来两个方向的错误：提交早了丢消息，提交晚了重复消费，且窗口期不可控。生产环境建议关闭自动提交，手动控制提交时机 + 业务幂等兜底。

**误区 7：平均处理耗时正常就说明健康。**
要看 P99 甚至 P999。少量长尾消息就能拖住整批的位点提交，进而触发 `max.poll.interval.ms` 超时。**平均值会骗人，长尾才是杀手。**

**误区 8：堆积只是"慢一点"，不影响正确性。**
错。堆积超过 retention 就是静默丢数据；过期的业务数据被下游使用会得出错误结论；磁盘打满会引发集群级故障。

## 附录 A：常用命令速查

```bash
# ── 消费组与 lag ────────────────────────────────────────────
# 查看所有消费组
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 --list

# 查看某组的分区级 lag、位点、成员分配（最常用）
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --describe --group <group>

# 查看组状态与成员
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 \
  --describe --group <group> --state --members

# ── 位点与速率 ──────────────────────────────────────────────
# 查看各分区最新位点（间隔采样可算生产速率）
kafka-get-offsets.sh --bootstrap-server kafka1:9092 --topic <topic> --time -1
# 旧版本：kafka-run-class.sh kafka.tools.GetOffsetShell --topic <topic> --time -1

# ── 位点重置（先停消费组！建议先 --dry-run）─────────────────
kafka-consumer-groups.sh --bootstrap-server kafka1:9092 --group <group> \
  --topic <topic> --reset-offsets --to-latest --dry-run
# 可选：--to-earliest / --to-offset N / --shift-by N / --to-datetime <ISO8601> / --by-duration PT30M

# ── Topic 与分区 ────────────────────────────────────────────
kafka-topics.sh --bootstrap-server kafka1:9092 --describe --topic <topic>
kafka-topics.sh --bootstrap-server kafka1:9092 --alter --topic <topic> --partitions <N>  # 只能增不能减

# ── 配置查看与修改 ──────────────────────────────────────────
kafka-configs.sh --bootstrap-server kafka1:9092 \
  --entity-type topics --entity-name <topic> --describe
kafka-configs.sh --bootstrap-server kafka1:9092 \
  --entity-type topics --entity-name <topic> --alter \
  --add-config retention.ms=604800000,segment.ms=1800000,message.timestamp.type=LogAppendTime

# ── 抓取消息样本（排查毒消息 / 数据倾斜）────────────────────
kafka-console-consumer.sh --bootstrap-server kafka1:9092 --topic <topic> \
  --partition <P> --offset <N> --max-messages 10 --property print.key=true

# ── 性能压测 ────────────────────────────────────────────────
kafka-producer-perf-test.sh --topic <topic> --num-records 1000000 \
  --record-size 1024 --throughput 20000 --producer-props bootstrap.servers=kafka1:9092
kafka-consumer-perf-test.sh --bootstrap-server kafka1:9092 \
  --topic <topic> --messages 1000000 --fetch-size 1048576

# ── 客户端诊断 ──────────────────────────────────────────────
jstack <pid> > /tmp/jstack.txt        # 抓线程栈，定位卡死点
jstat -gcutil <pid> 1000 20           # 观察 GC 情况

```

## 附录 B：参数默认值速查表

> 默认值随 Kafka 客户端版本变化，**生产环境请以你所用版本的官方文档为准**。下表为常见版本的典型值。

**消费者端：**

| 参数                            | 典型默认值                       | 与堆积的关系                             |
| ------------------------------- | -------------------------------- | ---------------------------------------- |
| `max.poll.records`              | 500                              | 单批条数，直接决定单批处理耗时           |
| `max.poll.interval.ms`          | 300000（5 分钟）                 | 超时被踢出组 → Rebalance                 |
| `session.timeout.ms`            | 45000（较新版本）/ 10000（早期） | 心跳超时 → 被判定死亡                    |
| `heartbeat.interval.ms`         | 3000                             | 应 ≤ `session.timeout.ms` / 3            |
| `enable.auto.commit`            | true                             | 生产建议改 false                         |
| `auto.commit.interval.ms`       | 5000                             | 间隔越长，lag 常态基线越高               |
| `fetch.min.bytes`               | 1                                | 调大提升吞吐、增加延迟                   |
| `fetch.max.wait.ms`             | 500                              | 与上一个配合                             |
| `fetch.max.bytes`               | 52428800（50MB）                 | 影响堆内存占用                           |
| `max.partition.fetch.bytes`     | 1048576（1MB）                   | 必须 ≥ 最大单条消息                      |
| `auto.offset.reset`             | latest                           | 位点失效时的起点，`earliest` 会海量重复  |
| `isolation.level`               | read_uncommitted                 | `read_committed` 受长事务 LSO 影响       |
| `partition.assignment.strategy` | Range 系                         | 建议切 `CooperativeStickyAssignor`       |
| `group.instance.id`             | null                             | 配置后成为静态成员，重启不触发 Rebalance |

**Broker 端：**

| 参数                               | 典型默认值        | 与堆积的关系                            |
| ---------------------------------- | ----------------- | --------------------------------------- |
| `log.retention.ms` / `.hours`      | 7 天              | 小于追平时间就会丢数据                  |
| `log.retention.bytes`              | -1（不限）        | 与时间策略同时配时，先触发的生效        |
| `log.segment.bytes`                | 1073741824（1GB） | segment 不滚动就不会被删除              |
| `log.retention.check.interval.ms`  | 300000（5 分钟）  | 清理线程扫描周期                        |
| `offsets.retention.minutes`        | 10080（7 天）     | 消费组停用超期后位点丢失                |
| `group.initial.rebalance.delay.ms` | 视版本            | 调大可合并启动期的多次 Rebalance        |
| `message.timestamp.type`           | CreateTime        | 建议改 LogAppendTime 避免客户端时钟问题 |

## 一句话总结

**Kafka 消息堆积的诊断，本质上就是回答三个问题：有没有在消费（位点是否前进）、消费得够不够快（速率差）、并行度有没有用满（实例数 vs 分区数）。** 应急时优先加实例和批量化改造，分区数成瓶颈时用搬运工模式绕过；根治靠三个超时参数的合理配比、消费逻辑的批量化与幂等、以及按"分钟消费量"设置的 lag 告警。最容易被忽略但后果最严重的，是**堆积时间超过消息保留时长导致的静默数据丢失**——这一条，值得为它单独配一个 `records-lead-min` 告警。

## 参考文献

1. Apache Kafka 官方文档 — Consumer Configs. https://kafka.apache.org/documentation/#consumerconfigs
2. Apache Kafka 官方文档 — Consumer Metrics（`records-lag-max`、`records-lead-min`）. https://kafka.apache.org/30/generated/consumer_metrics.html
3. Confluent Documentation — Monitor Consumer Lag in Confluent Platform. https://docs.confluent.io/platform/current/monitor/monitor-consumer-lag.html
4. Confluent — What's New in Apache Kafka 3.0.0（`session.timeout.ms` 默认值变更）. https://www.confluent.io/blog/apache-kafka-3-0-major-improvements-and-new-features/
5. Confluent Documentation — Kafka Consumer for Confluent Platform（消费组规模、`CooperativeStickyAssignor` 切换、静态成员）. https://docs.confluent.io/platform/current/clients/consumer.html
6. Confluent — Kafka Rebalancing Explained. https://www.confluent.io/learn/kafka-rebalancing/
7. IBM — Kafka 应用程序需避免的五个可扩展性陷阱. https://www.ibm.com/cn-zh/think/topics/kafka-scalability
8. 阿里云 — Group 出现消息堆积怎么办. https://help.aliyun.com/zh/apsaramq-for-kafka/cloud-message-queue-for-kafka/support/what-do-i-do-if-messages-are-accumulated-when-i-use-apsaramq-for-kafka
9. 阿里云 — Kafka 订阅者最佳实践. https://help.aliyun.com/zh/apsaramq-for-kafka/cloud-message-queue-for-kafka/support/best-practices-for-consumers
10. 阿里云 — 为什么消费客户端频繁出现 Rebalance. https://help.aliyun.com/document_detail/154454.html
11. 华为云 — Kafka 消息堆积了怎么办. https://support.huaweicloud.com/kafka_faq/kafka-faq-200426100.html
12. 华为云 — 为什么 Kafka 实例常常发生 Rebalance，导致消息拉取失败. https://support.huaweicloud.com/kafka_faq/kafka-faq-200426036.html
13. 天翼云 — 消息堆积最佳实践. https://www.ctyun.cn/document/10029624/10460501
14. 腾讯云 CKafka — 消费速度缓慢. https://cloud.tencent.com/document/faq/597/60501
15. 腾讯云开发者社区 — Kafka 消息积压、重复、丢失，根源基本都是 Rebalance. https://cloud.tencent.cn/developer/article/2577624
16. 袋鼠云 — Kafka 消费者 Lag 处理：诊断、消除与预防实战. https://www.dtstack.com/zh-cn/blogs/kafka-consumer-lag-management
17. 袋鼠云 — Kafka Consumer Lag 排查：从积压到恢复. https://www.dtstack.com/zh-cn/blogs/kafka-consumer-lag-troubleshooting
18. Conduktor — Kafka Retention: When Messages Disappear and Why. https://www.conduktor.io/blog/kafka-retention-policies-when-messages-disappear
19. Microsoft Learn — Performance optimization for Apache Kafka HDInsight clusters. https://learn.microsoft.com/en-us/azure/hdinsight/kafka/apache-kafka-performance-tuning
20. Red Hat Documentation — Kafka 配置调整（选择正确的分区分配策略、恢复失败以避免数据丢失）. https://docs.redhat.com/zh-cn/documentation/red_hat_streams_for_apache_kafka/2.7/html/kafka_configuration_tuning/

------