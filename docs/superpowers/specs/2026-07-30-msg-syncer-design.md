# MsgSyncer（重连 / 漏推补齐）设计

**Date:** 2026-07-30  
**Status:** Approved (delivery B / store IndexedDB / trigger A / APIs C / hasRead use A / approach 1)  
**Scope:** P0 消息正确性第 1 项（独立 spec；未读 / 群成员 / 撤回推送 / 已读回执另开）

## Goal

登录或 WebSocket 重连后，按「本地 maxSeq vs 服务端 maxSeq」拉齐缺口，避免推送丢失后永远看不到消息（除非手动进会话拉历史）。

对齐 OpenIM 重连主路径：`GetMaxSeq` → 比本地 `syncedMaxSeq` → 按 seq 拉取；本地持久化用 IndexedDB（Web 侧对应 OpenIM SQLite）。

## Decisions

| 项 | 选择 |
|---|---|
| 交付节奏 | 独立 spec（5 项 P0 拆开，本轮只做 MsgSyncer） |
| 本地存储 | IndexedDB（会话游标 + 消息） |
| 触发 | 登录成功 + WS `onopen` / 重连成功 |
| 服务端接口 | 同时加 `GetMaxSeq` 与 `GetConversationsHasReadAndMaxSeq` |
| hasRead 用法 | 写入 IndexedDB；**不**校准会话列表未读 UI（留给「未读模型」spec） |
| 实现路线 | 方案 1：复用现有 `GetMessagesBySeq`，不新增 PullByRange；不改 conversation 扛 seq |

## Non-goals（本轮）

- 会话列表未读 = `max_seq - read_seq` 的服务端/UI 打通
- 网关主动下发 `sync` 帧
- `PullMessageBySeqs` range RPC
- 群发成员服务端解析、撤回推送、已读回执推送
- `suim-sdk-core`（Go）MsgSyncer

## Architecture

```text
登录成功 / WS onopen
        │
        ▼
   MsgSyncer.run()
        │
        ├─1─ LoadSeq：IndexedDB 读每会话 localMaxSeq
        │
        ├─2─ 并行 GET GetMaxSeq + HasReadAndMaxSeq
        │         → serverMaxSeqs；HasRead 写 IDB hasReadSeq（不改 UI 未读）
        │
        ├─3─ 对每个 conv：若 serverMax > localMax
        │         seqs = [localMax+1 … serverMax]（分批，默认 ≤100）
        │         GET /messages/by-seq → upsert IndexedDB messages
        │         更新 localMaxSeq = 本批已成功落库的最大 seq
        │
        └─4─ emit sync.completed；活跃会话 merge 进 ChatContext
```

| 层 | 职责 |
|---|---|
| `services/message` | 新增 `GetMaxSeq` / `GetConversationsHasReadAndMaxSeq`，读 `seq_user` |
| `services/apigateway` | HTTP 暴露上述接口；`GET /messages/by-seq` 已有 |
| `frontend/src/suim-sdk` | IndexedDB + `MsgSyncer`；login / WS connected 触发 |
| `ChatContext` | 订阅 sync 结果；活跃会话 merge；本轮不改 unread 公式 |

## IndexedDB

**库名：** `suim-im-${userId}`（按用户分库，避免切换账号串数据）。

| Store | Key | 字段 | 用途 |
|---|---|---|---|
| `conversations` | `conversationId` | `maxSeq`, `minSeq`, `hasReadSeq`, `updatedAt` | 本地游标（对齐 OpenIM `LocalConversation` 的 seq 字段） |
| `messages` | `clientMsgId` | 完整消息字段 + `conversationId`, `seq` | 对齐 `LocalChatLog` |

**索引：** `messages` 上 `[conversationId+seq]`，用于按会话有序扫描与缺口检测。

**写入规则：**

- by-seq 拉取 / WS `message.new` / 发送成功 → upsert `messages`
- 同时 `conversations.maxSeq = max(本地, msg.seq)`
- `HasReadAndMaxSeq` 响应 → 更新 `hasReadSeq`（及可选服务端视角的 max，仅作缓存；比对以 `GetMaxSeq` 为准）

## Server APIs

数据源：当前用户的 `seq_user`。鉴权 user 覆盖请求体中的 user 字段。只返回调用者自己的行。

### GetMaxSeq

```text
Req:  { conversation_ids?: string[] }  // 空 = 该用户全部有游标的会话
Resp: { max_seqs: map[string]int64 }   // conversation_id → max_seq
```

- gRPC：`message.GetMaxSeq`
- HTTP：`GET /api/v1/messages/max-seqs`（可选 query：`conversation_ids`）

### GetConversationsHasReadAndMaxSeq

```text
Req:  { conversation_ids?: string[] }  // 空 = 全部
Resp: { seqs: map[string]{ max_seq, has_read_seq } }
```

- gRPC：`message.GetConversationsHasReadAndMaxSeq`
- HTTP：`GET /api/v1/messages/has-read-and-max-seqs`

### Gap pull（已有）

`GET /api/v1/messages/by-seq?conversation_id=&seqs=1,2,3`

客户端将 `[localMax+1, serverMax]` 切批（建议每批 ≤100），顺序请求并写入 IDB。

## SDK MsgSyncer

### 触发

1. `login` / session restore 成功并 `connect()` 后：等 WS `connected === true` 再 `run()`
2. `onStatusChange(false → true)`（重连）再 `run()`
3. 并发控制：同时只跑一个；运行中再触发则设 `pending`，结束后重跑一次

### `run()` 步骤

1. `LoadSeq`：读 IDB `conversations` → `localMaxSeqs`
2. 并行请求 `GetMaxSeq` + `HasReadAndMaxSeq`
3. 写回 IDB `hasReadSeq`（不改 React `unreadCount`）
4. 对每个 `serverMax > localMax` 的会话：分批 by-seq → upsert → 推进 `localMaxSeq`
5. Emit `sync.completed`（可带变更过的 `conversationIds`）；若某会话是当前活跃会话，按 `clientMsgId` / `seq` 去重 merge 进内存消息列表

### 限流与错误

- 单会话单次 sync 最多 N 批（建议 5×100 = 500 seq）；仍落后则保留已推进的 `localMax`，下次重连继续
- 单批失败：指数退避重试 2 次；仍失败则跳过该会话并打日志，不阻断其它会话

### 与现有路径

| 路径 | 行为 |
|---|---|
| 进会话 | 优先读 IDB 该会话消息；不足再 `getAdvancedHistoryMessageList`；结果回写 IDB |
| `message.new` | 照常更新内存；并 upsert IDB、推进 `maxSeq` |
| 发送成功 | 同样写 IDB |

## OpenIM 对照（本轮对齐点）

| OpenIM | SuIM 本轮 |
|---|---|
| 重连：`GetMaxSeq` → `[local+1, serverMax]` pull | 同左；pull 用已有 by-seq |
| 本地 SQLite `LocalChatLog` + `LocalConversation.max_seq` | IndexedDB `messages` + `conversations` |
| `GetConversationsHasReadAndMaxSeq` 校准未读 | 只落 `hasReadSeq`，未读 UI 留给下一 spec |
| `PullMessageBySeqs` range | 不新增；客户端展开 seq 列表 |

## Files（预期改动）

**服务端**

- `proto/message.proto`（及生成代码）
- `services/message/internal/repository/message.go`
- `services/message/internal/service/message.go`
- `services/message/internal/handler/message.go`
- `services/message/internal/types/interfaces/message.go`
- `services/apigateway/internal/handler/message.go`
- `services/apigateway/internal/router/router.go`（若路由集中注册）

**前端 / SDK**

- `frontend/src/suim-sdk/cache/idb.ts`（新建）
- `frontend/src/suim-sdk/modules/sync.ts`（新建 MsgSyncer）
- `frontend/src/suim-sdk/modules/message.ts`
- `frontend/src/suim-sdk/core/rest.ts`
- `frontend/src/suim-sdk/listener/ws.ts` / `client.ts`（触发 sync）
- `frontend/src/contexts/ChatContext.tsx`（merge sync 结果）
- `frontend/src/types/index.ts`（如需 `sync.completed` 事件类型）

## Testing

- 单测（message service）：`GetMaxSeq` / `HasReadAndMaxSeq` 只返回本人 `seq_user`；空 ID 列表 = 全部
- 集成/手工：A 在线收消息后断网；B 继续发；A 重连后无需进会话即可在 IDB/活跃会话看到缺口消息
- 并发：快速断连重连，只应串行完整跑完（含最多一次 pending 重跑）
- 串号：切换账号后 IDB 不得读到上一用户消息

## Follow-ups（后续独立 spec）

1. 未读模型统一：`unread = max_seq - read_seq`；列表以服务端为准（可消费本轮写入的 `hasReadSeq`）
2. 群聊接收方服务端解析成员
3. 撤回后 WS tip
4. 已读回执推对方
