# SuIM 项目代码风格与目录规范

> 本规范严格参照 [WeKnora](https://github.com/Tencent/WeKnora)（腾讯）的 Go 后端架构风格制定。
> 以下每一条规则都以 WeKnora 的真实源码为依据，并针对本项目的 monorepo + gRPC 形态做了落地适配。

---

## 一、项目结构

本项目是 **monorepo**：根目录用 `go.work` 聚合多个独立模块（每个微服务一个 `go.mod`）。WeKnora 是单模块应用，其后端分层完全适用于本项目的单个服务内部。

```
SuIM/                              # 仓库根（go.work 聚合）
├── CODE_STYLE.md                  # 本文件
├── go.work                        # 多模块聚合
├── Makefile                       # 项目级构建入口（可选）
├── migrations/                    # 数据库迁移脚本（版本化，按服务分文件）
│   └── relation.sql
├── proto/                         # protobuf 定义 + 生成的 Go 代码
│   ├── relation.proto
│   └── relationpb/                # protoc 生成代码（go_package 指向此目录）
└── services/
    └── {service_name}/            # 每个微服务一个独立 Go module
        ├── cmd/server/main.go     # 组合根（依赖装配入口）
        ├── go.mod                 # 服务独立模块
        ├── Makefile               # 服务级构建脚本
        └── internal/              # 私有代码，不对外暴露
            ├── config/            # 配置加载（环境变量 based）
            ├── database/          # 数据库初始化（GORM 连接池 + AutoMigrate）
            ├── errors/            # 统一错误类型（AppError）与构造器
            ├── handler/           # 传输层适配（gRPC handler）
            ├── logger/            # 结构化日志封装（slog，携带 request-id）
            ├── middleware/        # gRPC 拦截器（recovery / 日志 / 链路）
            ├── repository/        # 数据访问层（GORM 实现，按领域聚合）
            ├── service/           # 业务逻辑层
            └── types/             # 领域类型（GORM 模型）
                └── interfaces/    # Service / Repository 接口契约
```

**WeKnora 对应结构参考**：WeKnora 使用 `internal/application/service`、`internal/application/repository`、`internal/handler`、`internal/types`、`internal/types/interfaces`、`internal/config`、`internal/middleware`、`internal/errors`、`internal/logger`、`internal/container`（uber/dig 做 DI）。本项目将 `application/` 下的 `service`、`repository` 扁平化到 `internal/` 下（单服务模块，无需 `application` 这一层），语义与 WeKnora 完全一致。

---

## 二、分层架构与依赖方向

```
┌─────────────────────────────────────────┐
│  proto / handler（传输层）               │  ← proto 类型 ↔ 领域类型转换
├─────────────────────────────────────────┤
│  service（业务逻辑层）                   │  ← 纯业务逻辑，依赖接口
├─────────────────────────────────────────┤
│  repository（数据访问层）                │  ← GORM 实现，满足接口契约
├─────────────────────────────────────────┤
│  database（基础设施 / config / logger）  │  ← 连接池、迁移、配置
└─────────────────────────────────────────┘
```

**依赖方向（箭头 = 依赖）**：

```
handler ──▶ service interface ──▶ repository interface
   │              ↑                     ↑
   │         service impl          repository impl
   └──────────────┴─────────────────────┘
           全部经由 types/interfaces/ 做依赖倒置
```

- **handler** 只持有 `interfaces.XxxService`，不感知实现细节，也不直接 import repository。
- **service** 只持有 `interfaces.XxxRepository`，不感知存储细节（几张表、什么 SQL 全封在 repository 内）。
- **types/interfaces/** 定义所有接口契约，是各层之间的唯一耦合点。
- **禁止上层直接依赖下层实现**（如 handler 直接 import `repository` 包）。
- 依赖装配（wire）集中在 `cmd/server/main.go` 组合根，不使用 DI 框架，全部手动构造函数注入（与 WeKnora 一致）。

---

## 三、Repository 设计（最重要：按领域聚合，不按表拆分）

> 这是本项目早期版本踩过的坑，也是与 WeKnora 风格差异最大的一点。

**WeKnora 的原则**：`repository` 包按**业务领域**组织，一个领域一个文件、一个 `xxxRepository` 结构体，它内部可以操作多张表。绝不是「一张表一个 repository」。

WeKnora 实例：`internal/application/repository/knowledge.go` 里只有一个 `knowledgeRepository struct { db *gorm.DB }`，它同时管理 `knowledges`、`knowledge_bases`、`knowledge_tags` 等多张表。

❌ **错误（按表拆分，存储结构泄漏到 service 层）**：

```go
// 早期 relation 服务的错误写法：3 个 repo，service 被迫感知 3 张表
type FriendRequestRepository interface { ... }
type FriendRepository interface { ... }
type BlackRepository interface { ... }
// service 构造函数要注入 3 个 repo，且到处编排跨表逻辑
```

✅ **正确（按领域聚合为单个 Repository）**：

```go
// types/interfaces/relation.go
type RelationRepository interface {
    // —— 好友请求 ——
    CreateFriendRequest(ctx context.Context, req *types.FriendRequest) error
    GetFriendRequest(ctx context.Context, from, to string) (*types.FriendRequest, error)
    GetPendingBetween(ctx context.Context, a, b string) (*types.FriendRequest, error)
    UpdateFriendRequestStatus(ctx context.Context, from, to string, status types.FriendRequestHandleResult) error
    ListIncomingRequests(ctx context.Context, user string, offset, limit int) ([]*types.FriendRequest, int64, error)
    ListOutgoingRequests(ctx context.Context, user string, offset, limit int) ([]*types.FriendRequest, int64, error)
    // —— 好友 ——
    CreateFriend(ctx context.Context, f *types.Friend) error
    DeleteFriendPair(ctx context.Context, a, b string) error
    FriendExists(ctx context.Context, owner, friend string) (bool, error)
    ListFriends(ctx context.Context, owner string, offset, limit int) ([]*types.Friend, int64, error)
    // —— 拉黑 ——
    CreateBlock(ctx context.Context, b *types.Black) error
    DeleteBlock(ctx context.Context, owner, blocked string) error
    BlockExists(ctx context.Context, owner, blocked string) (bool, error)
    ListBlocks(ctx context.Context, owner string, offset, limit int) ([]*types.Black, int64, error)
    FindBlock(ctx context.Context, owner, target string) (*types.Black, error)
}
```

实现侧把同一 `relationRepository` 类型的方法**按子域分散到多个文件**（与 WeKnora 把多张表收敛进一个 repo 一致）。以 relation 服务为例，`repository/` 目录如下，三个文件都定义 `*relationRepository` 的方法：

```
repository/
├── relation.go          # 结构体定义 + 包级 sentinel error + 构造函数
├── relation_request.go  # 好友请求相关方法
├── relation_friend.go   # 好友相关方法
└── relation_block.go    # 拉黑相关方法
```

`relation.go`（结构体、sentinel、构造函数）：

```go
var (
    ErrFriendRequestNotFound = errors.New("friend request not found")
    ErrFriendNotFound        = errors.New("friend not found")
    ErrBlackNotFound         = errors.New("black record not found")
)

type relationRepository struct { db *gorm.DB }

func NewRelationRepository(db *gorm.DB) interfaces.RelationRepository {
    return &relationRepository{db: db}
}
```

`relation_request.go`（同一结构体的部分方法，附共享 helper）：

```go
func (r *relationRepository) CreateFriendRequest(ctx context.Context, req *types.FriendRequest) error {
    return r.db.WithContext(ctx).Create(req).Error
}

// listFriendRequests 是分页查询的共享 helper，避免 incoming/outgoing 重复。
func listFriendRequests(ctx context.Context, db *gorm.DB, cond string, offset, limit int, args ...any) ([]*types.FriendRequest, int64, error) {
    var reqs []*types.FriendRequest
    var total int64
    base := db.WithContext(ctx).Model(&types.FriendRequest{}).Where(cond, args...)
    if err := base.Count(&total).Error; err != nil {
        return nil, 0, err
    }
    query := base.Order("create_time DESC")
    if limit > 0 { query = query.Limit(limit) }
    if offset > 0 { query = query.Offset(offset) }
    if err := query.Find(&reqs).Error; err != nil {
        return nil, 0, err
    }
    return reqs, total, nil
}
```

> 要点：所有方法 Receiver 都是 `(r *relationRepository)`；每个查询链式 `.WithContext(ctx)`；列表查询返回 `(*[]T, int64, error)`（int64 为总数）；not-found 用 `errors.Is(err, gorm.ErrRecordNotFound)` 转成包级 sentinel。

`service` 构造函数只需注入 **1 个** `interfaces.RelationRepository`，跨表编排（如「拉黑时删除双向好友」）完全封在 repository / service 内部，对上层透明。

---

## 四、接口约定（interfaces 包）

- 每个领域在 `types/interfaces/{domain}.go` 中同时声明 `XxxService` 与 `XxxRepository` 两个接口（WeKnora 习惯把同一领域的 service 和 repository 接口放在同一文件）。
- **构造函数一律返回接口类型**，而非具体结构体：

  ```go
  func NewRelationService(repo interfaces.RelationRepository, ...) interfaces.RelationService
  func NewRelationRepository(db *gorm.DB) interfaces.RelationRepository
  ```

- 接口方法按**功能分组**（CRUD / 查询 / 生命周期 / 异步），用空行分隔，分组前可加简短注释。WeKnora 实例：`KnowledgeService` 内部分为 Ingestion / Retrieval / Listing / Lifecycle / FAQ / Tagging / Async / Progress / Search 等组。
- 接口级别注释一句话说明职责；方法级别注释说明**语义、状态机、并发保证、错误条件**（而非「做什么」），WeKnora 甚至会引用内部 PR / issue 号。

---

## 五、命名规范

### 5.1 包名
- 全小写，单数，无下划线：`handler`、`service`、`repository`、`types`、`config`、`logger`、`errors`、`middleware`、`database`。
- 与目录名一致。

### 5.2 文件名
- 全小写，下划线分隔；按领域 / 子域拆分：`relation.go`、`relation_friend.go`、`relation_block.go`、`knowledge.go`。

### 5.3 类型 / 函数 / 变量
| 类别 | 规范 | 示例 |
|------|------|------|
| 导出类型（结构体/接口） | PascalCase | `RelationService`、`RelationRepository`、`AppError` |
| 私有实现结构体 | camelCase | `relationService`、`relationRepository` |
| 导出函数 | PascalCase | `NewRelationService()`、`NewRelationNotFoundError()` |
| 私有函数 | camelCase | `friendRequestToProto()`、`parseRequestID()` |
| 接口命名 | 名词 / 名词 + er | `RelationService`、`RelationRepository` |
| 构造函数 | `New` + 类型名，返回接口 | `func NewRelationService(...) interfaces.RelationService` |
| 包级 sentinel error | `Err` + 描述 | `ErrFriendRequestNotFound`、`ErrFriendNotFound` |
| JSON tag | snake_case | `json:"user_id"` |
| GORM column | snake_case | `gorm:"column:user_id"` |
| GORM 表名 | 全小写，多词用下划线 | `TableName() => "friend_request"` |

### 5.4 接收者命名
- 单字母，取自类型名首字母小写：`service → s`、`repository → r`、`handler → h`。
- 统一使用**指针接收者** `(s *relationService)`、`(r *relationRepository)`、`(h *relationHandler)`。
- 所有方法 `context.Context` 作为**第一个参数**，透传到 repository / logger。

---

## 六、错误处理

### 6.1 分层错误职责（与 WeKnora 一致）

```
repository  ──▶ 返回包级 sentinel error（如 ErrXxxNotFound）
service     ──▶ 将 sentinel / 领域错误包装为 *AppError（带错误码）
handler     ──▶ 将 *AppError 转为传输层错误（gRPC status / HTTP）
```

- **repository**：用 `var ErrXxx = errors.New("...")` 定义包级 sentinel；通过 `errors.Is(err, gorm.ErrRecordNotFound)` 将 GORM 的 not-found 翻译成 sentinel 返回。
- **service**：`var ErrXxx = repository.ErrXxx` 别名引用，使 `errors.Is` 可跨层匹配；再用 `errors.NewXxxError()` 构造 `*AppError` 向上抛。**service 不得返回 `(Response{Success: false}, nil)` 这种吞错写法**。
- **handler**：通过 switch 错误码，将 `*AppError` 映射为 gRPC `codes.*`（`status.Error`）。

### 6.2 统一 AppError
```go
// internal/errors/errors.go
type ErrorCode int
type AppError struct {
    Code    ErrorCode
    Message string
    Err     error // 底层错误，便于排查；支持 errors.Is/As 透传
}
```
- 提供 `IsAppError(err)` / `GetAppError(err)` 辅助函数。
- 每个错误码提供构造器：`NewValidationError(msg)`、`NewInternalError(msg)`、`NewAlreadyFriendsError()` 等。

### 6.3 错误码分段
| 范围 | 领域 | 示例 |
|------|------|------|
| 1000-1099 | 通用 | 1000 未知 / 1001 参数校验 / 1002 内部 |
| 1100-1199 | 认证 / 授权 | |
| 2000-2099 | 用户 | |
| 2100-2199 | Token | |
| 3000-3099 | 关系（relation） | 3000 关系不存在 / 3001 已是好友 / 3002 已拉黑 / 3003 未拉黑 / 3004 不能加自己 / 3005 好友请求不存在 / 3006 已发起请求 / 3007 请求已处理 / 3008 未授权 |

> 新增领域时，在既有分段后追加，不要改动已有数值（避免客户端缓存的码值错位）。

### 6.4 handler → gRPC status 映射示例
```go
func appErrorToStatus(err error) error {
    ae := apperrors.GetAppError(err)
    if ae == nil {
        return status.Error(codes.Internal, err.Error())
    }
    var code codes.Code
    switch ae.Code {
    case apperrors.CodeValidation, apperrors.CodeCannotFriendSelf:
        code = codes.InvalidArgument
    case apperrors.CodeRelationNotFound, apperrors.CodeFriendRequestNotFound, apperrors.CodeNotBlocked:
        code = codes.NotFound
    case apperrors.CodeAlreadyFriends, apperrors.CodeAlreadyBlocked,
        apperrors.CodeAlreadyRequested, apperrors.CodeRequestAlreadyProcessed:
        code = codes.AlreadyExists
    case apperrors.CodeNotAuthorized:
        code = codes.PermissionDenied
    default:
        code = codes.Internal
    }
    return status.Error(code, ae.Message)
}
```

### 6.5 repository sentinel 与 service 别名
```go
// repository/relation.go
var ErrFriendRequestNotFound = errors.New("friend request not found")

// service/relation.go
var (
    ErrFriendRequestNotFound = repository.ErrFriendRequestNotFound // 别名，供 errors.Is 跨层匹配
)
```

---

## 七、GORM 使用约定（来自 WeKnora 实践）

- **每条查询都链式 `.WithContext(ctx)`**，保证超时 / 取消可穿透到数据库。
- not-found 统一转换：`if errors.Is(err, gorm.ErrRecordNotFound) { return nil, ErrXxxNotFound }`。
- 分页查询返回 `([]*T, int64, error)`，其中 `int64` 为总数（先 `Count` 再 `Find`，复用 baseQuery）。
- 查询排序、过滤用 `Where` / `Order` / `Offset` / `Limit`；列表字段用 `Find`，单行用 `First`。
- 并发安全的更新：用 `gorm.Expr("pending_count - 1")` 做原子递减，避免先读后写覆盖；写操作对易变的计数器 / 时间戳用 `.Omit(...)` 跳过，防止陈旧内存值覆盖并发更新（WeKnora 的 `omitFieldsOnUpdate` 实践）。
- 软删除：查询显式补 `deleted_at IS NULL`（若启用 GORM 软删除）。
- repository 结构体只持有 `*gorm.DB`，不内嵌领域模型；模型定义在 `types` 包。

---

## 八、日志

- 基于 `log/slog` 的封装，所有方法以 `context.Context` 为第一参数，自动携带 request-id（由 middleware 注入）。
- 结构化字段：`logger.Info(ctx, "message", "key", value, "user_id", id)`。
- 级别：`Info` 流程 / `Warn` 软性异常 / `Error` 失败 / `Debug` 调试；错误日志带上 `err`。
- handler 层在调用 service 失败后用 `logger.Error(ctx, "xxx failed", "error", err)` 记录，再转 status 返回。
- 不记录敏感字段；动态值若用于日志需脱敏（WeKnora 用 `SanitizeForLog`）。

---

## 九、配置

- 通过环境变量加载，`os.Getenv` + 默认值；集中在 `internal/config/config.go`。
- 提供 `DSN()` 等方法构造连接串；启动时校验关键配置（端口、数据库地址）。
- 结构体字段用 `mapstructure` / 显式读取，不散落到各处。

---

## 十、注释风格

- **写「为什么」，不写「做什么」**。明显的代码不写注释。
- 函数级注释：单行 `// MethodName action`（如 `// CreateFriendRequest creates a friend request.`）。
- 非显而易见的并发、状态机、边界条件用块注释说明（WeKnora 会对「防并发覆盖计数器」「副本延迟风险」等写详细 why）。
- 接口方法注释说明状态转换语义、幂等性、错误条件。
- 公共 API / 接口必须有文档注释（godoc 可生成）。

---

## 十一、测试

- 测试文件与源文件同目录，命名 `*_test.go`。
- 使用标准库 `testing`；复杂依赖用接口 mock（手写或 mockgen）。
- repository 可用真实 DB（testcontainers / sqlite）或 mock；service 用 mock repository 验证编排逻辑。
- 表驱动测试（table-driven）为主。

---

## 十二、Proto 规范

- 定义文件放在 `proto/{service}.proto`。
- proto package 小写点分隔：`package suim.relation;`
- `go_package` 指向生成代码目录：`option go_package = "SuIM/proto/relationpb";`
- 生成的代码放在 `proto/{name}pb/`，不手写、不提交手动改动（用 `make proto` 重新生成）。
- 请求 / 响应消息字段用 snake_case；枚举表示状态（如 `FriendRequestHandleResult`）。

### 12.1 双向关系查询（OpenIM 风格 `isFriend` / `isBlack`）

关系本质是**单向存储、双向呈现**：好友 / 拉黑在 `friend` / `black` 表里都是「owner → target」的单边记录。对外查询时，不要返回「friend / blocked / none」这种复合字符串（信息有限且掩盖单边异常），而应**把两个方向拆开返回明细**，参考 OpenIM 的 `isFriend` / `isBlack`：

```proto
rpc IsFriend(IsFriendReq) returns (IsFriendResp);
rpc IsBlack(IsBlackReq)   returns (IsBlackResp);

message IsFriendReq {
  string user1 = 1;
  string user2 = 2;
}
message IsFriendResp {
  bool in_user1_friends = 1; // user2 在 user1 好友列表中
  bool in_user2_friends = 2; // user1 在 user2 好友列表中
}
message IsBlackReq {
  string user1 = 1;
  string user2 = 2;
}
message IsBlackResp {
  bool in_user1_blacklist = 1; // user2 在 user1 黑名单中（user1 拉黑了 user2）
  bool in_user2_blacklist = 2; // user1 在 user2 黑名单中（user2 拉黑了 user1）
}
```

落地的工程约定（relation 服务已按此实现）：

- **service 层两个方向独立查询**，不合并、不短路：

  ```go
  func (s *relationService) IsFriend(ctx context.Context, user1, user2 string) (inUser1Friends, inUser2Friends bool, err error) {
      b1, e1 := s.repo.FriendExists(ctx, user1, user2) // user2 ∈ user1 好友
      b2, e2 := s.repo.FriendExists(ctx, user2, user1) // user1 ∈ user2 好友
      if e1 != nil { return false, false, apperrors.NewInternalError("check friend status failed").WithDetails(e1) }
      if e2 != nil { return false, false, apperrors.NewInternalError("check friend status failed").WithDetails(e2) }
      return b1, b2, nil
  }
  ```

- 复用 repository 已有的「单边」原子查询 `FriendExists(owner, friend)` / `BlockExists(owner, blocked)`，**不新增 per-direction 的 repo 方法**；双向只是两次独立调用。
- handler 直接透传两个 bool，不做字符串拼装；错误走统一的 `appErrorToStatus` 转换。
- 用「双边都 true 才算好友 / 拉黑」作为业务判定条件，把单边数据不一致的兜底判断交给调用方。

---

## 十三、import 分组

按以下顺序，空行分隔（与 WeKnora / goimports 一致）：
1. 标准库
2. 第三方库
3. 本服务内部包（`relation/internal/...`）

```go
import (
    "context"
    "errors"

    "google.golang.org/grpc/codes"
    "gorm.io/gorm"

    "relation/internal/types"
    "relation/internal/types/interfaces"
)
```

---

## 十四、Git 提交规范（Conventional Commits）

- `feat:` 新功能
- `fix:` 修复
- `refactor:` 重构（不改变外部行为）
- `docs:` 文档
- `style:` 代码格式
- `test:` 测试
- `chore:` 构建 / 工具 / 依赖

提交信息用中文或英文均可，但同一服务保持一致；涉及接口 / 错误码变更须在描述中说明影响范围。
