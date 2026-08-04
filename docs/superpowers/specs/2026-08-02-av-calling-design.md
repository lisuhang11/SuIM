# SuIM 音视频通话设计

**Date:** 2026-08-02  
**Status:** Approved — implementing Phase A  
**Scope:** 分阶段 A（1v1 语音）→ B（1v1 视频）→ C（多人）；本规格以 A 期可落地为准，并锁定不阻挡 B/C 的架构。

## 1. 目标与约束

### 1.1 目标

在 SuIM 现有 IM 栈上增加可靠音视频通话能力：

- 控制平面由 SuIM 自有（呼叫状态、权限、tip、时间线摘要）
- 媒体平面使用自建 **LiveKit**（从 A 期引入，避免 B→C 换栈）
- Web 客户端（`frontend` + `suim-sdk`）先闭环；后续客户端复用同一 REST/tip 协议

### 1.2 已确认决策

| 项 | 选择 |
|---|---|
| 分期 | A 1v1 语音 → B 1v1 视频 → C 多人 |
| 媒体 | 自建 LiveKit SFU（非 P2P、非商业 RTC） |
| 离线振铃 | A 期仅在线可打；预留 `OfflinePush.signal_info` / push 钩子 |
| 服务边界 | 新建 `rtc` 微服务；msggateway 只推 tip |
| 总装路径 | SuIM 原生：REST 控制 + tip 推送 + LiveKit 媒体 |
| 通话记录 | 结束写入会话时间线（系统/摘要消息）；A 期不做独立通话列表页 |

### 1.3 非目标（A 期）

- 真实离线推送通道（FCM / Web Push / 厂商）
- 群通话 / 会议室
- SDP/ICE 经 SuIM 转发
- 录制、转写、虚拟背景
- Go `suim-sdk-core` 通话模块

---

## 2. 架构

### 2.1 组件

```
Client (Web / suim-sdk)
  │  REST: invite / accept / reject / cancel / hangup / token
  ▼
apigateway :9000  ──JWT──►  rtc :8087 (新)
                              │  状态机 · rtc_call · 签 LiveKit JWT
                              │  OnlinePush(call tips)
                              ▼
                         msggateway :9001 ──WS──► Client
                              │
rtc ──gRPC──► message :8084   （结束时写入时间线）
rtc ──gRPC──► relation        （好友校验）
rtc ──gRPC──► msggateway      （在线查询 / OnlinePush）
rtc ──HTTP──► LiveKit         （可选 CreateRoom）
Client ═══════ WebRTC ═══════► LiveKit（媒体）
```

端口约定：`rtc` gRPC **8087**（与现有 8080–8086 域服务并列）。LiveKit 在 compose 中固定 HTTP/WebRTC 端口，并写入 `deploy/.env.example`（实施 plan 阶段选定具体宿主机映射，推荐 LiveKit `7880`）。

### 2.2 平面划分

| 平面 | 职责 | 承载 |
|---|---|---|
| 控制平面 | 发起/振铃/接听/拒绝/取消/超时/忙线/挂断；签发进房 token；写时间线 | `rtc` + apigateway + msggateway tip |
| 媒体平面 | RTP、编解码、弱网、A 期音频 / B 期视频 / C 期多人 | LiveKit + 客户端 LiveKit SDK |

### 2.3 刻意不放进 msggateway

- LiveKit API Key / Secret 与房间生命周期
- 呼叫超时定时器与忙线仲裁
- `rtc_call` 持久化

msggateway 仅增加对 call tip（`contentType` 1401–1407）的透传推送（已有 `OnlinePush` 路径，无需为通话扩 WS 业务上行）。

### 2.4 与现有模式对齐

- 域服务 + etcd 发现 + apigateway REST：同 user/relation/message
- 实时提示：同 `pkg/notification` + msggateway `OnlinePushMsg`
- 系统摘要落会话：同群事件写 message 的思路（专用 `content_type`）
- 在线判断：复用 msggateway / Redis presence（与 2026-08-02 online-presence 一致）

---

## 3. 呼叫状态机（A 期 1v1）

### 3.1 状态

| 状态 | 含义 |
|---|---|
| `ringing` | 已发起，等待被叫 |
| `accepted` | 被叫已接，双方持 token 可进房 |
| `active` | 通话进行中（A 期可用「已发 token」简化；精确进房可用 LiveKit webhook，B 期补齐亦可） |
| `ended` | 终态，带 `end_reason` |

### 3.2 主路径

1. Caller `POST /calls/invite` → rtc 校验好友、被叫在线、双方非忙线 → 建 `rtc_call=ringing` → 为主叫签 LiveKit token → tip `1401 call.invite` 推被叫  
2. Callee `POST /calls/{id}/accept` → `accepted` → 返回被叫 token → tip `1402` 推主叫（及被叫其他端停铃）  
3. 双方 LiveKit `connect`  
4. 任一方 `POST /hangup` → `ended/completed` → tip `1407` → message 写 `1501` 摘要 → 删除/关闭 LiveKit room  

### 3.3 结束原因

| reason | 场景 | 时间线文案示例 |
|---|---|---|
| `completed` | 接通后挂断 | 语音通话 03:24 |
| `rejected` | 被叫拒绝 | 已拒绝 |
| `cancelled` | 振铃中主叫取消 | 已取消 |
| `timeout` | 振铃超时（默认 **45s**，rtc 定时器） | 未接来电 |
| `busy` | 被叫已有进行中通话 | 忙线未接通 |
| `unavailable` | 被叫不在线（A 期） | 对方不在线 |

### 3.4 并发与多端

- 同一 `user_id` 同时最多 **1** 路进行中（`ringing|accepted|active`）
- 新 invite 打到忙线用户 → 创建一条立即 `ended/busy` 的 `rtc_call`，向主叫返回错误（并可选 tip `1406`），**不**向被叫振铃；随后写时间线 `1501`（reason=`busy`）
- 被叫离线 → 创建一条立即 `ended/unavailable` 的 `rtc_call`，向主叫返回错误，写时间线 `1501`（reason=`unavailable`），**不**发 `1401`
- 多 platform：所有在线端收 tip；**任一端 accept** 后，其余端收 `accepted`/`ended` 停铃

---

## 4. API

均需 JWT；经 apigateway 转发 `rtc` gRPC。

| Method | Path | 说明 |
|---|---|---|
| POST | `/api/v1/calls/invite` | body: `{ callee_id, media_type }` → `{ call_id, room_name, token, status }` |
| POST | `/api/v1/calls/{call_id}/accept` | → `{ token, room_name, media_type }` |
| POST | `/api/v1/calls/{call_id}/reject` | |
| POST | `/api/v1/calls/{call_id}/cancel` | 主叫振铃中取消 |
| POST | `/api/v1/calls/{call_id}/hangup` | |
| GET | `/api/v1/calls/{call_id}` | 重连/多端校准 |
| POST | `/api/v1/calls/{call_id}/token` | 刷新进房 token |

约束：

- A 期 `media_type` 仅 `audio`；B 期允许 `video`
- 好友关系：rtc 调 relation（与单聊发信一致）；非好友拒绝
- 被叫离线：invite 返回 `unavailable`，并写时间线摘要（见 §3.4）

### 4.1 Proto（新建）

`proto/rtc.proto`：Invite / Accept / Reject / Cancel / Hangup / GetCall / RefreshToken 及对应 messages。  
`services/rtc` 按现有服务骨架：`cmd/server`、`internal/service`、`etc/*.yaml`，etcd 注册名 `rtc`。

---

## 5. Tip 与 SDK 事件

| contentType | 事件名 | 载荷要点 |
|---|---|---|
| 1401 | `call.invite` | call_id, caller_id, media_type, conversation_id |
| 1402 | `call.accepted` | call_id |
| 1403 | `call.rejected` | call_id |
| 1404 | `call.cancelled` | call_id |
| 1405 | `call.timeout` | call_id |
| 1406 | `call.busy` | call_id（可选；主叫也可仅看 REST 错误） |
| 1407 | `call.ended` | call_id, reason, duration_sec |

- 走 `pkg/notification` + msggateway `OnlinePush`；**不**作为可拉取历史消息持久化
- 常量写入 `pkg/notification/constant.go`
- `frontend/src/suim-sdk/listener/ws.ts` 归一化为 `call.*` 事件

---

## 6. 数据模型

### 6.1 `rtc_call`（MySQL，rtc 服务）

| 字段 | 说明 |
|---|---|
| call_id | PK |
| conversation_id | `si_<min>_<max>` |
| caller_id / callee_id | |
| media_type | `audio` \| `video` |
| status | `ringing` \| `accepted` \| `active` \| `ended` |
| end_reason | nullable |
| room_name | LiveKit room |
| started_at / answered_at / ended_at | |
| duration_sec | |
| created_at / updated_at | |

索引：`(caller_id, status)`、`(callee_id, status)` 支撑忙线查询。

### 6.2 时间线消息

- `content_type = 1501`（CallRecord）
- `content` JSON：`{ call_id, media_type, reason, duration_sec }`
- `msg_from` 使用系统消息约定（与现有 tip/系统消息一致）
- 单聊一条 `msg_info`，双方可见；更新会话 lastMsg
- UI：`MessageBubble` 映射文案

---

## 7. 推送钩子（A 期）

- invite / ended 时组装 `OfflinePush.signal_info`（至少含 call_id、media_type、action）
- 调用现有 `push.PushMsg` 路径；实现可保持 no-op，直到真实通道就绪
- **不**将离线振铃列入 A 期完成标准

---

## 8. 客户端

### 8.1 suim-sdk

- `modules/call.ts`：REST 封装
- `modules/call_media.ts`（或等价）：LiveKit Room connect / mute / disconnect；（B）camera
- WS：`1401–1407` → `call.*`
- `ChatContext`（或专用 CallContext）订阅事件，驱动**全局**来电层

### 8.2 UI

- 来电层：接听 / 拒绝（接现有 `ChatHeader` 语音按钮）
- 通话中层：时长、静音、挂断；（B）视频画面与摄像头开关
- A 期仅启用语音按钮；视频按钮 B 期启用
- 时间线渲染 `1501`

### 8.3 客户端错误处理

- invite：`unavailable` / `busy` / `not_friend` → toast，不进通话页
- WS 断线：媒体靠 LiveKit；恢复后 `GET /calls/{id}` 校准；已 `ended` 则退出 UI
- token 过期：`POST .../token` 重进

---

## 9. 基础设施

- `deploy/docker-compose.yml` 增加：`livekit`、`rtc`
- 环境变量：`LIVEKIT_URL`、`LIVEKIT_API_KEY`、`LIVEKIT_API_SECRET`；前端 `NEXT_PUBLIC_LIVEKIT_URL`
- 开发：LiveKit 官方单节点镜像；生产再强化 TURN / 多节点
- `.env.example` 同步文档化

---

## 10. 分期交付

### Phase A — 1v1 语音（本规格实施重点）

- [ ] `proto/rtc.proto` + `services/rtc`
- [ ] apigateway 路由与 JWT 保护
- [ ] LiveKit 进 compose；token 签发
- [ ] invite/accept/reject/cancel/hangup/timeout/busy/unavailable
- [ ] tip 1401–1407；时间线 1501
- [ ] push 钩子调用（no-op 可接受）
- [ ] suim-sdk call + media；Web 来电/通话 UI；ChatHeader 接通
- [ ] 好友 + 在线校验

### Phase B — 1v1 视频

- [ ] `media_type=video`；本地/远端渲染与摄像头开关
- [ ] 可选 LiveKit webhook 精准确认 `active`
- [ ] 权限/弱网提示

### Phase C — 多人

- [ ] 群/房间维度 API（如 `POST /calls/group`）
- [ ] 同一 LiveKit room 多参与者；成员与静音管理
- [ ] 群通话时间线摘要
- [ ] 仍复用 rtc + tip 骨架，不换媒体栈

### 并行随后

- [ ] 真实离线推送接入 `signal_info`（不阻塞 A）

---

## 11. 测试要点（A 期）

1. 双浏览器账号：接通、拒绝、取消、超时、忙线  
2. 被叫离线：`unavailable` + 时间线文案  
3. 多端：一端接听，另一端停铃  
4. 挂断后双方出现 `1501` 摘要且会话 lastMsg 更新  
5. 非好友无法 invite  

---

## 12. 风险与缓解

| 风险 | 缓解 |
|---|---|
| NAT / 公司网不通 | 尽早在 compose 配好 LiveKit TURN；真机跨网测 |
| tip 丢失导致停铃失败 | `GET /calls/{id}` 校准；振铃 UI 本地也起 45s 超时 |
| rtc 与 message 写摘要失败 | 结束先改 call 状态再尽最大努力写 message；可补偿任务（非 A 阻塞） |
| LiveKit 密钥泄漏 | 仅 rtc 持有；前端只拿短 TTL 用户 token |

---

## 13. 主要触点文件（实施时）

| 层 | 路径 |
|---|---|
| Proto | `proto/rtc.proto` |
| 服务 | `services/rtc/**` |
| 网关 | `services/apigateway/internal/router/**`、`handler/**`、`grpc/clients.go` |
| 常量 | `pkg/notification/constant.go` |
| 部署 | `deploy/docker-compose.yml`、`deploy/.env.example` |
| SDK | `frontend/src/suim-sdk/modules/call.ts`、`listener/ws.ts` |
| UI | `frontend/src/components/chat/ChatHeader.tsx`、来电/通话层、`MessageBubble.tsx` |
