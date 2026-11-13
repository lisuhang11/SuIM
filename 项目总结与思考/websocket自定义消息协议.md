# SuIM 自定义 WebSocket 消息协议

IM 的实时通道建立在 WebSocket 之上，但 WebSocket 本身只保证「双方能互发二进制或文本帧」，并不规定业务语义。SuIM 在 msggateway 上定义了一套**业务帧协议**：握手参数、外层信封、`reqIdentifier` 操作码、内层 protobuf 载荷。SDK / Web 前端与网关之间的发消息、拉历史、推送、踢下线、在线订阅、通话信令，全部走这套约定。

本文只讲 SuIM 自己的协议与实现（代码主路径：`services/msggateway/internal/ws/`，载荷定义：`proto/msg/msg.proto`）。

------

## 一、协议在整体里的位置

```
客户端 SDK / Web
        │  WebSocket（握手鉴权 + 业务二进制帧 + 文本心跳）
        ▼
   msggateway
        │  gRPC / 内部调用
        ├─► msg 服务     （发消息、拉 seq、已读）
        ├─► rtc 服务     （通话信令 Assemble）
        ├─► online       （在线状态读写与订阅扇出）
        └─► toPush 消费  （离线/跨机推送到本机连接）
```

分工可以概括成一句话：

> **WebSocket 负责「在线用户 ↔ 网关」的实时双向通道；真正的消息落库、序号分配、会话逻辑在 msg；通话状态机在 rtc；网关只做鉴权、路由、编解码和推送落地。**

因此协议设计的目标很明确：

1. **一条连接复用多种业务**——用 `reqIdentifier` 区分，而不是开多条 WebSocket。
2. **请求可关联响应**——`msgIncr` / `operationID` 做客户端侧匹配。
3. **服务端可主动推**——推送帧没有客户端请求，但外层仍用同一套 `Resp` 信封。
4. **载荷可演进**——外层 JSON/gob 字段稳定，内层用 protobuf，便于加字段。

------

## 二、握手：连上之前先把身份说清楚

### 2.1 入口

客户端连接 msggateway 的 HTTP Upgrade 路径（Web 示例为 `/gateway?...`）。握手阶段**尚未升级成功前**就会校验参数和 token；失败则可能直接 HTTP 错误，或在 `isMsgResp=true` 时先 Upgrade 再写一帧错误 JSON 后关闭。

### 2.2 参数两种写法

**写法 A：扁平 Query（Web / JS 常用）**

| 参数 | 含义 | 是否必填 |
|------|------|----------|
| `token` | 登录令牌 | 必填 |
| `sendID` | 用户 ID（与 token 内 user 必须一致） | 必填 |
| `platformID` | 端类型，整数 `1–12`（如 Web=5、iOS=1） | 必填 |
| `operationID` | 本次握手的追踪 ID | 必填 |
| `sdkType` | `go` 或 `js`，决定后续帧编码 | 默认 `go` |
| `sdkVersion` | SDK 版本字符串，仅记录 | 可选 |
| `compression` | 填 `gzip` 则启业务层压缩 | 可选 |
| `isMsgResp` | `true` 时握手结果以 WS 文本帧返回 | 可选 |
| `isBackground` | 初始是否后台（影响 iOS 在线推送） | 可选 |

**写法 B：压缩握手包 `v=`**

把上述字段 JSON 序列化后做 `base64.RawURLEncoding`，放进单个 query 参数 `v=`。适合参数多、不想把 token 明文散在日志 URL 里的原生 SDK。Query 上的 `isMsgResp` / `sdkVersion` 仍可覆盖或补齐。

### 2.3 鉴权与建连结果

1. 解析参数失败 → 参数错误（业务码约 `1001`）。
2. 连接数超过上限 → 服务不可用。
3. `tokens.Validate(token)` 失败，或解析出的 `userID` / `platformID` 与 query 不一致 → token 无效（约 `1502`）。
4. Upgrade 成功后：
   - 若 `isMsgResp=true`，先写一帧文本：`{"errCode":0,"errMsg":"","errDlt":""}`；
   - 创建 `Client`（分配 `connID`、选择 encoder、是否 gzip）；
   - 注册进 Hub；同平台旧连接可能被静默替换或踢下线（见第七节）；
   - 上报在线、触发 `afterUserOnline` webhook；
   - 异步 `claimPlatform`（通知其它网关实例踢掉同平台旧会话）。

握手示例（JS）：

```
ws://host/gateway?sendID=u1&token=...&platformID=5&operationID=<uuid>&sdkType=js
```

------

## 三、外层信封：所有业务帧长什么样

### 3.1 请求 `Req`（客户端 → 服务端）

| 字段 | 类型 | 作用 |
|------|------|------|
| `reqIdentifier` | int | 操作码，决定走哪条业务 |
| `token` | string | 可带；网关主要信任握手时的 token |
| `sendID` | string | 若填写，必须等于当前连接的 userID |
| `operationID` | string | 链路追踪，响应原样带回 |
| `msgIncr` | string | **客户端生成的请求序号/关联键**，响应用它配对 |
| `data` | bytes | 内层 protobuf（或空） |

### 3.2 响应 / 推送 `Resp`（服务端 → 客户端）

| 字段 | 类型 | 作用 |
|------|------|------|
| `reqIdentifier` | int | 与请求相同（推送则用推送类 opcode） |
| `msgIncr` | string | 回带请求的 `msgIncr`；纯推送可为空 |
| `operationID` | string | 回带 |
| `errCode` | int | `0` 成功，非 0 失败 |
| `errMsg` | string | 错误文案 |
| `data` | bytes | 成功时的 protobuf 载荷 |

关键约定：

- **请求-响应型**（`1001–1007`、`2003–2005` 等客户端发起的）：响应的 `reqIdentifier` / `msgIncr` / `operationID` 与请求对齐，客户端用 `msgIncr` 完成 Promise 匹配。
- **服务端推送型**（`2001` 消息、`2002` 踢人、`2005` 在线变更推送、`2006` 通话信令）：通常没有对应请求；客户端按 `reqIdentifier` 分支处理，`msgIncr` 可空。

### 3.3 编码与压缩

| `sdkType` | 外层序列化 | 典型端 |
|-----------|------------|--------|
| `js` | JSON（`data` 常为 protobuf 的 base64 字符串） | Web |
| `go`（默认） | gob | Go SDK |

若握手 `compression=gzip`：

```
写出：Encode(Resp/Req) → gzip → WebSocket Binary
读入：Binary → gunzip → Decode
```

未开压缩则直接 Encode 后的二进制进 Binary 帧。

单帧读上限约 **51200** 字节（`maxBytes`），写缓冲队列深度 256，单次写超时 10s。

### 3.4 文本帧只做心跳，不做业务

| 方向 | 内容 |
|------|------|
| 客户端 → 服务端 | `{"type":"ping","body":...}` |
| 服务端 → 客户端 | `{"type":"pong",...}`（原 body 带回） |

其它 `type` 视为非法并断开。业务一律走 **Binary**。另有一层 WebSocket 控制帧 Ping/Pong（服务端约每 27s 发 Ping，读超时 30s），与文本心跳并存，详见《IM WebSocket 长连接实战指南》。

------

## 四、操作码总表

### 4.1 客户端请求（1xxx）

| Opcode | 常量名（网关） | 含义 | 内层请求 protobuf | 内层成功响应 |
|--------|----------------|------|-------------------|--------------|
| **1001** | `reqGetNewestSeq` | 拉取各会话最大/最小 seq | （可空；user 取自连接） | `GetMaxSeqResp` |
| **1002** | `reqPullMsgByRange` | 按会话 seq 区间拉消息 | `PullMessageBySeqsReq` | `PushMessages` |
| **1003** | `reqSendMsg` | 发送一条消息 | `MsgData` | `SendMsgResp` |
| **1004** | `reqSendSignalMsg` | 通话信令（上行） | `CallSignal` | `CallSignal` |
| **1005** | `reqPullMsgBySeqList` | 按指定 seq 列表拉消息 | `GetSeqMessageReq` | `PushMessages` |
| **1006** | `reqGetConvMaxReadSeq` | 会话已读 seq + 最大 seq | `GetConversationsHasReadAndMaxSeqReq` | `GetConversationsHasReadAndMaxSeqResp` |
| **1007** | `reqPullConvLastMsg` | 各会话最后一条 | `GetLastMessageReq` | `GetLastMessageResp` |

> 命名注意：常量里 `1002` 叫 ByRange、`1005` 叫 BySeqList，与「按区间 / 按列表」的语义一致；对接时以 **数字 opcode** 为准。

### 4.2 服务端推送 / 双向控制（2xxx）

| Opcode | 常量名 | 方向 | 含义 | 内层 `data` |
|--------|--------|------|------|-------------|
| **2001** | `reqPushMsg` | S→C | 实时消息推送 | `PushMessages` |
| **2002** | `reqKickOnlineMsg` | S→C | 被踢下线 | 通常为空；随后关连接 |
| **2003** | `reqLogoutMsg` | C→S | 主动登出 | `DelUserPushTokenReq` → `DelUserPushTokenResp` |
| **2004** | `reqSetBackground` | C→S | 前后台状态 | `SetAppBackgroundStatusReq` |
| **2005** | `reqSubUserOnline` | 双向 | 订阅在线；变更时再推 | 请求 `SubUserOnlineStatus`；响应/推送 `SubUserOnlineStatusTips` |
| **2006** | `reqCallSignal` | S→C | 通话信令推送 | `CallSignal` |

### 4.3 常见外层错误码

| 码 | 含义 |
|----|------|
| `0` | 成功 |
| `1001` | 参数错误（含不支持的 opcode、protobuf 解失败、sendID 不匹配） |
| `1502` | Token 无效（握手阶段） |
| `500` | 内部错误 / 依赖服务不可用 |
| 其它业务码 | 来自 msg/rtc 的错误码透传（如非好友、被拉黑、无权限、通话非法等） |

------

## 五、消息类 opcode 详解

### 5.1 `1001` 获取最新 Seq

**用途：** 重连或冷启动后，先知道每个会话「服务端目前最大 seq 到哪」，再决定要不要拉增量。

- 网关调用 `messages.GetMaxSeq(userID)`。
- 响应 `GetMaxSeqResp`：
  - `max_seqs`：`conversation_id → max_seq`
  - `min_seqs`：会话保留窗口下沿（若有）

客户端逻辑通常是：对每个会话，若本地 `syncedSeq < max`，再发 `1002` 或 HTTP pull 补齐。

### 5.2 `1002` 按区间拉消息

**请求 `PullMessageBySeqsReq`：**

| 字段 | 说明 |
|------|------|
| `user_id` | 网关会强制写成连接用户 |
| `seq_ranges` | 多个 `SeqRange`：`conversation_id` + `begin`/`end`/`num` |
| `order` | `0` 升序，`1` 降序 |

**响应 `PushMessages`：** 与推送同结构（见 5.5），按会话分桶。

典型场景：发现空洞（本地 10，推送来了 15）时，拉 `11..15`。

### 5.3 `1003` 发消息

**请求 body：** 直接是 `MsgData`（不是包一层 `SendMsgReq`）。

关键字段：

| 字段 | 说明 |
|------|------|
| `client_msg_id` | 客户端生成，幂等与 UI 乐观更新主键 |
| `recv_id` / `group_id` | 单聊 / 群聊目标 |
| `session_type` | 会话类型 |
| `content_type` / `content` | 内容类型与二进制内容 |
| `options` 等 | 扩展选项、离线推送信息等 |

网关行为：

1. 解出 `MsgData`，**强制** `send_id = 连接用户`，`sender_platform_id = 连接平台`（防伪造）。
2. 调 `messages.SendMsg`。
3. 回 `SendMsgResp`：`server_msg_id`、`client_msg_id`、`send_time`，以及可能的 `modify`。

客户端侧习惯：把 `msgIncr` 设成与 `client_msg_id` 相同或可映射的值，这样 `1003` 回执能直接关掉「发送中」状态。失败则 `errCode != 0`，UI 标红。

消息真正到达对端在线设备，不依赖这条回执本身，而是依赖后续的 **`2001` 推送**（发送方多端、接收方各端都会走推送链路）。

### 5.4 `1005` / `1006` / `1007`

| Opcode | 场景 |
|--------|------|
| **1005** | 已知精确 seq 列表（例如通知补洞、指定回放），`GetSeqMessageReq` |
| **1006** | 会话列表页需要「已读到哪 / 最大到哪 / 置顶」，`GetConversationsHasReadAndMaxSeq*` |
| **1007** | 会话预览最后一条，`GetLastMessage*` |

三者都把 `user_id` 覆盖为连接用户，防止越权读别人的会话。

### 5.5 `2001` 消息推送（最核心的下行）

当 toPush 队列把一条已落序消息交到本机网关时：

1. 解出 `PushMsgData`（`msg_data` + `conversation_id`）。
2. 算接收人列表：群聊查成员；单聊取 `send_id`/`recv_id`。
3. 对每个在线且「接受在线推送」的连接写一帧：
   - `reqIdentifier = 2001`
   - `data = proto(PushMessages)`

**`PushMessages` 结构：**

```
msgs             : map<conversation_id, PullMsgs>   // 普通会话消息
notification_msgs: map<conversation_id, PullMsgs>   // 通知会话（如群通知流）
```

**`PullMsgs`：**

| 字段 | 说明 |
|------|------|
| `msgs` | 本批 `MsgData` |
| `is_end` / `end_seq` / `max_seq` | 拉取场景下的分页信息；单条实时推送时也可能带 |

**特殊规则：iOS + 后台不推在线帧。**  
`platformID == 1 (iOS)` 且 `background == true` 时跳过 `2001`，留给系统离线推送，避免浪费且符合后台策略。

客户端收到 `2001` 后应：

1. 解码并入本地会话；
2. 检查 seq 连续性，有空洞则 `1002` 补拉；
3. 更新本地 maxSeq；活跃会话可再调已读接口。

------

## 六、会话控制类：登出、前后台、在线订阅

### 6.1 `2003` 登出

客户端发送 `DelUserPushTokenReq`（可空 body）：

```
user_id, platform_id
```

当前实现：**校验/解包后回空的 `DelUserPushTokenResp`，然后关闭连接**。注释写明尚未接推送令牌表，因此**不会真正删除远端 push token**。协议位已留好，业务落地仍是缺口。

登出成功后连接结束；客户端不应把这次关闭当网络抖动去无限重连，应清会话、回登录页。

### 6.2 `2004` 设置前后台

请求 `SetAppBackgroundStatusReq`：

```
user_id, is_background
```

网关只更新本连接的 `background` 标志并回成功。效果主要体现在：

- iOS 进后台后不再收 `2001` 在线推送；
- 查询在线状态时，`Platform.is_background` 会反映该标志。

### 6.3 `2005` 订阅用户在线状态

**客户端请求 `SubUserOnlineStatus`：**

```
subscribe_user_id[]     // 增加订阅
unsubscribe_user_id[]   // 取消订阅
```

**立即响应 `SubUserOnlineStatusTips`：** 对刚订阅的用户，带回当前 `online_platform_ids`。

**之后的推送：** 当被订阅用户上下线（本机或其它网关经 online 组件广播），网关向所有 Watcher 再推一帧 `reqIdentifier=2005`，body 仍是 `SubUserOnlineStatusTips`（通常只含变更的那一个用户）。

注意：`2005` 既是「订阅 RPC 的响应码」，也是「后续状态变更的推送码」。客户端要用上下文区分：带 `msgIncr` 的多半是对自己请求的回执；无 `msgIncr`、异步到达的是状态推送。

------

## 七、多端互踢：`2002` 与连接替换

### 7.1 Hub 规则（同一网关进程内）

同一 `userID + platformID`：

| 情况 | 行为 |
|------|------|
| 新连接 token **相同** | 旧连接**静默关闭**（不发 2002），在线记录可保持 |
| 新连接 token **不同** | 旧连接走 **Kick**：先发 `2002`，再关连接 |

### 7.2 跨网关

新连接建立后 `claimPlatform`：

1. 尽量 `ReplacePlatform` 作废同平台旧 token；
2. 通过 peer 列表对其它 msggateway 调 `Kick`，带上新 token；
3. 对端把同用户同平台、且 token 不匹配的连接踢掉（发 `2002`）。

### 7.3 客户端如何对待 `2002`

这是**业务踢人，不是断网**：

- 应停止自动重连；
- 清本地 session / token；
- 提示「账号已在其他端登录」并回到登录。

Web 参考实现收到 `2002` 后会 `localStorage.removeItem('suim.session')` 并清空内存会话。

------

## 八、通话信令：`1004` 上行 + `2006` 下行

音视频媒体面走 LiveKit 等 RTC；**呼叫状态机信令**走 SuIM 自定义帧，不占用 `2001` 消息通道。

### 8.1 职责拆分

| 角色 | 做什么 |
|------|--------|
| 客户端 | `1004` 发 `CallSignal`（邀请、接听、拒绝、挂断、入会等） |
| msggateway | 鉴权后把信号交给 rtc `Assemble`；把 rtc 广播的事件写成 `2006` 推给相关在线用户 |
| rtc 服务 | 维护通话房间状态，决定谁该收到什么 op，以及何时下发 `url`/`token` |

### 8.2 `CallSignal` 字段

| 字段 | 含义 |
|------|------|
| `op` | 见下方枚举 |
| `call_id` | 通话 / 房间 ID |
| `peer_id` | 单聊对端；或群通话里进/出的人 |
| `media` | `AUDIO` / `VIDEO` |
| `caller_id` / `callee_id` | 主叫 / 被叫 |
| `reason` | 失败或结束原因文案 |
| `url` / `token` | 进 LiveKit 等媒体房的凭证 |
| `group_id` | 非空表示群通话 |
| `user_ids` | 已入会名单等 |

**`CallOp` 枚举（节选）：**

| 值 | 名 | 典型含义 |
|----|-----|----------|
| 1 | `CALL_INVITE` | 发起邀请 |
| 2 | `CALL_CANCEL` | 取消 |
| 3 | `CALL_ACCEPT` | 接听 |
| 4 | `CALL_REJECT` | 拒绝 |
| 5 | `CALL_HANGUP` | 挂断 |
| 6 | `CALL_RING` | 振铃 |
| 7 | `CALL_TIMEOUT` | 超时 |
| 8 | `CALL_BUSY` | 忙线 |
| 9 | `CALL_ANSWERED` | 已在其他端接听 |
| 10 | `CALL_JOINED` | 已加入 |
| 11 | `CALL_LEFT` | 已离开 |
| 12 | `CALL_JOIN` | 请求加入 |
| 13 | `CALL_GROUP` | 群呼相关 |

媒体凭证下发规则（协议注释约定）：

- **单聊：** 一般在 `CALL_ACCEPT` 时带 `url`/`token`；
- **群聊：** 主持人在 `CALL_INVITE`、其余人在加入成功时带 `url`/`token`。

### 8.3 交互时序（单聊示意）

```
主叫                          网关 / rtc                         被叫
  │ 1004 INVITE ──────────────►│                                  │
  │◄──── 1004 回执(可能含状态)──│                                  │
  │                            │──────── 2006 INVITE/RING ───────►│
  │                            │◄─────── 1004 ACCEPT ─────────────│
  │◄─────── 2006 ACCEPT(+url)──│─────── 1004 回执(+url) ─────────►│
  │         双方拿 token 进媒体房                                   │
```

要点：

- **自己的操作结果**看 `1004` 回执（用 `msgIncr` 等待）。
- **对端或系统推过来的状态**看 `2006`（无 `msgIncr`）。
- 网关推 `2006` 时会跳过 `Except` 连接（避免操作方自己再收一份重复），其它端、对端照常收。

Bot / 自动接听场景下，Accept 可能出现在 `1004` 回执里，也可能先以 `2006` 推到；客户端两边都要能收敛状态机。

------

## 九、一条消息从发到收的完整路径（协议视角）

以用户 A 给 B 发文本为例：

1. **A 发 `1003`**：`data=MsgData`，`msgIncr=clientMsgId`。
2. **网关 → msg**：校验关系、分配 `server_msg_id` / `seq` / `send_time`，持久化，投递 toPush（及落库流水）。
3. **网关 → A**：`1003` 成功回执（`SendMsgResp`）。A 的 UI 从「发送中」变「已发送」。
4. **推送消费者 → 各网关**：带上 `PushMsgData`。
5. **B（及 A 的其它在线端）收 `2001`**：解码 `PushMessages`，插入会话；若 seq 不连续则 `1002` 补洞。
6. **B 读了消息**：可通过 HTTP/RPC 已读接口（网关也暴露了部分 HTTP 读写），不一定再走 WS opcode；已读变更若以通知消息形式出现，仍可能再推 `2001`。

重连恢复最小闭环：

```
连上 → 1001 拿 maxSeq → 与本地比较 → 1002 拉增量 → 开始收 2001
```

------

## 十、客户端实现要点（对照 Web）

1. **`binaryType = 'arraybuffer'`**（若走二进制）；JS 路径下外层是 JSON 文本解码后的对象，`data` 字段再 base64→protobuf。
2. **忽略文本心跳帧**：`type: ping/pong` 不要当业务 JSON 解析失败。
3. **分支顺序建议：**
   - `2002` 踢人 → 停重连、清登录；
   - `1003` → 结束发送 Promise；
   - `1004` → 结束信令 Promise；
   - `2006` → 更新通话 UI；
   - `2001` → 入消息 + 补洞；
   - `2005` → 更新在线点。
4. **`msgIncr` 必须唯一且在超时前可清理**，避免泄漏 Map。
5. **踢人与断网分流：** `onclose` 可重连；`2002` 不可盲目重连。

------

## 十一、当前实现上的边界与缺口

写协议文档时把「约定」和「已落地」分开，避免对接方误判：

| 项 | 状态 |
|----|------|
| 消息收发 / 拉取 / 推送 `1001–1003,1005–1007,2001` | 已落地 |
| 前后台 `2004`、在线订阅 `2005`、踢人 `2002` | 已落地 |
| 通话 `1004` + `2006` | 已落地（依赖 rtc） |
| 登出 `2003` 清推送 token | **协议有，push token 表/删除未接** |
| 多端策略 | 固定「同平台异 token 互踢」，无更细的 PC 共存等策略配置 |
| 网关旁路 HTTP 发/拉/已读 | 存在，但是 HTTP API，不属于本文 opcode 表 |

------

## 十二、一张图串起来

```
                    ┌─────────────────────────────────────┐
                    │           WebSocket 连接              │
                    │  握手: token + sendID + platformID   │
                    │  编码: js→JSON / go→gob (±gzip)      │
                    └───────────────┬─────────────────────┘
                                    │
         ┌──────────────────────────┼──────────────────────────┐
         │                          │                          │
    文本 ping/pong            二进制 Req/Resp              控制帧 Ping
         │                          │
         │              ┌───────────┴───────────┐
         │              │   reqIdentifier       │
         │              └───────────┬───────────┘
         │     ┌────────────┬───────┼───────┬────────────┐
         │   1001–1007    1004    2003–2005 │          服务端推
         │   消息 RPC     通话上行  会话控制 │     2001 消息
         │              回执同码            │     2002 踢人
         │                                 │     2005 在线变更
         │                                 │     2006 通话推
         ▼                                 ▼
      保活读超时刷新              data = protobuf 业务体
```

------

## 十三、总结

SuIM 的自定义消息协议，本质是在 WebSocket 上叠了三层约定：

1. **握手层**——用 query（或 `v=` 包）完成身份、端类型、编码与压缩协商；
2. **信封层**——统一的 `Req`/`Resp` + `reqIdentifier` + `msgIncr`，让一条连接承载请求响应与服务端推送；
3. **载荷层**——protobuf（`MsgData`、`PushMessages`、`CallSignal`…）承载可变业务字段。

对接时记住三条经验即可：

- **号码决定语义，名字只是注释**；
- **`1003`/`1004` 看回执，`2001`/`2006` 看推送**；
- **`2002` 是业务下线，不要当网络抖动重连**。

代码入口：`SuIM/services/msggateway/internal/ws/codec.go`（操作码与编解码）、`handler.go`（分发）、`signal.go`（通话）、`client.go` / `server.go`（读写与握手）、`proto/msg/msg.proto`（载荷 schema）。
