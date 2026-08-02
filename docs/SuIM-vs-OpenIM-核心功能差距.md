# SuIM vs OpenIM · 核心功能差距分析

**日期：** 2026-08-01（刷新；基线 2026-07-31）  
**对照源：**

| 侧 | 路径 |
|---|---|
| OpenIM Server | `OpenIM/`（open-im-server） |
| OpenIM SDK | `OpenIM/openim-sdk-core/` |
| OpenIM Protocol | `OpenIM/protocol/` |
| SuIM | `services/*`、`proto/*`、`frontend/src/suim-sdk`、`suim-sdk-core` |

交互版看板：Cursor Canvas [`suim-vs-openim-core-gap.canvas.tsx`](../../.cursor/projects/c-Users-Desktop-SuIM/canvases/suim-vs-openim-core-gap.canvas.tsx)（可在聊天旁打开）。

---

## 1. 总判断

SuIM 在 **账号 + 好友/黑名单 + 群基础（含退群）+ 会话元数据 + 发收消息 + seq 补齐（MsgSyncer）+ 好友/群成员增量同步 + BFF 会话列表** 主链路上已接近可用 IM。

与 OpenIM 的**核心差距**不在「有没有会话/消息表」，而在：

1. **离线推送真实通道**（push 仍为 stub）
2. **实时 tip 闭环**（已读回执 / 撤回经 msggateway 可靠下发）
3. **未读实时校准**（BFF 已按 max−read；WS 仍 +1）
4. **WS 上行协议过窄**（网关只处理 heartbeat/ack）
5. **会话增量同步质量**（conversation incremental≈全量；好友/群增量已对齐）
6. **SDK 本地优先**（TS 半套较强；Go SDK 仅 user）

---

## 2. 自 7/31 以来的进展

| 项 | 7/31 | 现在 |
|---|---|---|
| 好友 version 增量 | 缺口 | **对齐**（server + TS `friend_sync`） |
| 群成员 / 入群 version 增量 | 缺口 | **对齐**（join/member version + TS sync） |
| 退群 QuitGroup | — | **对齐**（后端 + ChatHeader） |
| BFF 活跃会话 + unread | — / 部分 | **部分↑**（刷新准；WS 实时仍 +1） |
| MsgSyncer | 规划中 | **部分↑**（TS + IDB 已落地） |
| 手动 clear-msg | 缺口 | **部分↑**（RPC 有；无 cron） |
| 离线推送 / WS 上行 / Go SDK / 搜消息 / Webhook | 缺口 | **仍缺口** |

---

## 3. 架构对照

```text
OpenIM:
  Client → REST API / WS msggateway
        → RPC (auth/user/relation/group/msg/conversation/third)
        → Kafka msgtransfer → Redis seq + Mongo
        → Push → online (msggateway) / offline (FCM|Getui|JPush)

SuIM:
  Client → apigateway REST + msggateway WS
        → domain gRPC (user/relation/group/conversation/message/file/push)
        → MySQL + Redis + MinIO
        → 发信路径直调 msggateway OnlinePush（无 Kafka transfer）
```

| 维度 | OpenIM | SuIM |
|---|---|---|
| 削峰 / 异步落库 | Kafka msgtransfer | 同步服务内写库+推送 |
| 历史存储 | Mongo | MySQL |
| 对象存储 | S3 multipart（third） | MinIO（file） |
| 账号 | IM 用户导入 + 外部业务账号 | 内建 Register/Login/JWT |
| 客户端核心 | openim-sdk-core 全模块 SQLite/WASM | TS mini-sdk（较强）+ Go user-only |

---

## 4. 模块级差距矩阵

图例：**对齐** = 主路径可用；**部分** = API/字段有但行为未闭环；**缺口** = 无实现或 stub；**非核心** = OpenIM 有但非聊天必需。

### 4.1 消息正确性（P0）

| 能力 | 状态 | OpenIM | SuIM | 差距 |
|---|---|---|---|---|
| 发消息 + 在线推送 | 对齐 | SendMsg → transfer → push | SendMsg → OnlinePush | 无队列，可工作 |
| Seq / 重连补齐 | 部分 | PullBySeqs + SDK syncer | GetMaxSeq + by-seq + TS MsgSyncer/IDB | 无 PullByRange；Go 无 syncer |
| 离线推送 | **缺口** | FCM/Getui/JPush | `PushMsg` TODO 打日志 | 杀进程收不到 |
| 已读回执推送 | 部分 | MarkAsRead + Receipt tip | Mark API 有；不走 dispatchPush | 对端无实时已读 |
| 撤回推送 + UI | 部分 | Revoke + tip + SDK | Revoke RPC 有；tip/SDK/UI 缺 | 对端不刷新 |
| 未读 = max−read | 部分 | 列表 unread | BFF 已算；WS +1 | 实时不准 |

### 4.2 实时交互（P1）

| 能力 | 状态 | 差距 |
|---|---|---|
| Typing / 输入状态 | **缺口** | 前端发 typing；msggateway 忽略业务上行帧 |
| 在线状态订阅推送 | 部分 | 可查 OnlineStatus；无 Subscribe + 变更推送 |
| WS 上行（seq/pull/send/signal） | **缺口** | 仅 heartbeat/ack；业务全走 REST |

### 4.3 会话（P0–P2）

| 能力 | 状态 | 差距 |
|---|---|---|
| 置顶 / 免打扰 | 对齐 | — |
| BFF 活跃会话 | 对齐（读路径） | 聚合 lastMsg/remark/unread |
| GetIncrementalConversation | 部分 | 当前实现≈全量，非 version 增量 |
| 字段：private/burn/destruct | 部分 | 手动 clear-msg 有；无 cron、不删消息体 |
| 草稿 / 隐藏会话 | 缺口（SDK） | OpenIM 本地草稿；SuIM 无 |

### 4.4 好友 / 黑名单（P0）

| 能力 | 状态 | 差距 |
|---|---|---|
| 申请/同意/列表/备注置顶 | 对齐 | — |
| 黑名单 | 对齐 | SuIM 拉黑保留好友关系（产品选择） |
| 好友 version 增量 | **对齐** | server + TS `friend_sync` |

### 4.5 群组（P0–P1）

| 能力 | 状态 | 差距 |
|---|---|---|
| 建群/解散/转让/邀请踢人/禁言/申请/退群 | 对齐 | — |
| 成员 / 入群 version 增量 | **对齐** | join/member version + TS sync |
| 群公告等展示字段 | 部分 | 有 notification 字段；产品化弱于 OpenIM |

### 4.6 用户 / 文件 / 消息类型 / 扩展

| 能力 | 状态 | 差距 |
|---|---|---|
| 登录资料 / 批量查询 / 全局免打扰 | 对齐 | SuIM 自带账号是优势 |
| 通知账号 / UserCommand / ClientConfig | 非核心 | 可不对齐 |
| 上传附件 / 头像 | 对齐 | 无 multipart 分片、无日志收集 API |
| Text + File 消息 | 对齐（主路径） | — |
| Quote / At / Merge / Card… | 部分 | 字段有，UI/一等公民 API 弱 |
| 服务端消息搜索 | **缺口** | OpenIM `search_msg` |
| Webhook / 业务回调 | **缺口** | 难接外部业务 |
| Cron 清理（S3/消息销毁） | 部分 | 手动 RPC 有；任务无 |

### 4.7 SDK

| 能力 | 状态 | 差距 |
|---|---|---|
| TS `suim-sdk` | 部分 | REST + MsgSyncer + friend/group sync；缺 revoke/delete、完整 listener |
| Go `suim-sdk-core` | **缺口** | 仅 user；缺 friend/group/conversation/message/WS |

---

## 5. 接下来要做的任务清单

### P0 — 消息正确性闭环（优先）

1. **真实离线推送**：对接至少一条通道（FCM 或国内厂商）；打通 token 注册 → `PushMsg`
2. **撤回 / 已读 tip**：`RevokeMsg` & `MarkAsRead` 后经 msggateway 可靠下发；WS 事件归一化（`message.revoke` / `message.read`）
3. **未读闭环**：WS 新消息用 `maxSeq − hasReadSeq` 刷新列表，去掉 +1 启发式
4. **TS SDK + UI**：暴露 revoke / delete / clear；气泡支持撤回展示

### P1 — 实时与同步质量

1. msggateway 处理 **typing / online-subscribe** 上行并转发
2. **在线状态变更推送**（连接上下线 → 订阅者）
3. **会话真增量**：`GetIncrementalConversation` 按 version 返回 insert/update/delete
4. 扩展 **suim-sdk-core**：friend → group → conversation/message + WS

### P2+ — 产品完整度

1. Quote / At 消息类型产品化  
2. 服务端搜消息 `SearchMessage`  
3. 阅后即焚 / 定时销毁 cron + 真实清消息体  
4. Webhook 框架  
5. 本地草稿 / 隐藏会话  

---

## 6. 刻意不必对齐

- OpenIM「仅导入 IM 用户」模型（SuIM 已内建账号）
- UserCommand / ClientConfig / NotificationAccount（非聊天主路径）
- 必须上 Kafka msgtransfer（可用队列演进，非功能前提）
- 全套厂商推送并行接入
- Prometheus discovery / 配置中心管理面

---

## 7. 证据索引

| 主题 | 路径 |
|---|---|
| OpenIM REST 全表 | `OpenIM/internal/api/router.go` |
| OpenIM WS 协议 | `OpenIM/internal/msggateway/constant.go` |
| OpenIM 推送 | `OpenIM/internal/push/` |
| OpenIM SDK 导出 | `OpenIM/openim-sdk-core/open_im_sdk/*.go` |
| SuIM 离线推送 stub | `services/push/internal/service/push.go` |
| SuIM WS 仅 heartbeat/ack | `services/msggateway/internal/ws/server.go` |
| SuIM 好友增量 | `services/relation/internal/service/friend_sync.go` |
| SuIM 群增量 | `services/group/internal/service/join_sync.go`、`member_sync.go` |
| SuIM BFF 会话 | `services/apigateway/internal/handler/bff.go` |
| SuIM 会话 incremental≈全量 | `services/conversation/internal/service/conversation.go` |
| TS MsgSyncer | `frontend/src/suim-sdk/modules/sync.ts` |
| Go SDK 范围 | `suim-sdk-core/README.md` |
