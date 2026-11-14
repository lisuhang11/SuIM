# 跟 tdd（一个测试 skill）学测试

在 SuIM 里连续对 `user` / `relation` / `conversation` / `third` / `msg` / `apigateway` / `msggateway` / `rtc` 做过一轮 `/tdd` 之后，体会最深的不是「测试框架怎么用」，而是：**怎样用最小的可执行断言，把「本来应该怎样」钉死，再改实现。**

本文不以教科书式的 TDD 教义为主，而是以 `tdd` skill 为纲，结合这轮真实修 bug 的过程，整理「学测试」时真正有用的几件事。

代码主路径：各服务 `internal/**/**_test.go`；skill 原文：`~/.cursor/skills/tdd/SKILL.md`。

------

## 一、这个 skill 到底在约束什么

Skill 的触发条件写得很克制：

> 只有用户明确要求 TDD / 失败用例 / 回归测试，**或者** bug 有明显、便宜的本地测试靶点时才用；路径不清、昂贵、偏集成、或没人要求时，跳过。

它要做的不是「把覆盖率刷上去」，而是：

> **先把错误行为跑成红灯，再做最小生产修复，最后用同一盏灯证明变绿。**

一句话目标：

> **focused regression test that fails before the fix and passes after it.**

这和「先写一堆表驱动用例再实现功能」的经典 TDD 不完全一样。它更偏 **bug-fix TDD / 回归锁**：

| 经典「先测后写」 | 这个 skill |
|------------------|------------|
| 从需求推导接口 | 从已有坏行为出发 |
| 往往覆盖一整条功能 | 只锁住这一处回归 |
| 容易为仪式造测试 | 明确说：不实用就别硬造 |

学测试时，先接受这个边界：  
**好测试是证据；坏测试是噪音。宁愿没有新测试，也不要坏测试。**

------

## 二、工作流：六步，缺一不可

Skill 把流程压成六步。对照 SuIM 实战，每一步都有具体含义。

### 1. Understand the bug（先说清楚「应该怎样」）

要写下来的不是「代码现在怎么写」，而是四件东西：

1. **意图行为**（合同）
2. **当前行为**（现状）
3. **受影响路径**（哪个函数 / 哪个 HTTP / 哪条 WS）
4. **最小可观察复现**

例：`msg` 的 `SessionType=4`（`sn_` 会话）非通知消息，本应走好友 / 拉黑校验，却掉进 `authorizeSend` 的 `default: return nil`。  
意图清楚了，测试才写得出来；意图不清，测试只会复述实现。

### 2. Choose the narrowest executable check（选最近的那把尺子）

优先顺序大致是：

```text
已有同路径的 unit / component 测试
    → 同包加一条 focused 用例
        → 再才考虑新 harness
            → 再才上 e2e / 浏览器 / 手工
```

SuIM 里已经有大量「假依赖」：`fakeRelations`、`memSocket`、httptest + gRPC stub。  
**学测试的第一课：复用现有 harness，比发明新框架重要。**

### 3. Write the failing test first（测试编码的是意图）

Skill 强调：

> The test should encode **intended behavior**, not mirror the **current implementation**.

对照写：

```go
// 好：编码合同 —— SessionType 4 普通消息必须拒非好友
_, err := srv.SendMsg(..., SessionType: 4, ...)
if code != errorcode.MsgNotFriend { ... }

// 坏：编码实现 —— 断言走到了某个私有分支名 / 某次 mock 调用次数凑巧对了
```

### 4. Run before fixing（红灯必须「红对原因」）

只看到 FAIL 不够。要确认：

- 失败原因就是这个 bug（例如 `code 0 err <nil>`、HTTP `500/1001`）
- 不是编译错、不是 Redis 没起、不是断言写反了

这轮里典型的「红对了」：

| 用例 | 失败信号 |
|------|----------|
| `TestSessionType4RequiresFriends` | 非好友却 `err=<nil>` |
| `TestRequireEitherRejectsForeignEmptyPeer` | `user2=""` 仍 200 |
| `TestSyncGroupsMapsGroupBusinessErrors` | `GroupInvalidArgs` → HTTP 500 / code 1001 |
| `TestAcceptRequiresLiveKit` | LiveKit 未配置 Accept 仍成功 |

红错了就要先改测试或复现，**不要急着改生产代码**。

### 5. Fix the bug（最小生产改动）

只改能满足意图的那一小段，旁路合同尽量不动。

例：

- `authorizeSend`：把 `sessionNotification` 并入单聊校验，而不是重写整条发送链路
- `requireEither`：去掉「空对端放行」，而不是重做全部鉴权中间件
- `sign`：LiveKit 未启用返回错误，与 `GetToken` 对齐，而不是改通话状态机

### 6. Rerun（绿灯 + 邻近验证）

至少：

1. 新回归用例通过  
2. 同包 / 同服务原有测试不红  

Skill 要求最终回复报**证据**，不只报「修好了」：

- 修前：哪个测试、什么失败  
- 修后：哪个命令绿了、还跑了哪些邻近用例  
- 若无法红前：为什么，用了什么替代检查

这也是在训练一种工程习惯：**没有证据的「已修复」不可信。**

------

## 三、什么时候「不要硬写测试」

Skill 单独开了一节：**If a Failing Test Is Impractical**。

不实用的信号包括：

- 要搭很大的 harness / 脆 mock
- 慢 e2e、生产态、复现步骤含糊
- 为了小修引入大量无关 fixture

这时改用「最近的可执行回归」：定向脚本、手工复现命令、浏览器自动化、快照、日志断言、窄集成检查。

并明确：

> Prefer **no new test** over a **bad test**.

坏测试画像：

1. 主要在测 mock，而不是测行为  
2. 把当前实现细节写死进断言  
3. 依赖时序或无关全局状态（易 flaky）  
4. 为小修拉起昂贵基础设施  
5. 证明完就该删的「一次性表演测试」

**学测试的第二课：覆盖率不是目标；可维护的失败信号才是目标。**

------

## 四、护栏：测的是合同，不是实现的脾气

Skill 的 Guardrails 可以记成四条戒律：

1. **不要为了迁就错误实现去改测试**  
2. **不要在行为并未改变时放宽断言**  
3. **回归只锁 bug，别顺手扩一大片无关覆盖**  
4. **flaky 要尽量确定性，并写明锁的是哪一个信号**

若一个 bug 暴露出一类问题：先落地**这一条**回归，再考虑兄弟用例。  
这轮对 apigateway 就是先钉 `requireEither` 空对端，再顺带钉 `user_ids:[""]`；对 msggateway 则是 HTTP 码映射、群推送 fail-closed、同 token 顶号、Accept 空房间几条各自独立落地。

------

## 五、用 SuIM 案例看「测试在教什么」

### 1. 合同 vs 实现细节

`errArgs = 1001` 与 `errorcode.Internal = 1001` 撞号：`fromServiceErr` 把 `InvalidArgument` 收成 `errArgs` 后，HTTP 层再当成 Internal → 500。  

测试锁的是：

> `GroupInvalidArgs` 应对客户端返回 **400 + 业务码 4001**，而不是「某个内部常量叫不叫 errArgs」。

**测试应锁定对外合同（状态码 / 业务码 / 谁被推送），而不是内部枚举别名。**

### 2. Fail closed 比「凑合降级」更可测

群消息成员查询失败时，原逻辑降级成 `SendId/RecvId` 的 1:1 推送——成员丢实时推送，发送方却可能收到「假在线」。  

回归断言很干脆：查找失败时 **sender writes == 0**。  
可测性反过来逼设计选择：**失败时宁可不推，也不要推错人。**

### 3. 空值不是「省略」

`requireEither` 里 `a == "" || b == ""` 直接放行，导致 `user1=别人, user2=""` 能查好友。  
`requireSelf` 允许空 ID「表示省略」时，列表里的 `""` 槽位又不是省略。  

测试逼你分清：

- **字段未出现**（可注入当前用户 / 交给下游）  
- **字段出现但是空串**（无效输入，应拒绝）

### 4. 对称性是好的测试灵感

- 远程同 token 替换走 `replaced`（keepOnline），本地却 `close` → 应对称  
- bot 邀请 dispatch 带 `VoiceID`，`JoinOnAccept` 却不带 → 应对称  
- `GetToken` 拒绝未配置 LiveKit，`Accept` 却返回空凭据成功 → 应对称  

**找「同一合同在 A 路径成立、在 B 路径失效」的裂缝，往往就是最便宜的回归靶点。**

### 5. 红灯证据比「我感觉修好了」重要

Skill 强制输出 fail-before / pass-after。  
这和写文章、做 code review 一样：没有前后对比，修复无法复核。

------

## 六、Go 里测试文件放哪（顺带澄清）

有人会想把 `*_test.go` 收到服务下的 `test/` 目录「更整齐」。在 Go 里这**并不更规范**：

- `go test ./pkg` 只跑**该包目录**内的 `*_test.go`
- 挪到 `services/xxx/test/` 会变成另一个包，未导出符号难测，或只能做外黑盒
- 标准做法是：`foo.go` 旁放 `foo_test.go`；顶层 `tests/e2e` 留给跨服务集成

这轮 `/tdd` 的用例继续放在 `internal/logic`、`internal/ws`、`internal/api` 等同目录，是正确姿势。

------

## 七、一张可复用的检查清单

下次修 bug 或带 `/tdd` 时，可以按这张表过一遍：

```text
[ ] 意图行为一句话说清了吗？
[ ] 有没有现成 harness / 邻近测试可挂？
[ ] 新断言编码的是合同，还是实现细节？
[ ] 修前跑过？失败原因对得上 bug 吗？
[ ] 生产改动是最小的吗？旁路合同还在吗？
[ ] 修后再跑：本用例 + 同包邻近？
[ ] 若写不出便宜测试：有没有更近的可执行检查？还是故意 skip？
[ ] 最终留下了 fail-before / pass-after 证据吗？
```

------

## 八、收束：从这个 skill 里真正学到的

1. **测试是合同的可执行文档**，不是覆盖率仪表盘。  
2. **先红后绿**，是为了证明测试真的咬住了那个 bug，而不是事后补一张「永远绿」的奖状。  
3. **窄、稳、便宜** 三件套：路径近、断言少、依赖少。  
4. **不会写测试时，诚实 skip 并换验证手段**，比硬造坏测试更专业。  
5. **报告证据**：修前失败长什么样、修后哪条命令绿了——这是工程可信度，不是流程表演。

若用一句话概括这个 skill 对「学测试」的贡献：

> **它不教你怎么 mock 全世界，而教你怎么用最小失败信号，证明世界按合同转。**

------

