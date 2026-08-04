# suim-sdk-core

SuIM 客户端 SDK 核心（参考 [openim-sdk-core](../OpenIM/openim-sdk-core) 结构）。

当前已实现 **user 模块**（OpenIM 同名 API）：

| API | 说明 | 对接 SuIM |
|---|---|---|
| `InitSDK` | 初始化 ApiAddr / DataDir | — |
| `Login` | userID + token 登录 | 本地库 + Sync |
| `LoginWithPassword` | 邮箱密码登录 | `POST /users/login` |
| `Logout` | 登出 | — |
| `GetSelfUserInfo` | 获取自己（内存→本地→HTTP） | `GET /users/me` |
| `SetSelfInfo` | 更新自己 + 同步本地 | `PUT /users/me` |
| `GetUsersInfo` | 批量公开资料 | `GET /users/batch?ids=` |
| `SetUserListener` | `OnSelfInfoUpdated` 等 | — |

## 目录

```
suim_sdk/            # 对外导出（对齐 open_im_sdk）
suim_sdk_callback/   # 回调接口
internal/user/       # user 业务
pkg/api|network|cache|db|ccontext|sdkerrs
sdk_struct/          # 数据结构
```

## 用法示例

```go
ok := suim_sdk.InitSDK(&sdk_struct.IMConfig{
    ApiAddr: "http://127.0.0.1:9000/api/v1",
    DataDir: "./suim_data",
}, nil)

suim_sdk.SetUserListener(myListener)
suim_sdk.Login(cb, "op1", userID, token)
// 或
suim_sdk.LoginWithPassword(cb, "op1", "a@b.com", "password")

suim_sdk.GetSelfUserInfo(cb, "op2")
suim_sdk.SetSelfInfo(cb, "op3", `{"nickname":"新昵称"}`)
suim_sdk.GetUsersInfo(cb, "op4", `["uid1","uid2"]`)
```

`ApiAddr` 需包含网关前缀，例如 `http://host:9000/api/v1`。
