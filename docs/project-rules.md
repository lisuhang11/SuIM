# 项目规范

先写 `services/*/internal/logic`。新代码按下面写。

## 文件

一个 RPC 一个文件。文件名是方法名的 snake_case：`DeleteFriend` 写在 `delete_friend.go`。

`service.go` 放 `Server`、它依赖的接口、`New` 和 `UseXxx`。多个 RPC 共用的辅助逻辑按职责另开文件，例如 `actor.go`、`convert.go`、`notify.go`、`cache.go`。

测试文件与被测文件同名，加 `_test.go`。

## 函数

RPC 实现与 proto 方法同名，挂在 `*Server` 上，签名是 `(ctx, req) (*Resp, error)`。

注入依赖用 `Use` 开头：`UseHooks`、`UseCache`。构造函数用 `New`。

包内函数小写驼峰，用动作命名：`actorID`、`dropFriends`、`freezeHistory`。标识符里的 `ID` 保持全大写：`userID`、`GetFullFriendUserIDs`。

通知、回调沿用现有前缀：`notifyXxx`、`afterXxx`、`webhookBeforeXxx`、`webhookAfterXxx`。

## 注释

导出的类型和函数写一行中文注释，说明结果和边界，不重复函数名。例如谁能读、缺行代表什么、传入 nil 会怎样。

包内函数只在规则看不出来时注释。函数体内只在流程分成几段时用短句标步骤。

返回给调用方的错误信息用英文，短原因放在 `WithMsg` 里。
