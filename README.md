# SuIM · 即时通讯系统

SuIM（**Su** Instant Messaging）是一套面向学习与工程实践的**轻量级即时通讯全栈项目**：自研微服务后端、Web 客户端，以及对齐 OpenIM 风格的 TypeScript / Go SDK。

项目参考 [OpenIM](https://github.com/openimsdk/open-im-server) 的协议与 API 设计，但刻意简化基础设施依赖——**不用 Kafka / MongoDB**，以 MySQL + Redis + MinIO + etcd 支撑完整 IM 主链路，适合理解即时通讯系统如何从账号、关系、会话、消息到实时推送一路打通。

> **当前阶段：** 账号 / 好友 / 黑名单 / 群组 / 会话 / 消息 / 文件 / 在线状态等主路径已接近可用；离线推送、音视频通话、部分 tip 闭环仍在推进。详见 [SuIM vs OpenIM 核心功能差距](docs/SuIM-vs-OpenIM-核心功能差距.md)。

---

## 目录

- [项目亮点](#项目亮点)
- [功能一览](#功能一览)
- [系统架构](#系统架构)
- [技术栈](#技术栈)
- [仓库结构](#仓库结构)
- [微服务说明](#微服务说明)
- [快速开始](#快速开始)
- [开发联调](#开发联调)
- [API 与协议概览](#api-与协议概览)
- [客户端 SDK](#客户端-sdk)
- [数据与基础设施](#数据与基础设施)
- [工程规范](#工程规范)
- [文档索引](#文档索引)
- [Roadmap](#roadmap)
- [与 OpenIM 的定位差异](#与-openim-的定位差异)
- [常见问题](#常见问题)
- [贡献与许可](#贡献与许可)

---

## 项目亮点

| 维度 | 说明 |
|------|------|
| **完整 IM 主链路** | 注册登录 → 好友/群 → 会话 → 发收消息 → WebSocket 在线推送，端到端可跑通 |
| **内建账号体系** | 邮箱注册 / 登录 / JWT，无需像 OpenIM 那样先「导入 IM 用户」 |
| **轻量基础设施** | MySQL 存历史、Redis 做缓存与在线、MinIO 存附件、etcd 做服务发现；无消息队列削峰层 |
| **OpenIM 风格 API** | REST 路径、gRPC 服务拆分、contentType 通知分段、SDK 命名尽量对齐，便于对照学习 |
| **TS Mini SDK** | 前端内嵌 `suim-sdk`：本地优先（内存 + IndexedDB）、MsgSyncer 按 seq 补齐、好友/群增量同步 |
| **BFF 聚合读** | `POST /api/v1/bff/active-conversations` 一次拿到会话 + lastMsg + unread |
| **可观测入口** | apigateway / msggateway 暴露 Prometheus metrics 端口 |

---

## 功能一览

### 已可用（主路径）

| 域 | 能力 |
|----|------|
| **账号** | 邮箱注册 / 登录、JWT access & refresh、改密、注销、用户搜索、批量资料、头像上传、全局免打扰 |
| **好友** | 申请 / 同意 / 拒绝、列表、备注与置顶、删除、version 增量同步 |
| **黑名单** | 拉黑 / 解除 / 列表、双向 `IsBlack`（产品选择：拉黑**保留**好友关系） |
| **群组** | 建群 / 解散 / 转让 / 邀请 / 踢人 / 退群 / 禁言 / 入群申请；成员与入群 version 增量 |
| **会话** | 单聊 / 群聊、置顶、免打扰、BFF 活跃会话列表（lastMsg + unread） |
| **消息** | 文本 / 文件消息、会话 seq、历史拉取、按 seq 补齐（MsgSyncer + IndexedDB）、撤回 / 已读 / 删除 RPC |
| **文件** | MinIO 预签名直传、会话绑定、下载 URL；单文件与用户配额限制 |
| **实时** | WebSocket 长连接、heartbeat / ack、新消息与 tip 在线推送 |
| **在线状态** | Redis 全局在线、WS 订阅好友上下线、REST 批量查询 |
| **通知 tip** | 好友类、在线状态、撤回 / 已读等 contentType 常量体系（部分推送闭环仍在完善） |

### 进行中 / 已知缺口

| 项 | 状态 |
|----|------|
| 离线推送（FCM / APNs / 厂商通道） | `push` 服务为 stub，仅打日志 |
| 撤回 / 已读 tip 可靠下发 + UI | RPC 已有，实时闭环与展示未完全打通 |
| WS 未读实时校准 | BFF 刷新准确；WS 路径仍有启发式 +1 |
| Typing 输入状态 | 前端可发，msggateway 尚未处理业务上行 |
| 会话真增量同步 | `GetIncrementalConversation` 当前行为接近全量 |
| 服务端消息搜索 / Webhook | 未实现 |
| 1v1 音视频（LiveKit） | `rtc` 服务代码已有，尚未接入 compose / apigateway / 前端 |
| Go SDK | 目前仅 user 模块 |

权威差距矩阵与 P0/P1 任务清单见：[docs/SuIM-vs-OpenIM-核心功能差距.md](docs/SuIM-vs-OpenIM-核心功能差距.md)。

---

## 系统架构

```text
┌─────────────────────────────────────────────────────────────┐
│  Client                                                      │
│  Next.js Web  ·  TypeScript suim-sdk  ·  Go suim-sdk-core    │
└───────────────┬─────────────────────────────┬───────────────┘
                │ REST /api/v1/*              │ WebSocket /ws
                ▼                             ▼
         apigateway :9000              msggateway :9001
         (Gin + JWT + BFF)             (在线推送 / presence)
                │                             ▲
                │ gRPC + etcd:///svc          │ OnlinePushMsg
                ▼                             │
    ┌─────────────────────────────────────────┴──────────────┐
    │  Domain Services (gRPC)                                 │
    │  user · relation · group · conversation                 │
    │  message · file · push · rtc(进行中)                    │
    └───────────────────────┬────────────────────────────────┘
                            ▼
              MySQL 8  ·  Redis 7  ·  MinIO  ·  etcd 3.5
```

### 通信约定

| 层级 | 方式 |
|------|------|
| 浏览器 ↔ 网关 | HTTP/JSON（Gin）、WebSocket JSON 帧 |
| 网关 ↔ 域服务 | gRPC + Protobuf |
| 服务发现 | etcd v3；resolver 形如 `etcd:///user` |
| 负载均衡 | gRPC `round_robin` |
| 消息路径 | `message` 同步写 MySQL 后直调 `msggateway.OnlinePushMsg`（**无 Kafka**） |
| 鉴权 | JWT；REST 由 apigateway 校验，WS 连接带 `token` 查询参数 |

---

## 技术栈

| 类别 | 选型 |
|------|------|
| 后端语言 | Go **1.24**（toolchain 1.24.4） |
| RPC | gRPC + Protocol Buffers |
| HTTP 网关 | Gin |
| ORM | GORM → MySQL 8.0 |
| 缓存 / 在线 | Redis 7 |
| 对象存储 | MinIO（预签名 PUT/GET） |
| 服务发现 | etcd 3.5 |
| 日志 | `log/slog` 结构化 JSON |
| 鉴权 | JWT（`golang-jwt/jwt/v5`） |
| 前端 | Next.js 15 · React 19 · TypeScript 5.7 · Tailwind CSS 3.4 · GSAP |
| 音视频（规划） | LiveKit SFU + WebRTC |
| 部署 | Docker Compose |

---

## 仓库结构

```text
SuIM/
├── services/                 # 后端微服务（每服务独立 go.mod）
│   ├── apigateway/           # REST 聚合网关 :9000
│   ├── msggateway/           # WebSocket 网关 :9001
│   ├── user/                 # 账号与用户资料
│   ├── relation/             # 好友 / 黑名单
│   ├── group/                # 群组与成员
│   ├── conversation/         # 会话元数据
│   ├── message/              # 消息与 seq
│   ├── file/                 # 文件元数据 + MinIO
│   ├── push/                 # 离线推送（stub）
│   └── rtc/                  # 1v1 通话（进行中，未进 compose）
├── proto/                    # .proto 定义与生成的 *pb 代码
├── pkg/                      # 跨服务共享库（etcd 发现、notification 常量等）
├── migrations/               # 按域拆分的 SQL 迁移
├── deploy/                   # Docker Compose、Dockerfile、.env.example、init-db.sql
├── frontend/                 # Next.js Web 客户端 + 内嵌 suim-sdk
├── suim-sdk-core/            # Go 客户端 SDK（目前仅 user）
├── scripts/                  # Windows 一键启停（.ps1 / .bat）
├── docs/                     # 设计文档、学习笔记、差距分析
├── CODE_STYLE.md             # Go 后端分层与编码规范
└── go.mod                    # 根模块（供 pkg / proto 被各服务 replace）
```

每个微服务内部遵循统一分层（详见 [CODE_STYLE.md](CODE_STYLE.md)）：

```text
cmd/server/main.go          # 组合根：手动依赖注入
internal/
  handler/                  # gRPC / 传输适配
  service/                  # 业务 + cache-aside
  repository/               # 纯 GORM，不碰 Redis
  cache/                    # Redis key 与助手（可选）
  types/ + interfaces/      # 领域模型与接口契约
  config/ · database/ · errors/ · logger/ · middleware/
etc/*.yaml                  # 本地配置
```

---

## 微服务说明

| 服务 | 默认端口 | 职责 |
|------|----------|------|
| **user** | gRPC `:8080` | 注册登录、JWT、资料、头像、搜索、全局免打扰 |
| **relation** | gRPC `:8081` | 好友全生命周期、黑名单、好友 version 增量 |
| **group** | gRPC `:8082` | 群管理、成员、入群申请、join/member 增量 |
| **conversation** | gRPC `:8083` | 会话元数据、置顶/免打扰、seq 相关、增量接口 |
| **message** | gRPC `:8084` | 发消息、历史、按 seq 拉取、撤回、已读、删除 |
| **push** | gRPC `:8085` | 推送 token 管理；`PushMsg` 待对接真实通道 |
| **file** | gRPC `:8086` | 文件元数据、预签名上传/下载、绑定会话 |
| **rtc** | gRPC `:8087` | 呼叫状态机 + LiveKit JWT（未接入网关/compose） |
| **msggateway** | WS `:9001` · gRPC `:9091` · metrics `:9092` | 长连接、在线推送、在线状态、踢下线 |
| **apigateway** | HTTP `:9000` · metrics `:9090` | REST 聚合、JWT、限流、CORS、BFF |
| **frontend** | `:3000` | Web IM 客户端 |

文件服务 HTTP 细节见：[services/file/README.md](services/file/README.md)。

---

## 快速开始

### 环境要求

- Docker Desktop（或兼容的 Docker Engine + Compose）
- （可选本地开发）Go **1.24+**、Node.js **18+**

### 方式 A：Docker 全栈（推荐）

```bash
cd deploy
cp .env.example .env   # 可按需修改密码与端口
docker compose up -d
```

启动后访问：

| 入口 | 地址 |
|------|------|
| Web 前端 | http://localhost:3000 |
| REST API | http://localhost:9000/api/v1 |
| WebSocket | ws://localhost:9001/ws |
| MinIO Console | http://localhost:10006 |
| etcd | localhost:2379 |
| MySQL | localhost:3306 |
| Redis | localhost:6379 |

常用命令：

```bash
docker compose ps
docker compose logs -f apigateway
docker compose down
```

> **安全提示：** `.env.example` 中的数据库密码、JWT Secret、MinIO 密钥仅供本地开发，**生产环境必须替换**。

### 方式 B：基础设施 Docker + 本地进程（Windows）

适合改后端 / 前端时热重载：

```powershell
# 项目根目录
.\scripts\start-all.ps1              # infra + 后端微服务 + 前端
.\scripts\start-all.ps1 -NoFrontend  # 仅后端
.\scripts\start-all.ps1 -NoDocker    # 假设 MySQL 等已运行
.\scripts\stop-all.ps1               # 停止本地进程
```

也可使用 `scripts\start-all.bat` / `stop-all.bat`。

脚本会启动 user、relation、group、conversation、message、push、msggateway、apigateway；**file / rtc 需按需手动启动**（完整服务列表以 `deploy/docker-compose.yml` 为准）。

### 方式 C：仅基础设施 + 手动起服务

```bash
# 1. 基础设施
docker compose -f deploy/docker-compose.infra.yml up -d

# 2. 按依赖启动各服务（示例）
cd services/user && go run ./cmd/server/
# ... relation / group / conversation / message / file / push / msggateway / apigateway

# 3. 前端
cd frontend
npm install
npm run dev
```

数据库首次初始化：

- Docker MySQL 会挂载执行 `deploy/init-db.sql`
- 分域迁移脚本在 `migrations/*.sql`

---

## 开发联调

### 前端环境变量

| 变量 | 默认 | 说明 |
|------|------|------|
| `NEXT_PUBLIC_API_URL` | `http://localhost:9000/api/v1` | REST 基址 |
| `NEXT_PUBLIC_WS_URL` | `ws://localhost:9001/ws` | WebSocket |
| `NEXT_PUBLIC_MOCK_MODE` | 开发环境默认可为 true | 是否使用前端 mock 数据 |

**联调真实后端时请关闭 mock：**

```bash
# frontend/.env.local 或 shell
NEXT_PUBLIC_MOCK_MODE=false
```

### Proto 生成

各服务目录通常提供 `make proto`（以 user 为例）：

```bash
cd services/user
make proto   # 输出到 ../../proto/userpb
```

根目录 `proto/` 下同时存放 `.proto` 与生成的 Go 包（`userpb`、`relationpb`、`messagepb` 等）。

### 后端配置

各服务读取 `services/<name>/etc/<name>.yaml`，可通过环境变量覆盖（Docker Compose 中已注入 MySQL / Redis / etcd / JWT 等）。

---

## API 与协议概览

### REST（经 apigateway）

统一前缀：`/api/v1`

| 分组 | 示例 |
|------|------|
| `/users` | `POST /register` · `POST /login` · `GET /me` · `GET /batch` · `POST /online-status` |
| `/relations` | 好友申请/列表 · `POST /friends/incremental` · 黑名单 · `GET /is-friend` |
| `/groups` | 建群 · 成员管理 · `POST /:id/quit` · 成员增量 |
| `/conversations` | 列表 · 设置置顶/免打扰 · 增量 · `POST /clear-msg` |
| `/messages` | 发送 · 历史 · `GET /by-seq` · 撤回 · 已读 |
| `/files` | `POST /initiate` · 完成上传 · `GET /:id/download` |
| `/bff` | `POST /active-conversations`（会话 + lastMsg + unread） |

路由实现：`services/apigateway/internal/router/` 与各 `handler`。

### WebSocket

- 连接：`ws://host:9001/ws?token=<JWT>`
- 上行（已支持）：`heartbeat` · `ack` · `presence.subscribe` / `unsubscribe`
- 下行：`push` 帧承载新消息与 tip

更多细节见 `services/msggateway/internal/ws/`。

### Protobuf 服务

| Proto | 生成包 | 服务 |
|-------|--------|------|
| `proto/user.proto` | `proto/userpb` | UserService |
| `proto/relation.proto` | `proto/relationpb` | RelationService |
| `proto/group.proto` | `proto/grouppb` | GroupService |
| `proto/conversation.proto` | `proto/conversationpb` | Conversation |
| `proto/message.proto` | `proto/messagepb` | Message |
| `proto/file.proto` | `proto/filepb` | FileService |
| `proto/push.proto` | `proto/pushpb` | PushMsgService |
| `proto/msggateway.proto` | `proto/msggatewaypb` | MsgGateway |
| `proto/rtc.proto` | `proto/rtcpb` | RtcService |

通知 contentType 分段定义见 `pkg/notification/constant.go`（好友 1000+、在线 1303、通话 1401–1407、撤回 2101、已读 2200 等）。

---

## 客户端 SDK

### TypeScript Mini SDK（Web 主用）

路径：[`frontend/src/suim-sdk`](frontend/src/suim-sdk)  
文档：[frontend/src/suim-sdk/README.md](frontend/src/suim-sdk/README.md)

```ts
import { IMSDK } from "@/suim-sdk";

await IMSDK.login({ username, password });
const me = await IMSDK.getSelfUserInfo();
const friends = await IMSDK.getFriendList();
await IMSDK.sendMessage(/* ... */);
```

| 模块 | 能力摘要 |
|------|----------|
| user / relation / group | OpenIM 风格 REST 封装 |
| conversation / message | 会话与消息；历史、已读等 |
| friend_sync / group_sync / member_sync | version 增量同步 |
| sync（MsgSyncer） | IndexedDB 本地 seq + 缺口补齐 |
| file | 预签名上传、头像、下载 URL |
| presence | 在线状态订阅 |
| listener/ws | 连接管理与事件 |

本地库名形如 `suim-im-${userId}`。UI 层应通过 `@/suim-sdk` 访问能力。

### Go SDK Core

路径：[`suim-sdk-core`](suim-sdk-core)  
文档：[suim-sdk-core/README.md](suim-sdk-core/README.md)

面向原生 / 移动端集成，目录结构参考 `openim-sdk-core`。**当前仅实现 user 模块**（`InitSDK`、`Login` / `LoginWithPassword`、`GetSelfUserInfo`、`SetSelfInfo`、`GetUsersInfo`、`SetUserListener` 等）。

```go
suim_sdk.InitSDK(&sdk_struct.IMConfig{
    ApiAddr: "http://127.0.0.1:9000/api/v1",
    DataDir: "./suim_data",
}, nil)
suim_sdk.LoginWithPassword(cb, "op1", "a@b.com", "password")
```

---

## 数据与基础设施

| 组件 | 用途 |
|------|------|
| **MySQL** | 用户、关系、群、会话、消息、seq、推送 token 等持久化 |
| **Redis** | 用户/群资料 cache-aside；msggateway 在线状态等 |
| **MinIO** | 聊天附件与头像；私有桶 + 预签名 |
| **etcd** | 微服务注册与 gRPC 地址发现 |

Compose 默认凭证（开发用）：

| 项 | 默认值 |
|----|--------|
| MySQL root | `suim123`（见 `DB_ROOT_PASSWORD`） |
| Redis | 见 compose / 服务 yaml（示例常含 `suim-redis`） |
| MinIO | access `suim` / secret `suim-file-secret` |
| JWT | `GATEWAY_JWT_SECRET` / `JWT_SECRET`，示例为 `change-me-in-production` |

消息存储采用「MySQL 映射 OpenIM Mongo seq 模型」的思路：`msg_info` + `seq_conversation` + `seq_user` 等表协同保证会话序号与补齐。

---

## 工程规范

请阅读并遵循：[CODE_STYLE.md](CODE_STYLE.md)

要点摘要：

- Monorepo：每服务独立 `go.mod`，共享 `proto` / `pkg`
- 分层：handler → service interface → repository interface；禁止跨层直连实现
- Repository 只做 GORM；缓存编排放在 service（cache-aside）
- Proto 字段 snake_case；`go_package` 指向 `SuIM/proto/xxxpb`
- 错误使用统一 `AppError`；日志带 request-id
- 风格参照腾讯 [WeKnora](https://github.com/Tencent/WeKnora) 的 Go 后端结构，并适配 gRPC 微服务形态

---

## 文档索引

| 文档 | 说明 |
|------|------|
| [docs/SuIM-vs-OpenIM-核心功能差距.md](docs/SuIM-vs-OpenIM-核心功能差距.md) | 功能矩阵、架构对照、P0/P1 Roadmap |
| [CODE_STYLE.md](CODE_STYLE.md) | 目录分层、缓存、Proto、错误码约定 |
| [docs/OpenIM源码学习.md](docs/OpenIM源码学习.md) | OpenIM 模块与 API 对照笔记 |
| [docs/OpenIM用户模块学习.md](docs/OpenIM用户模块学习.md) | User 域深入 |
| [docs/superpowers/specs/](docs/superpowers/specs/) | 设计规格（SDK、BFF、退群、音视频等） |
| [docs/superpowers/plans/](docs/superpowers/plans/) | 实施计划（MsgSyncer、在线状态、撤回 tip、通话等） |
| [frontend/src/suim-sdk/README.md](frontend/src/suim-sdk/README.md) | TS SDK 模块说明 |
| [suim-sdk-core/README.md](suim-sdk-core/README.md) | Go SDK 用法 |
| [services/file/README.md](services/file/README.md) | 文件服务 API 与生命周期 |

近期设计主题举例：

- [BFF 活跃会话](docs/superpowers/specs/2026-08-01-bff-active-conversations-design.md)
- [退群 QuitGroup](docs/superpowers/specs/2026-08-01-openim-quitgroup-design.md)
- [音视频通话](docs/superpowers/specs/2026-08-02-av-calling-design.md)
- [在线状态计划](docs/superpowers/plans/2026-08-02-online-presence.md)
- [撤回 / 已读 tip](docs/superpowers/plans/2026-08-02-revoke-read-tips.md)

---

## Roadmap

按优先级（摘自差距分析，随开发进展更新）：

**P0 — 消息正确性**

1. 真实离线推送通道（至少一条）
2. 撤回 / 已读 tip 经 msggateway 可靠下发，并打通 SDK + UI
3. WS 未读用 `maxSeq − hasReadSeq` 校准，去掉启发式 +1
4. TS SDK 暴露 revoke / delete / clear 等完整 API

**P1 — 实时与同步质量**

1. Typing 等业务上行帧
2. 会话真增量（version / hash）
3. 在线状态推送与订阅体验打磨

**P2 — 扩展能力**

1. 服务端消息搜索
2. Webhook / 业务回调
3. Go SDK 补齐 friend / group / conversation / message / WS
4. 1v1 音视频 Phase A：LiveKit + rtc 接入 compose 与前端

---

## 与 OpenIM 的定位差异

SuIM **不是** OpenIM 的精简发行版，而是：

1. **学习向 / 工程向自研 IM**：用更少组件跑通同等主链路，源码更易通读。
2. **账号内建**：业务注册登录与 IM 身份一体，降低上手成本。
3. **同步写库 + 直推**：牺牲 Kafka 削峰与 Mongo 历史模型，换取部署简单与路径清晰。
4. **选择性对齐**：好友/群/消息/seq/SDK 命名对齐 OpenIM；管理面（UserCommand、ClientConfig 等）与全量离线推送生态不强求一致。

| 维度 | OpenIM | SuIM |
|------|--------|------|
| 消息落库 | Kafka msgtransfer → Mongo | 同步 gRPC 写 MySQL |
| 历史存储 | MongoDB | MySQL |
| 对象存储 | S3 multipart | MinIO 预签名 |
| 账号 | 外部业务 + IM 用户导入 | 内建 Register / Login / JWT |
| 客户端 | openim-sdk-core 全模块 | TS mini-sdk（较强）+ Go user-only |

---

## 常见问题

**Q: 前端能登录但数据是假的？**  
A: 开发模式可能启用了 mock。设置 `NEXT_PUBLIC_MOCK_MODE=false` 并指向真实 `NEXT_PUBLIC_API_URL` / `WS_URL`。

**Q: Docker 起来了但服务互相找不到？**  
A: 确认 etcd 健康，且各服务配置了 etcd 地址；gRPC 目标应使用 `etcd:///服务名`，不要写死容器 IP。

**Q: 脚本启动后传文件失败？**  
A: `scripts/start-all.ps1` 默认未启动 `file` 服务；请用 Docker Compose 全栈，或额外 `go run` file 服务并保证 MinIO 可用。

**Q: 杀进程后收不到新消息？**  
A: 在线推送依赖 WS；离线通道尚未接通（push stub）。属已知缺口。

**Q: rtc / 通话按钮在哪？**  
A: 后端 `services/rtc` 与设计文档已有，前端与 compose 集成尚未完成，见音视频设计规格。

---

## 贡献与许可

欢迎通过 Issue / PR 讨论设计与补齐缺口；提交代码前请对照 [CODE_STYLE.md](CODE_STYLE.md) 与相关 `docs/superpowers` 规格。

本仓库**暂未放置开源许可证文件**。若你计划公开分发或二次开发，请先与仓库维护者确认许可条款后再使用。

---

**SuIM** — 把即时通讯主链路拆开、读懂、再亲手拼起来。
)
