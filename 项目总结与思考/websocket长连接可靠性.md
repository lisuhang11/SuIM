# IM WebSocket 长连接实战指南：心跳、断线重连与状态恢复

做 IM，绕不开长连接。而长连接做得好不好，用户是能直接感知到的：消息转圈发不出去、对方明明在线却显示离线、切个 WiFi 回来聊天就"死"了必须刷新页面——这些体验问题的根源，几乎都藏在长连接的三个基本问题里：

1. **怎么知道连接断了？**
2. **断了怎么重连？**
3. **重连后状态怎么恢复？**

这三个问题看起来朴素，但每一个都有大量工程细节。本文把这三个问题连同背后的原理（为什么要双向心跳、为什么要指数退避）以及生产环境的优化手段（后台节流、断网暂停、token 过期处理）一次讲透。

------

## 一、背景：长连接的真正敌人

### 1.1 为什么 IM 要用 WebSocket

HTTP 是请求-响应模型，服务端没法主动推消息给客户端。IM 的核心诉求恰恰是"服务端随时推"：别人给你发消息、有人输入中、被踢下线、多端同步……早期方案是轮询（每秒发一次 HTTP 请求问"有没有新消息"）和长轮询（服务端 hold 住请求直到有消息或超时），但两者都有明显缺陷：轮询浪费且延迟高，长轮询每次响应后都要重建请求，头部开销大、实时性也受限。

WebSocket 在一次 HTTP 握手（`Upgrade: websocket`）之后，把连接升级为全双工通道，双方可以随时互发数据帧，开销小、延迟低，天然适合 IM。RFC 6455 还定义了两种控制帧：**Ping（操作码 0x9）** 和 **Pong（操作码 0xA）**，这就是协议层给心跳准备的"原生工具"。

### 1.2 长连接不是"连上就万事大吉"

很多人以为 WebSocket 建连成功后就稳了，这是最大的误区。真实网络环境里，长连接面对的是三类敌人：

**第一类：半开连接（half-open）。** 对端进程崩了、手机突然掉电、拔了网线、NAT 网关悄悄丢弃了映射表项——这些情况下**没有任何一方发出 FIN 或 RST**，TCP 协议栈对此一无所知。你的 `readyState` 依然是 `OPEN`，`send()` 依然不报错（数据只是写进了内核缓冲区），但对端早就死了。如果没有额外机制，这条"僵尸连接"可以挂几个小时甚至几天。

**第二类：中间设备的空闲超时。** 运营商 NAT 的映射老化时间通常只有 30~300 秒；Nginx 默认 `proxy_read_timeout 60s`；AWS ALB 空闲超时默认 60 秒；企业防火墙也会主动掐掉长时间不通讯的连接。一条空闲的 WebSocket，哪怕双方都活得好好的，也会被中间设备单方面处决。

**第三类：客户端环境的不可控。** 浏览器标签页切后台会被定时器节流，电脑休眠会冻结网络栈，iOS WKWebView 进后台可能直接关掉 WebSocket，移动网络在 WiFi 和 4G 之间切换意味着 IP 变了、旧连接必然作废。

结论：**保活（尽量不断）+ 探活（断了立刻知道）+ 重连（知道后尽快连回来）+ 恢复（连回来后把业务状态补齐）**，这四件事一件都不能少。下面按三个基本问题逐一展开。

------

## 二、问题一：怎么知道连接断了

### 2.1 不能靠"感觉"

先明确一个原则：**不能只靠"我感觉没消息了"来判断断线**。半开连接下 TCP 可能长时间不报错，你必须有一套主动的判定体系。常见的判定来源有五个层次：

| 判定层         | 触发方式                                                     | 说明                                                         |
| -------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 读超时（最硬） | `ReadDeadline` 到期 → `Read` 报错                            | 服务端在 pongWait 时间内收不到**任何**可读数据（业务帧、Ping、Pong、文本心跳都算），就认定连接死亡并关闭 |
| 写失败         | Ping 写不出去 / 业务写失败 / 写缓冲满                        | 写操作报错说明底层连接已不可用，主动关闭                     |
| 对端关闭       | `onclose` 事件 / `Read` 返回 EOF / 收到 RST                  | 最"体面"的断开方式，双方都知情                               |
| 客户端心跳超时 | 发出 Ping 后在窗口内收不到 Pong，或读协程挂了                | SDK 侧本地判死，立即触发重连，不用等服务端                   |
| 业务层踢人     | 服务端下发踢下线帧（如 SuIM 的 2002、OpenIM 的 KickOnlineMsg） | **不是网络断**，是鉴权失败或多端互踢，应停止重连并引导重新登录 |

一句话总结：

> **断线 = 读超时 ∪ 写失败 ∪ 对端关闭 ∪（可选）心跳无应答；业务踢人单独归类，不走重连。**

### 2.2 为什么读超时是"最硬"的一道防线

对服务端来说，读超时是唯一能兜底的机制。设想没有读超时：一个客户端掉电后，它的连接在服务端内存里永远存在——占着一个文件描述符、占着路由表（userID → conn 的映射）里的一格、在线状态永远显示"在线"，消息推过去石沉大海。十万个这样的僵尸连接，就是一场内存和 fd 的慢性泄漏。

读超时的逻辑很简单：**给每次 Read 设一个 deadline，任何入站数据（业务消息、客户端 Ping、对服务端 Ping 的 Pong 响应）都会刷新这个 deadline；deadline 到期还没读到任何东西，就认定对端已死，关连接、清资源、发下线事件。**

这里有个容易被问到的细节：为什么"任意入站数据"都能刷新超时，而不是只认心跳？因为心跳的本质就是"证明对端活着"，一条业务消息同样能证明这一点——活跃用户频繁发消息时，没必要额外计较心跳是否按时到达。

------

## 三、心跳机制：参数怎么定，为什么要双向

### 3.1 先看 IM 服务端怎么做的

IM 的长连接入口是 msggateway 服务，其连接管理逻辑（`client_conn.go`）里有三个关键常量：

| 常量         | 值                       | 作用                                       |
| ------------ | ------------------------ | ------------------------------------------ |
| `pongWait`   | 30s                      | 读超时：这段时间收不到任何可读数据就关连接 |
| `pingPeriod` | 27s（`pongWait * 9/10`） | 服务端主动发 WebSocket Ping 控制帧的周期   |
| `writeWait`  | 10s                      | 单次写操作的超时                           |

核心运转逻辑：

- 每次读成功、或收到 Ping/Pong 控制帧 → 刷新 `ReadDeadline = now + 30s`；
- 服务端每 27s 主动发一个控制帧 Ping，对端（浏览器/SDK）按 WebSocket 协议自动回 Pong，这个 Pong 到达后同样刷新读超时；
- 对 Web 端，还额外支持业务层的文本心跳：浏览器发 `{"type":"ping"}`，服务端回 `{"type":"pong"}`，效果等同——都会刷新读超时。

服务端读循环的骨架代码（基于 gorilla/websocket）大致如下：

```go
const (
    pongWait  = 30 * time.Second
    pingPeriod = (pongWait * 9) / 10 // 27s
    writeWait = 10 * time.Second
)

func (c *Client) readPump() {
    defer c.close()
    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error {
        // 收到 Pong：刷新读超时
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil
    })
    for {
        messageType, message, err := c.conn.ReadMessage()
        if err != nil {
            // 读超时 / 对端关闭 / RST，统一在这里感知断线
            log.Warnf("read message failed: %v", err)
            return
        }
        // 任何成功读取都刷新 deadline
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        if isTextPing(message) { // 兼容浏览器文本心跳 {"type":"ping"}
            c.sendPong()
            continue
        }
        c.handleBusinessMessage(messageType, message)
    }
}

func (c *Client) pingTicker() {
    ticker := time.NewTicker(pingPeriod)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteControl(websocket.PingMessage, nil,
                time.Now().Add(writeWait)); err != nil {
                return // Ping 都写不出去，连接已死，退出触发清理
            }
        case <-c.done:
            return
        }
    }
}
```

### 3.2 `pingPeriod` 为什么是 `pongWait * 9/10`

这是一个经典的设计惯例：**Ping 的发送周期必须严格小于读超时**，留出足够的余量。

推演一下：假设服务端在 t=0 发了一次 Ping，读超时 deadline 是 t=30s。如果对端的 Pong 因为网络延迟花了 4 秒才回来，那么 deadline 会在 t=4s 被刷新。但如果 pingPeriod 等于 pongWait（都是 30s），任何一点网络抖动、GC 停顿、定时器漂移，都可能让"Ping 发出 → Pong 回来"这个往返恰好跨过 deadline，把一条健康连接误杀。取 9/10 意味着每次 Ping 之后还有 3 秒的缓冲等 Pong，误杀概率大大降低。

调参时的另一个约束是**心跳间隔要小于链路上最短的空闲超时**。前文说过，运营商 NAT 老化可能只有 30~60 秒，Nginx 默认 `proxy_read_timeout` 是 60 秒。30s 的 pongWait + 27s 的 Ping 周期正好卡在这条线以内，保证中间设备始终看到这条连接"有流量"。如果你的链路里混有超时更短的设备（某些移动网络 NAT 只有 30 秒左右），就需要把整组参数等比例调小。

### 3.3 客户端（SDK 侧）的心跳

OpenIM 的客户端 SDK（openim-sdk-core 的 `LongConnMgr`）并不完全依赖服务端：

- 自己也维持约 24s 间隔的心跳 Ping；
- 读协程失败、或心跳发出后超时未收到响应 → 判定断线，调用 `reConn`；
- 重连采用指数退避，最多重试约 300 次；
- 重连成功后发出 `CmdConnSuccess` 内部命令，触发 MsgSyncer 按 seq 拉取增量消息（详见第五章）。

### 3.4 为什么必须双向心跳，只让客户端发 Ping 不行吗

这是本文最重要的原理问题之一。很多教程只写"前端定时发 ping，后端回 pong"，然后宣称完成了心跳——这只做了一半。我们把两种方案摆在一起对比：

| 维度           | 只有客户端 Ping                                              | 双向（客户端 Ping + 服务端 Ping/读超时）                     |
| -------------- | ------------------------------------------------------------ | ------------------------------------------------------------ |
| 探活方向       | 只能证明"客户端→服务端"通路大致活着                          | 两边都能主动探活                                             |
| 服务端清僵尸   | 服务端若从不主动 Ping、只被动等业务包，实现上容易漏掉读超时，僵尸连接堆积 | 空闲连接也能在 pongWait 内被服务端主动清掉                   |
| 浏览器兼容性   | 浏览器 JS 无法主动发控制帧 Ping（API 不暴露），只能用文本心跳模拟 | OpenIM 对 Web 端专门让服务端发控制帧 Ping（`PlatformID == Web` 分支），浏览器协议栈自动回 Pong，前端零代码 |
| NAT/防火墙保活 | 只有单向周期流量                                             | 双向周期流量更符合中间设备"会话活跃"的判定，保活更可靠       |

再往深拆三点：

**第一，服务端的读超时不可省略。** 没有读超时，僵尸连接会永久占用 fd 和路由表，在线状态也会失真（人早掉了还显示在线）。而刷新读超时的燃料是"任意入站数据"：业务消息、客户端的文本 Ping、或对服务端控制帧 Ping 的 Pong 响应——三者任一即可。

**第二，客户端也需要自己的 Ping。** 考虑一种半开场景：客户端的读协程挂了（或本地网络单向故障，能收不能发的对偶情况），但服务端视角一切正常、还在按时发 Ping。此时只有客户端自己的"发 Ping → 等 Pong → 超时判死"机制能让它**在本地快速发现问题并重连**，而不是傻等 TCP 层几分钟后才报错。

**第三，文本心跳是对控制帧的兼容补丁，不是替代。** WebSocket 控制帧（Ping/Pong）由协议栈自动处理，浏览器 JS 代码既发不出 Ping 帧、也看不到收到的 Ping 帧。所以面向 Web 的方案普遍补一层业务文本心跳 `{"type":"ping"}` / `{"type":"pong"}`。成熟的服务端（OpenIM、SuIM 的服务端都是如此）会**两种都认**：控制帧 Pong 刷新读超时，文本 ping 也刷新读超时并回文本 pong。

一句话总结分工：

> **服务端 Ping + 读超时 = 清理僵尸连接、维持 NAT 映射；客户端 Ping = 半开连接的本地自愈。只做一侧，必然留下盲区。**

------

## 四、问题二：断了怎么重连

### 4.1 重连的标准闭环

感知到断线只是开始，重连是一个需要精心设计的状态机。标准闭环如下：

```
检测到断开（读超时 / onclose / 心跳无应答）
  → 标记状态为 Connecting，清空进行中的半成品请求
  → 判断是否允许重连：
      · 用户主动登出？          → 不重连
      · 收到业务踢下线帧？       → 不重连，引导重新登录
      · token 已过期？          → 先刷新 token，刷不动就引导登录
      · navigator.onLine=false？→ 暂停重连，等 online 事件
  → 等待退避时间（指数退避 + 随机抖动）
  → 重新发起 WS 握手（URL 或子协议中携带 token）
  → 握手成功 → 进入状态恢复流程（见第五章）
  → 握手失败 → 检查错误类型：
      · 401 / token 失效 → 停止重连，跳转登录
      · 网络错误 / 5xx   → 退避翻倍，继续重试（受最大次数约束）
```

几个容易忽略的细节：

- **"是否允许重连"必须显式判断。** 最常见 bug 是用户点了退出登录，3 秒后连接又"复活"了——因为 onclose 回调无脑触发了重连。要用一个 `manualClose` 标志区分"主动关"与"意外断"。
- **重连前要清空在途请求。** 断线时可能有一堆 `sendReqWaitResp` 式的请求在等响应，连接一断它们永远等不到了。必须让这些 waiter 立刻收到失败（超时错误），由上层决定重试还是报错，否则会泄漏 goroutine / Promise，UI 也会永远转圈。
- **状态要对外暴露。** IM SDK 提供 `Connecting / Success / Failed` 的连接状态回调，UI 据此显示"连接中…/已连接/连接失败"。没有状态回调，用户只能靠猜。

### 4.2 为什么要指数退避，固定 3 秒不行吗

先看固定间隔重连在生产环境会造成什么。假设你的 IM 有 10 万在线用户，服务端发布重启，全部连接同时断开：

- **固定 3s 重连**：每 3 秒，10 万个客户端同时发起一次 WS 握手。服务端还在启动中，握手全部失败；3 秒后又是 10 万个……这就是**重连风暴（thundering herd）**。刚起来的进程被握手洪峰再次打挂，进入"重启 → 风暴 → 再挂"的死循环。移动网络抖动（比如某运营商基站故障恢复）也是同样的剧本。
- 附带伤害：每个客户端每 3 秒空转一次 DNS + TCP + TLS + WS 握手，手机端的电池和流量都被白白烧掉，失败日志刷满磁盘。

**指数退避（exponential backoff）** 的做法是：第 n 次重试的等待时间按指数增长，并设置封顶：

```
delay_n = min(base * decay^n, maxDelay)
```

例如 base=1s、decay=2、maxDelay=30s，序列就是 1s → 2s → 4s → 8s → 16s → 30s → 30s → …。它同时满足三个诉求：

1. **前期恢复快**：第一次只等 1 秒，网络抖一下的场景用户几乎无感；
2. **后期压力小**：持续失败时逐渐拉长到 30s，给故障恢复留出喘息时间；
3. **有封顶**：不会退化成"几分钟才试一次"，避免用户网络早就恢复了却迟迟连不上。

但只有指数退避还不够——如果所有客户端参数相同，它们的退避节奏是**同步**的，第 1 秒一起连、第 2 秒一起连……尖峰依旧存在。所以要叠加**随机抖动（jitter）**，让每次的实际等待时间在 `[delay/2, delay]` 或 `[delay, delay*1.5]` 区间内随机取值，把重连请求在时间轴上摊平：

```javascript
function nextDelay(attempt) {
    const base = 1000, decay = 2, max = 30000;
    const delay = Math.min(base * Math.pow(decay, attempt), max);
    // full jitter：在 [0, delay] 内随机，或 half jitter：[delay/2, delay]
    return delay / 2 + Math.random() * (delay / 2);
}
```

此外还应设置**最大重试次数**（OpenIM SDK 约 300 次）。达到上限后停止自动重连，把状态置为 Failed，交给用户手动点"重新连接"——无限重试对"token 过期""账号被封"这类永远不会成功的场景毫无意义。

### 4.3 一份完整的前端实现

把心跳、退避、状态机合在一起，一个可用于生产的 `ReconnectingWebSocket` 骨架：

```javascript
class ReconnectingWebSocket {
    constructor(url, options = {}) {
        this.url = url;
        this.options = {
            heartbeatInterval: 24000,   // 客户端心跳间隔，小于服务端 pongWait
            heartbeatTimeout: 10000,    // 发 ping 后等 pong 的窗口
            reconnectDelay: 1000,       // 退避基数
            maxReconnectDelay: 30000,   // 退避封顶
            reconnectDecay: 2,          // 退避指数
            maxReconnectAttempts: 300,  // 最大重试次数
            ...options,
        };
        this.ws = null;
        this.attempts = 0;
        this.manualClose = false;
        this.pendingQueue = [];          // 断线期间的离线消息队列
        this.waiters = new Map();        // msgIncr -> {resolve, reject, timer}
        this.connect();
    }

    connect() {
        if (!navigator.onLine) return;   // 断网时不做无谓尝试
        this.ws = new WebSocket(this.url); // url 中带 token

        this.ws.onopen = () => {
            this.attempts = 0;
            this.startHeartbeat();
            this.flushQueue();           // 补发离线消息
            this.emit('open');
            this.onConnSuccess();        // 触发状态恢复（见第五章）
        };

        this.ws.onmessage = (e) => {
            const msg = JSON.parse(e.data);
            if (msg.type === 'pong') { this.clearHeartbeatTimeout(); return; }
            if (msg.errCode === 401 || msg.type === 'kick') {
                this.handleAuthFailure(msg); // 停止重连，引导登录
                return;
            }
            const w = this.waiters.get(msg.msgIncr);
            if (w) { clearTimeout(w.timer); w.resolve(msg); this.waiters.delete(msg.msgIncr); }
            else this.emit('push', msg);
        };

        this.ws.onclose = (e) => {
            this.stopHeartbeat();
            this.failAllWaiters(new Error('connection closed'));
            if (this.manualClose) return;
            this.scheduleReconnect();
        };

        this.ws.onerror = () => this.ws.close();
    }

    scheduleReconnect() {
        if (this.attempts >= this.options.maxReconnectAttempts) {
            this.emit('failed'); return;
        }
        const { reconnectDelay: base, maxReconnectDelay: max, reconnectDecay: decay } = this.options;
        const delay = Math.min(base * Math.pow(decay, this.attempts), max);
        const jittered = delay / 2 + Math.random() * (delay / 2);
        this.attempts++;
        this.emit('connecting', this.attempts);
        this.reconnectTimer = setTimeout(() => this.connect(), jittered);
    }

    startHeartbeat() {
        this.heartbeatTimer = setInterval(() => {
            if (this.ws.readyState === WebSocket.OPEN) {
                this.ws.send(JSON.stringify({ type: 'ping', ts: Date.now() }));
                // pong 超时窗口：等不到就本地判死
                this.heartbeatTimeoutTimer = setTimeout(() => {
                    this.ws.close(); // 触发 onclose → 重连
                }, this.options.heartbeatTimeout);
            }
        }, this.options.heartbeatInterval);
    }

    sendReqWaitResp(payload, timeout = 10000) {
        return new Promise((resolve, reject) => {
            const incr = genIncr();
            const timer = setTimeout(() => {
                this.waiters.delete(incr);
                reject(new Error('request timeout'));
            }, timeout);
            this.waiters.set(incr, { resolve, reject, timer });
            if (this.ws.readyState === WebSocket.OPEN) {
                this.ws.send(JSON.stringify({ ...payload, msgIncr: incr }));
            } else {
                this.pendingQueue.push({ ...payload, msgIncr: incr });
            }
        });
    }

    close() {
        this.manualClose = true;
        clearTimeout(this.reconnectTimer);
        this.ws && this.ws.close();
    }
}
```

对照这份代码再看 SuIM 前端的现状：**onclose 后固定 3s 重连一次，没有客户端 ping，没有指数退避，重连后几乎不做状态恢复（只把 `wsOnline` 置回 `true`）**——上面每一个缺失都对应生产环境里一类真实故障。

------

## 五、问题三：重连后状态怎么恢复

### 5.1 连上 ≠ 业务连续

这是三个问题里最容易被轻视的一个。很多实现重连成功后只干一件事：把"在线"标志置回 true，UI 恢复正常。但请想一下断线的那 30 秒里发生了什么：

- 有人给你发了 3 条消息，服务端推了，但你没有连接，**推送全部丢失**；
- 你订阅了 5 个好友的在线状态，这个订阅关系挂在**旧连接**上，新连接一无所知；
- 断线前你发出了一条消息，正在等回执，现在 waiter 已经悬空；
- 你的会话未读数、群成员变更，都停留在断线前的旧快照。

所以重连成功只是拿回了"管道"，**管道里流过的数据要靠状态恢复补回来**。

### 5.2 IM 的恢复清单

IM SDK 重连成功后会广播 `CmdConnSuccess` 命令，各模块闻令而动，典型的恢复动作有五类：

| 恢复项           | 做法                                                         |
| ---------------- | ------------------------------------------------------------ |
| 连接状态         | 通知 UI"已连接"，恢复发送入口                                |
| **消息同步**     | MsgSyncer 按每个会话的 `maxSeq`（服务端最大序号）对比本地游标，把缺口区间的消息批量拉回来 |
| 会话/好友/群增量 | 按 version 号对比，version 落后就增量拉取，差得太多则全量同步 |
| 在线订阅         | 重新发起 `SubUserOnlineStatus` 一类订阅，恢复好友在线状态推送 |
| 进行中的请求     | 超时失败的 RPC / 信令要么置为失败、要么重放，**绝不能假设对端还在等** |

其中**基于 seq 的消息增量同步**是 IM 的命脉，值得展开。IM 服务端给每个会话的消息编连续递增的 seq；客户端本地记录自己同步到的 `localMaxSeq`。正常在线时，收到推送消息，如果 `msg.seq == localMaxSeq + 1` 就直接落库推进游标，如果发现跳号（`msg.seq > localMaxSeq + 1`），说明中间漏了消息，立刻反查补齐。重连后更简单：直接问服务端"这个会话现在 maxSeq 是多少"，和本地一比，缺多少拉多少。这套机制让"百分百可达"不依赖推送通道的可靠性——**推送只是加速器，seq 同步才是兜底**。

以通话信令为例说明"进行中请求"的处理：断线时你正发起一个视频通话，信令发出去了，对方可能已收到并正在响铃。重连后你不能盲目重发邀请（对方会看到两个来电），也不能干等（旧 waiter 已随旧连接死亡）。正确做法是重连后主动查询通话状态，或让服务端在重连时下发未完成的信令事件，由业务层幂等地对账。


## 六、生产环境三大优化

前面讲的是"标准动作"，这一章是三件让方案真正经得起生产考验的加分项。

### 6.1 页面切后台：降频或暂停心跳，回前台立刻探活

**问题**：浏览器对后台标签页的 `setInterval` 有节流（Chrome 可低至每分钟 1 次），你的 24s 心跳在后台实际可能 60s 才发一次，必然触发服务端读超时被踢；即使不被踢，后台硬发心跳也只是白白耗电。

**做法**：监听 `visibilitychange` 事件——

```javascript
document.addEventListener('visibilitychange', () => {
    if (document.hidden) {
        // 切后台：拉长心跳间隔或直接暂停，交给服务端 Ping 保活
        conn.setHeartbeatInterval(60000); // 或 conn.pauseHeartbeat()
    } else {
        // 回前台：立刻检查连接状态
        conn.setHeartbeatInterval(24000);
        if (conn.ws.readyState !== WebSocket.OPEN) {
            conn.connect();               // 已断，马上重连
        } else {
            conn.sendPingNow();           // 还"开着"？发一次 ping 验活
        }
        conn.syncIncrement();             // 无论断没断，都触发一次增量同步
    }
});
```

两个关键点：其一，回前台时**不要相信 `readyState === OPEN`**——后台期间连接可能早已半开，必须用一次主动 ping 验活；其二，回前台要**无条件触发一次数据同步**，因为后台期间推送大概率丢了。OpenIM SDK 的前台恢复机制（`ResumeForegroundTasks` / `CmdWakeUpDataSync`）就是同样的思路：不假设推送都在，回前台就同步。

### 6.2 断网时暂停重连，网络恢复立即重连

**问题**：用户在飞机上、地铁里，`navigator.onLine === false`，而你的重连定时器还在按退避节奏空转——每次都是注定失败的 DNS/TCP 超时，刷满失败日志、耗光电量。

**做法**：监听浏览器的 `online` / `offline` 事件——

```javascript
window.addEventListener('offline', () => {
    conn.pauseReconnect();   // 清掉重连定时器，停止空转
    conn.setState('offline');
});

window.addEventListener('online', () => {
    // 网络恢复：重置退避计数，立即重连（可加几百 ms 防抖，
    // 因为 online 事件触发瞬间路由/DNS 可能还没就绪）
    setTimeout(() => {
        conn.resetBackoff();
        conn.connect();
    }, 500);
});
```

注意 `navigator.onLine` 的语义是"有网卡连接"，不代表真的能上外网（连着 WiFi 但 WiFi 没网时它仍是 true），所以它只能用来**省掉明知无网的重试**，不能替代心跳探活——两者是互补关系。

### 6.3 Token 过期：识别鉴权失败，停止重连，引导登录

**问题**：IM 的 token 总有有效期。用户挂着页面过夜，第二天 token 过期，连接断开，客户端开始重连——握手时被服务端拒绝，再重连，再被拒……如果把这个当普通网络错误无限退避重试，用户看到的就是永远的"连接中…"，既不报错也不跳登录。

**做法**：把"鉴权失败"从"网络错误"里单独分出来处理：

```javascript
// 握手阶段失败：HTTP 状态或关闭码里带出鉴权错误
// 握手成功后被踢：服务端下发 kick 帧 / errCode 401
handleAuthFailure(msg) {
    this.manualClose = true;        // 关键：阻止后续自动重连
    clearTimeout(this.reconnectTimer);
    this.ws.close();
    clearLocalSession();            // 清本地 token / session
    if (this.refreshToken) {
        tryRefreshAndReconnect();   // 有 refresh token 就先刷新再连
    } else {
        redirectToLogin();          // 没有就引导重新登录
    }
}
```

配套的约定：

- 服务端对 token 过期的连接应给出**明确的错误信号**——握手时返回 HTTP 401，或握手后下发带错误码的踢下线帧（如 SuIM 的 2002），不能含糊地直接 `close()` 让客户端猜；
- 客户端收到明确鉴权错误 → 停止重连 + 清 session + 跳登录页；有 refresh token 机制的先尝试静默刷新，刷新成功自动重连，失败再跳登录；
- IM 场景里 access token 过期直接重登也是常见且可接受的做法，比引入复杂的双 token 刷新链路更务实。

核心原则一句话：**鉴权失败和网络错误必须走不同分支，否则用户会卡死在"一直连接中"。**

------

## 七、落地自查清单

把全文压缩成一张核对表，做 IM 长连接时逐条打勾：

**服务端**

- [ ] 设置了读超时（pongWait，建议 30~60s，须小于链路上最短的中间设备空闲超时）
- [ ] 定时主动发 Ping（pingPeriod = pongWait × 9/10）
- [ ] 任意入站数据（业务帧 / 控制帧 / 文本心跳）都刷新 ReadDeadline
- [ ] 写操作有超时（writeWait），Ping 写失败即关连接
- [ ] 同时支持控制帧 Pong 和文本 `{"type":"ping"}` 两种心跳
- [ ] token 过期返回明确错误码（401 / 踢下线帧），不含糊关闭
- [ ] 连接关闭时清理路由映射、发布下线事件

**客户端**

- [ ] 有自己的心跳 Ping（间隔 < 服务端 pongWait），且 ping 后有 pong 超时窗口
- [ ] 断线重连用指数退避 + 随机 jitter + 封顶 + 最大次数
- [ ] 区分手动关闭 / 网络断开 / 业务踢人三种关闭原因
- [ ] 断线时清空所有在途请求的 waiter，让它们立刻失败
- [ ] 断线期间待发消息进离线队列，重连后补发（配合服务端幂等去重）
- [ ] 重连成功后执行状态恢复流水线：seq 增量同步消息、version 增量同步会话/好友、重订在线状态
- [ ] `visibilitychange`：后台降频/暂停心跳，回前台立刻验活 + 触发同步
- [ ] `online/offline`：断网暂停重连，网络恢复立即重连
- [ ] 鉴权失败：停止重连 → 刷新 token 或引导登录
- [ ] 连接状态（Connecting/Online/Failed）有回调暴露给 UI

------

## 八、总结

回到开头的三个问题，现在可以给出完整答案：

1. **怎么知道断了** —— 不靠感觉，靠体系：服务端读超时兜底清僵尸，写失败即时断，对端关闭走事件，客户端心跳超时本地自愈，业务踢人单独归类。
2. **断了怎么重连** —— 状态机 + 指数退避 + jitter + 上限次数；重连前判断该不该连（登出/踢人/断网/token），重连失败区分网络错误和鉴权错误。
3. **重连后怎么恢复** —— 连上只是拿回管道，必须主动补齐业务状态：按 seq 拉消息缺口、按 version 同步会话关系、重订在线状态、清算悬空请求。

而贯穿三者的两条原理性结论是：

- **心跳必须双向**：服务端 Ping + 读超时负责清僵尸和保 NAT，客户端 Ping 负责半开自愈，缺一侧就有盲区；
- **重连必须退避**：固定间隔在海量客户端场景下就是重连风暴，指数退避 + 随机抖动既保恢复速度又保服务端存活。

长连接这件事，写一个"能跑"的 demo 只要半天，写一个"生产可用"的版本要处理的全是本文这些边角：半开、节流、断网、过期、风暴、对账。IM 的口碑，恰恰就藏在这些边角里。

------
