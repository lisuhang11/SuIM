# SuIM 项目代码风格与目录规范

> 参考 [WeKnora](https://github.com/Tencent/WeKnora) 项目架构风格制定。

## 一、项目结构

```
SuIM/
├── CODE_STYLE.md              # 本文件
├── Makefile                   # 项目级构建入口
├── go.mod                     # 根模块定义（monorepo 入口）
└── services/
    └── {service_name}/        # 每个微服务一个目录
        ├── cmd/server/main.go # 服务入口
        ├── go.mod             # 服务独立模块
        ├── Makefile           # 服务级构建脚本
        ├── internal/          # 私有代码，不对外暴露
        │   ├── config/        # 配置加载（env-based）
        │   ├── database/      # 数据库初始化（GORM）
        │   ├── errors/        # 统一错误码体系
        │   ├── handler/       # 传输层适配（gRPC handler）
        │   ├── logger/        # 日志封装
        │   ├── middleware/    # gRPC 拦截器
        │   ├── repository/    # 数据访问层（GORM 实现）
        │   ├── service/       # 业务逻辑层
        │   └── types/         # 领域类型 / DTO / 常量
        │       └── interfaces/  # Service / Repository 接口契约
        ├── migrations/        # 数据库迁移脚本（版本化）
        └── proto/             # protobuf 定义 + 生成代码
            └── {service}pb/   # protoc 生成的 Go 代码
```

## 二、分层架构

```
┌─────────────────────────────────────┐
│  proto / handler（传输层）          │  ← proto 类型 ↔ 领域类型转换
├─────────────────────────────────────┤
│  service（业务逻辑层）              │  ← 纯业务逻辑，依赖接口
├─────────────────────────────────────┤
│  repository（数据访问层）           │  ← GORM 实现，满足接口契约
├─────────────────────────────────────┤
│  database（基础设施）               │  ← 连接池、迁移
└─────────────────────────────────────┘
```

**依赖方向（箭头 = 依赖）**：

```
handler → service interface → repository interface
                ↑                    ↑
            service impl        repository impl
```

- **handler** 持有 service 接口引用，不感知实现细节
- **service** 持有 repository 接口引用，不感知存储细节
- **types/interfaces/** 定义所有接口契约
- **禁止上层直接依赖下层实现**（如 handler 直接 import repository）

## 三、命名规范

### 3.1 包名
- 全小写，单数形式，无下划线
- 简短、表意清晰：`handler`、`service`、`repository`、`types`

### 3.2 文件名
- 全小写，下划线分隔：`user.go`、`auth_token.go`

### 3.3 类型 / 函数 / 变量
| 类别 | 规范 | 示例 |
|------|------|------|
| 导出类型（结构体/接口） | PascalCase | `UserService`、`AppError` |
| 私有实现结构体 | camelCase | `userService`、`userRepository` |
| 导出函数 | PascalCase | `NewUserService()`、`NewUserNotFoundError()` |
| 私有函数 | camelCase | `getJwtSecret()`、`userToProto()` |
| 接口命名 | 名词 / 名词 + er | `UserService`、`UserRepository` |
| 构造函数 | `New` + 类型名，返回接口类型 | `func NewUserService(...) interfaces.UserService` |
| 包级 sentinel error | `Err` + 描述 | `ErrUserNotFound`、`ErrTokenNotFound` |
| JSON tag | snake_case | `json:"user_id"` |
| GORM column | snake_case | `gorm:"column:user_id"` |

### 3.4 接收者命名
- 单字母，取自类型名首字母小写
- service: `s`、repository: `r`、handler: `h`

## 四、错误处理

### 4.1 统一 AppError
```go
// internal/errors/errors.go
type ErrorCode int
type AppError struct {
    Code    ErrorCode
    Message string
    Err     error
}
```

### 4.2 错误码分段
| 范围 | 领域 |
|------|------|
| 1000-1099 | 通用错误 |
| 1100-1199 | 认证/授权 |
| 2000-2099 | 用户 |
| 2100-2199 | Token |

### 4.3 错误流
- **repository** 返回 sentinel error（如 `ErrUserNotFound`）
- **service** 将 sentinel error 包装为 `AppError` 返回
- **handler** 将 `AppError` 转为 gRPC status error
- **禁止** service 返回 `(Response{Success: false}, nil)` 模式

## 五、日志

- 基于 `log/slog` 封装，支持 context 传递 request-id
- 所有日志方法接受 `context.Context` 为第一参数
- 格式：`logger.Info(ctx, "message", "key", value)`
- 组件前缀：`[user]`、`[auth]`

## 六、配置

- 通过环境变量加载，`os.Getenv` + 默认值
- Config 结构体集中在 `internal/config/config.go`
- 提供 `DSN()` 等方法构造连接字符串
- 启动时验证关键配置项

## 七、测试

- 测试文件与源文件同目录，命名 `*_test.go`
- 使用标准库 `testing` 包
- 复杂依赖使用接口 mock

## 八、Proto 规范

- 文件放在 `proto/` 目录
- proto package 使用小写、点分隔：`package suim.user`
- go_package 指向生成代码目录
- 生成的 pb 代码放在 `proto/{name}pb/`

## 九、import 分组

按以下顺序，空行分隔：
1. 标准库
2. 第三方库
3. 本项目内部包

```go
import (
    "context"
    "fmt"

    "github.com/google/uuid"
    "gorm.io/gorm"

    "user/internal/types"
    "user/internal/types/interfaces"
)
```

## 十、Git 提交规范

- feat: 新功能
- fix: 修复
- refactor: 重构
- docs: 文档
- style: 代码格式
- test: 测试
- chore: 构建/工具
