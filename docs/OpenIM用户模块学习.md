# OpenIM 用户模块学习

> 对照本地源码：`OpenIM/`（Server）与 `OpenIM/openim-sdk-core/`（SDK）。  
> 总目录见 [`OpenIM源码学习.md`](./OpenIM源码学习.md)。

---

## 1. GetSelfUserInfo — 获取当前登录用户信息（含本地缓存）

### 1.1 结论先看

| 问题 | 答案（以源码为准） |
|---|---|
| SDK 入口 | `open_im_sdk.GetSelfUserInfo` |
| 是否每次打服务端？ | **否**。内存 → 本地 SQLite → 才可能 HTTP 拉服务端 |
| 走 msggateway（WS）吗？ | **否**。仅缓存未命中时走 **openim-api** 的 HTTP：`POST /user/get_users_info` |
| 服务端 RPC | `User.GetDesignateUsers` |
| 服务端存储 | Redis（rockscache，key `USER_INFO:{userID}`，TTL 12h）→ 未命中再查 MongoDB |
| 返回给 App | 异步回调 `OnSuccess(json)`，结构是本地模型 `LocalUser` |

本地用户资料通常在**登录后同步**阶段已被 `SyncLoginUserInfo` / `SyncLoginUserInfoWithoutNotice` 写入 SQLite，因此日常调用 `GetSelfUserInfo` 多数只读本地。

---

### 1.2 客户端 SDK 做了什么

#### 调用链（从上到下）

```text
App
  └─ open_im_sdk.GetSelfUserInfo(callback, operationID)
        └─ call(...) 异步 goroutine
              └─ User.GetSelfUserInfo(ctx)
                    └─ User.GetUserInfoWithCache(ctx, loginUserID)
                          └─ UserCache.Fetch(ctx, loginUserID)
```

#### ① 导出层：异步回调包装

文件：`openim-sdk-core/open_im_sdk/user.go`

```go
func GetSelfUserInfo(callback open_im_sdk_callback.Base, operationID string) {
    call(callback, operationID, IMUserContext.User().GetSelfUserInfo)
}
```

`call`（`open_im_sdk/caller.go`）行为：

1. 新开 goroutine，带上 `operationID` 构造 `ctx`
2. 反射调用 `GetSelfUserInfo(ctx)`
3. 成功：`json.Marshal(结果)` → `callback.OnSuccess(string)`
4. 失败：解出 `CodeError` → `callback.OnError(code, msg)`

**注意**：App 拿到的是 JSON 字符串，不是直接对象。

#### ② 业务入口：固定查「当前登录用户」

文件：`openim-sdk-core/internal/user/api.go`

```go
func (u *User) GetSelfUserInfo(ctx context.Context) (*model_struct.LocalUser, error) {
    return u.GetUserInfoWithCache(ctx, u.loginUserID)
}
```

`loginUserID` 在登录 `initialize` 时由 `SetLoginUserID(userID)` 写入（`open_im_sdk/userRelated.go`），因此这里**不会**由调用方传 userID。

#### ③ 三级缓存：`UserCache.Fetch`

文件：

- `internal/user/user.go` — 组装 cache
- `pkg/cache/user_cache.go` — Fetch 逻辑
- `pkg/db/user_model.go` — SQLite 读登录用户

组装时：

```go
u.userCache = cache.NewUserCache(
    func(value *LocalUser) string { return value.UserID },
    nil,                         // batchDBFunc：本路径未用
    u.GetLoginUser,              // singleDBFunc：本地库
    u.GetUsersInfoFromServer,    // queryFunc：HTTP 拉服务端
)
```

`Fetch` 顺序（源码 `user_cache.go`）：

```text
1) 内存 Cache.Load(userID) 命中 → 直接返回
2) 未命中 → fetch():
     a) singleDBFunc = GetLoginUser(ctx, userID)
        → SQLite: WHERE user_id = ? Take
        → 成功则返回（并写回内存 Store）
     b) DB 失败（常见 RecordNotFound）→ queryFunc = GetUsersInfoFromServer
        → HTTP POST /user/get_users_info
        → 转成 LocalUser，写回内存后返回
```

要点：

- **DB 命中时不会再请求服务端**（本 API 路径内不做强制刷新）。
- 服务端拉回后，`Fetch` **只写入内存**，本路径**不会**自动 `InsertLoginUser`；落库靠登录同步 `SyncLoginUserInfo` 的 syncer（Insert/Update）。
- 若服务端也查不到：`ErrUserIDNotFound`。

#### ④ 缓存未命中时：HTTP 拉服务端

文件：`internal/user/server_api.go`、`pkg/api/api.go`、`pkg/network/http_client.go`

```go
// getUsersInfo
req := &user.GetDesignateUsersReq{UserIDs: userIDs}
return api.ExtractField(ctx, api.GetUsersInfo.Invoke, req, (*GetDesignateUsersResp).GetUsersInfo)
```

`api.GetUsersInfo` 绑定路径：`/user/get_users_info`。

`ApiPost` 实际发出：

| 项 | 值 |
|---|---|
| Method | `POST` |
| URL | `{ApiAddr}/user/get_users_info` |
| Header | `Content-Type: application/json` |
| Header | `operationID: {ctx}` |
| Header | `token: {登录 token}` |
| Header | `Accept-Encoding: gzip` |
| Body | `{"userIDs":["当前loginUserID"]}`（字段名以 protocol 序列化为准） |

响应外壳（SDK 解析）：

```json
{
  "errCode": 0,
  "errMsg": "",
  "errDlt": "",
  "data": { /* GetDesignateUsersResp */ }
}
```

`errCode != 0` 时直接当错误返回；`data` 再反序列化为 `GetDesignateUsersResp`，取出 `usersInfo`。

服务端 `UserInfo` → 本地：

```go
// conversion.go ServerUserToLocalUser
LocalUser{
  UserID, Nickname, FaceURL, CreateTime, Ex, GlobalRecvMsgOpt
  // AppMangerLevel / AttachedInfo 未从服务端映射进本地（源码注释掉）
}
```

#### ⑤ 本地库里的「登录用户」表结构

文件：`pkg/db/model_struct/data_model_struct.go`

```go
type LocalUser struct {
    UserID           string // PK
    Nickname         string `json:"nickname"`
    FaceURL          string `json:"faceURL"`
    CreateTime       int64  `json:"createTime"`
    AppMangerLevel   int32  `json:"-"`
    Ex               string `json:"ex"`
    AttachedInfo     string `json:"attachedInfo"`
    GlobalRecvMsgOpt int32  `json:"globalRecvMsgOpt"`
}
```

非 JS 端用 GORM + SQLite（`GetLoginUser`）；Web WASM 走另一套 IndexedDB 实现，接口名相同。

#### ⑥ 本地数据何时写入？（与 GetSelfUserInfo 的关系）

`GetSelfUserInfo` **本身不负责全量同步**。登录后会话模块会触发：

- 初始化同步：`SyncLoginUserInfoWithoutNotice`（`conversation_msg/notification.go`，`AppDataSync` 路径）
- 消息同步开始：`SyncLoginUserInfo`（`syncData` 异步任务）

`SyncLoginUserInfo`（`internal/user/full_sync.go`）：

1. `GetSingleUserFromServer(loginUserID)` → 同上 HTTP `/user/get_users_info`
2. `GetLoginUser` 读本地
3. `userSyncer.Sync(remote, local)`  
   - 无本地 → `InsertLoginUser`  
   - 有差异 → 删内存缓存 + `UpdateLoginUser`，并可能 `OnSelfInfoUpdated` 回调

因此：**先登录同步，再 GetSelfUserInfo，几乎只打本地缓存。**

---

### 1.3 服务端：从 API 网关到存储

> 本节仅描述 **缓存未命中、SDK 真正请求 HTTP** 时的服务端路径。  
> 注意：这里的「网关」是 **openim-api（HTTP API 服务）**，不是 **msggateway（WebSocket 长连接网关）**。

#### 总览

```text
SDK ApiPost
  → openim-api 中间件链
  → POST /user/get_users_info
  → UserApi.GetUsersPublicInfo
  → a2r.Call → gRPC User.GetDesignateUsers
  → userDatabase.Find
  → Redis rockscache (USER_INFO:{id})
       └─ miss → MongoDB user 集合 Find
  → convert.UsersDB2Pb → GetDesignateUsersResp
  → 统一 API 响应 { errCode, errMsg, data }
```

#### ① openim-api 中间件（`internal/api/router.go`）

全局大致顺序（与配置有关）：

1. （可选）Gzip  
2. （可选）RateLimiter  
3. `GinLogger` / Prometheus / `Recovery`  
4. `CorsHandler`  
5. `GinParseOperationID` — 解析请求头 `operationID`  
6. **`GinParseToken`** — 鉴权（本接口**不在白名单**，必须带 token）  
7. `setGinIsAdmin` — 注入管理员 ID 列表  

`GinParseToken` 关键：

1. 读 Header `token`；空则 `ErrArgs`（header must have token）并 Abort  
2. gRPC 调 **auth** 服务 `ParseToken`  
3. 成功则 `c.Set(OpUserID, resp.UserID)`、`c.Set(OpUserPlatform, ...)`  
4. 继续后续 handler  

白名单（无需 token）不含 `/user/get_users_info`，例如只有 `/auth/get_admin_token`、`/auth/parse_token` 等。

#### ② HTTP Handler

文件：`internal/api/user.go`、`router.go`

```go
userRouterGroup.POST("/get_users_info", u.GetUsersPublicInfo)

func (u *UserApi) GetUsersPublicInfo(c *gin.Context) {
    a2r.Call(c, user.UserClient.GetDesignateUsers, u.Client)
}
```

`a2r.Call`：把 HTTP JSON body 绑成 `GetDesignateUsersReq`，调 user RPC，再把 RPC resp 包进统一成功响应写回 gin。

**GetDesignateUsers 本身不做「只能查自己」的权限校验**（与 `UpdateUserInfo` 的 `authverify.CheckAccess` 不同）。任意合法 token 都可在请求里带多个 `userIDs` 批量查询；`GetSelfUserInfo` 只是 SDK 把自己的 `loginUserID` 放进去。

#### ③ user RPC：`GetDesignateUsers`

文件：`internal/rpc/user/user.go`

```go
func (s *userServer) GetDesignateUsers(ctx context.Context, req *pbuser.GetDesignateUsersReq) (*pbuser.GetDesignateUsersResp, error) {
    resp := &pbuser.GetDesignateUsersResp{}
    users, err := s.db.Find(ctx, req.UserIDs)
    if err != nil {
        return nil, err
    }
    resp.UsersInfo = convert.UsersDB2Pb(users)
    return resp, nil
}
```

特点：

- **无 Webhook**（注册/改资料才有 Before/After）  
- **无 Kafka / 推送**  
- 只做「按 ID 查资料 + 结构转换」

注释在 controller：`Find` 时 **userID 找不到也不报错**（与 `FindWithError` 不同），可能返回比请求更短的列表。

#### ④ 存储控制器 → Redis → MongoDB

文件：

- `pkg/common/storage/controller/user.go` — `Find` → `cache.GetUsersInfo`  
- `pkg/common/storage/cache/redis/user.go` — rockscache 批量读  
- `pkg/common/storage/database/mgo/user.go` — Mongo 持久化  

```go
func (u *userDatabase) Find(ctx context.Context, userIDs []string) ([]*model.User, error) {
    return u.cache.GetUsersInfo(ctx, userIDs)
}
```

Redis 层：

```go
func (u *UserCacheRedis) GetUsersInfo(ctx context.Context, userIDs []string) ([]*model.User, error) {
    return batchGetCache2(ctx, u.rcClient, u.expireTime, userIDs, u.getUserInfoKey, u.getUserID, u.userDB.Find)
}
```

- Key：`USER_INFO:{userID}`（`cachekey.GetUserInfoKey`）  
- TTL：`12 * time.Hour`  
- Miss：回调 Mongo `userDB.Find`，再回填 Redis（rockscache）  

Mongo 模型（`pkg/common/storage/model/user.go`）：

```go
type User struct {
    UserID, Nickname, FaceURL, Ex string
    AppMangerLevel, GlobalRecvMsgOpt int32
    CreateTime time.Time
}
```

#### ⑤ 结构转换与返回字段

`convert.UsersDB2Pb` → `[]*sdkws.UserInfo`：

| 字段 | 来源 |
|---|---|
| `userID` | UserID |
| `nickname` | Nickname |
| `faceURL` | FaceURL |
| `ex` | Ex |
| `createTime` | CreateTime.UnixMilli() |
| `appMangerLevel` | AppMangerLevel |
| `globalRecvMsgOpt` | GlobalRecvMsgOpt |

RPC 响应：`GetDesignateUsersResp{ usersInfo: [...] }`  
经 API 包装后，SDK 看到的 HTTP body 形如：

```json
{
  "errCode": 0,
  "errMsg": "",
  "errDlt": "",
  "data": {
    "usersInfo": [
      {
        "userID": "xxx",
        "nickname": "...",
        "faceURL": "...",
        "ex": "...",
        "createTime": 1710000000000,
        "appMangerLevel": 0,
        "globalRecvMsgOpt": 0
      }
    ]
  }
}
```

（具体 JSON tag 以 `github.com/openimsdk/protocol` 生成代码为准。）

---

### 1.4 回到客户端：最终给 App 什么

SDK 在内存/DB/HTTP 任一路径得到 `*LocalUser` 后：

1. `Fetch` 成功则 `Store` 进内存  
2. `call` 将其 `json.Marshal`  
3. `callback.OnSuccess(jsonString)`

App 侧典型 JSON（字段名来自 `LocalUser` 的 json tag）：

```json
{
  "userID": "xxx",
  "nickname": "...",
  "faceURL": "...",
  "createTime": 1710000000000,
  "ex": "...",
  "attachedInfo": "",
  "globalRecvMsgOpt": 0
}
```

`appMangerLevel` 带 `json:"-"`，**不会**出现在回调 JSON 里。

---

### 1.5 时序图（含「多数不打服务端」）

```text
App                    SDK Cache              本地 SQLite         openim-api            auth-rpc          user-rpc         Redis          MongoDB
 |                        |                      |                    |                    |                 |               |              |
 |--GetSelfUserInfo------>|                      |                    |                    |                 |               |              |
 |                        |--Load 内存---------->|                    |                    |                 |               |              |
 |                        |  命中则 OnSuccess-----|                    |                    |                 |               |              |
 |                        |  未命中 GetLoginUser->|                    |                    |                 |               |              |
 |                        |  DB 命中则 Store+成功-|                    |                    |                 |               |              |
 |                        |  DB 未命中 ApiPost---------------------->|                    |                 |               |              |
 |                        |                      |                    |--ParseToken------>|                 |               |              |
 |                        |                      |                    |--GetDesignateUsers----------------->|               |              |
 |                        |                      |                    |                    |                 |--GetUsersInfo->|              |
 |                        |                      |                    |                    |                 |  miss--------->|--Find------->|
 |                        |                      |                    |                    |                 |<--users--------|<--docs-------|
 |                        |                      |                    |<--UsersInfo--------|<----------------|               |              |
 |                        |<--usersInfo----------|<-------------------|                    |                 |               |              |
 |                        | ServerUserToLocalUser + Store 内存        |                    |                 |               |              |
 |<--OnSuccess(LocalUser JSON)-------------------|                    |                    |                 |               |              |
```

---

### 1.6 源码索引

| 步骤 | 路径 |
|---|---|
| SDK 导出 | `openim-sdk-core/open_im_sdk/user.go` |
| 异步 call | `openim-sdk-core/open_im_sdk/caller.go` |
| GetSelfUserInfo | `openim-sdk-core/internal/user/api.go` |
| Cache 组装 / Fetch | `openim-sdk-core/internal/user/user.go`、`pkg/cache/user_cache.go` |
| 本地 DB | `openim-sdk-core/pkg/db/user_model.go` |
| HTTP 封装 | `openim-sdk-core/internal/user/server_api.go`、`pkg/api/api.go`、`pkg/network/http_client.go` |
| 登录后同步 | `openim-sdk-core/internal/user/full_sync.go`、`internal/conversation_msg/notification.go` |
| API 路由/鉴权 | `OpenIM/internal/api/router.go` |
| API Handler | `OpenIM/internal/api/user.go` |
| RPC | `OpenIM/internal/rpc/user/user.go` → `GetDesignateUsers` |
| DB 控制器 | `OpenIM/pkg/common/storage/controller/user.go` |
| Redis | `OpenIM/pkg/common/storage/cache/redis/user.go` |
| Mongo | `OpenIM/pkg/common/storage/database/mgo/user.go` |
| PB 转换 | `OpenIM/pkg/common/convert/user.go` |

---

### 1.7 对 SuIM 的对照提示

| OpenIM | SuIM 现状（对照用） |
|---|---|
| SDK 三级缓存 + 异步回调 | 前端多为 Context/内存 + 直接 REST |
| 查自己走 `/user/get_users_info`（通用批量查询） | 通常有独立「当前用户」接口 |
| 资料读路径无 msggateway | 若只做资料查询，也不必经 WS |
| 服务端 Redis → Mongo | 按 SuIM 自己的缓存/DB 选型对齐即可 |

---

## 后续接口

下一个建议：`SetSelfInfo`（改资料 → UpdateUserInfoEx → Webhook/通知 → SDK 再 Sync），或继续 `GetUsersInfo`（批量他人资料 + 好友/会话联动）。
