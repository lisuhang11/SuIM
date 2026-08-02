# OpenIM 源码学习笔记

本地仓库位置：

- Server：`OpenIM/`（open-im-server）
- SDK Core：`OpenIM/openim-sdk-core/`

本文按模块梳理接口与完整调用链路，并对照 SuIM 设计。

---

## 1. User 模块

源码入口：

| 层 | 路径 |
|---|---|
| REST 路由 | `OpenIM/internal/api/router.go`（`/user` 分组） |
| REST Handler | `OpenIM/internal/api/user.go` |
| gRPC 实现 | `OpenIM/internal/rpc/user/` |
| Proto | 外部包 `github.com/openimsdk/protocol/user` |
| SDK | `OpenIM/openim-sdk-core/internal/user/`、`open_im_sdk/user.go` |

### 1.1 REST API（`openim-api`，前缀 `/user`）

对外 HTTP 接口，多数通过 `a2r.Call` 转发到 user RPC。

#### 账号 / 资料

| HTTP 路径 | Handler | 对应 RPC | 说明 |
|---|---|---|---|
| `POST /user/user_register` | `UserRegister` | `UserRegister` | IM 用户注册（导入用户，非业务账号登录） |
| `POST /user/update_user_info` | `UpdateUserInfo` | `UpdateUserInfo` | 更新用户信息（已废弃，建议用 Ex） |
| `POST /user/update_user_info_ex` | `UpdateUserInfoEx` | `UpdateUserInfoEx` | 扩展字段更新用户信息（推荐） |
| `POST /user/set_global_msg_recv_opt` | `SetGlobalRecvMessageOpt` | `SetGlobalRecvMessageOpt` | 设置全局消息接收选项 |
| `POST /user/get_users_info` | `GetUsersPublicInfo` | `GetDesignateUsers` | 按 userID 批量查用户公开信息 |
| `POST /user/get_all_users_uid` | `GetAllUsersID` | `GetAllUserID` | 分页获取全部用户 ID |
| `POST /user/account_check` | `AccountCheck` | `AccountCheck` | 检查账号是否已注册到 IM |
| `POST /user/get_users` | `GetUsers` | `GetPaginationUsers` | 分页查询用户列表 |

#### 在线状态

| HTTP 路径 | Handler | 实际后端 | 说明 |
|---|---|---|---|
| `POST /user/get_users_online_status` | `GetUsersOnlineStatus` | **msggateway**（非 user RPC） | 聚合各网关连接，查在线状态 |
| `POST /user/get_users_online_token_detail` | `GetUsersOnlineTokenDetail` | **msggateway** | 查在线 token / 平台详情 |
| `POST /user/subscribe_users_status` | `SubscriberStatus` | `SubscribeOrCancelUsersStatus` | 订阅/取消订阅用户在线状态 |
| `POST /user/get_users_status` | `GetUserStatus` | `GetUserStatus` | 获取用户在线状态 |
| `POST /user/get_subscribe_users_status` | `GetSubscribeUsersStatus` | `GetSubscribeUsersStatus` | 获取已订阅用户的在线状态 |

#### 用户自定义 Command（通用 KV 扩展）

| HTTP 路径 | Handler | 对应 RPC | 说明 |
|---|---|---|---|
| `POST /user/process_user_command_add` | `ProcessUserCommandAdd` | 同名 | 添加 |
| `POST /user/process_user_command_delete` | `ProcessUserCommandDelete` | 同名 | 删除 |
| `POST /user/process_user_command_update` | `ProcessUserCommandUpdate` | 同名 | 更新 |
| `POST /user/process_user_command_get` | `ProcessUserCommandGet` | 同名 | 按条件获取 |
| `POST /user/process_user_command_get_all` | `ProcessUserCommandGetAll` | 同名 | 获取全部 |

#### 系统通知号

| HTTP 路径 | Handler | 对应 RPC | 说明 |
|---|---|---|---|
| `POST /user/add_notification_account` | `AddNotificationAccount` | 同名 | 添加通知账号 |
| `POST /user/update_notification_account` | `UpdateNotificationAccountInfo` | 同名 | 更新通知账号 |
| `POST /user/search_notification_account` | `SearchNotificationAccount` | 同名 | 搜索通知账号 |

#### 客户端配置

| HTTP 路径 | Handler | 对应 RPC | 说明 |
|---|---|---|---|
| `POST /user/get_user_client_config` | `GetUserClientConfig` | 同名 | 获取用户客户端配置 |
| `POST /user/set_user_client_config` | `SetUserClientConfig` | 同名 | 设置 |
| `POST /user/del_user_client_config` | `DelUserClientConfig` | 同名 | 删除 |
| `POST /user/page_user_client_config` | `PageUserClientConfig` | 同名 | 分页查询 |

#### 统计（挂在 `/statistics`，非 `/user`）

| HTTP 路径 | Handler | 说明 |
|---|---|---|
| `POST /statistics/user/register` | `UserRegisterCount` | 用户注册量统计 |
| `POST /statistics/user/active` | `GetActiveUser`（msg 模块） | 活跃用户统计 |

### 1.2 gRPC（`openim-rpc-user`）— REST 未直接暴露、供内部调用

| RPC 方法 | 所在文件 | 说明 |
|---|---|---|
| `GetGlobalRecvMessageOpt` | `user.go` | 读全局收消息选项（其他服务查询） |
| `GetNotificationAccount` | `user.go` | 按 ID 取通知账号 |
| `SortQuery` | `user.go` | 排序查询 |
| `SetUserStatus` | `online.go` | 设置用户状态（含通知） |
| `SetUserOnlineStatus` | `online.go` | 设置在线平台状态（网关上下线时调用） |
| `GetAllOnlineUsers` | `online.go` | 游标遍历全部在线用户 |

### 1.3 SDK 侧暴露的用户能力（客户端常用子集）

SDK 不会把服务端全部 REST 都暴露给 App，客户端常用的大致是：

| SDK 能力 | 说明 |
|---|---|
| `GetSelfUserInfo` | 获取当前登录用户信息（含本地缓存） |
| `SetSelfInfo` | 修改自己的资料，并同步本地 |
| `GetUsersInfo` | 批量获取用户公开信息（缓存 + 好友补充） |
| `GetUserClientConfig` | 拉取客户端配置 |
| `UserOnlineStatusChange` / Listener | 在线状态变更回调 |

### 1.4 完整流程精读

逐接口的源码级流程见：**[`OpenIM用户模块学习.md`](./OpenIM用户模块学习.md)**。

已完成（OpenIM 源码精读）：

- [x] `GetSelfUserInfo` / `SetSelfInfo` / `GetUsersInfo` / 配置与在线状态 API 清单

### SuIM SDK 落地

已按 openim-sdk-core 结构实现 **`suim-sdk-core` user 模块**（可复制扩展）：

- 路径：`suim-sdk-core/`
- API：`InitSDK` / `Login` / `LoginWithPassword` / `GetSelfUserInfo` / `SetSelfInfo` / `GetUsersInfo` / `SetUserListener`
- 说明见 `suim-sdk-core/README.md`

---

## 后续

继续精读 OpenIM group 等模块；扩展 `suim-sdk-core`（好友 / 群 / 会话）。
