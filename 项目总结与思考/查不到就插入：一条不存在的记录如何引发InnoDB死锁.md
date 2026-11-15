# 查不到就插入：一条不存在的记录如何引发 InnoDB 死锁

## 一、从一条线上告警说起

订单创建接口在压测时开始零星报错，大促峰值期间错误率进一步爬升。日志里反复出现同一句：

```
ERROR 1213 (40001): Deadlock found when trying to get lock; try restarting transaction
```

开发同学去看死锁日志，发现一件很反常的事：冲突的两个事务，执行的是**完全相同的 SQL**，查询条件是**同一个订单号**，而且这条 SQL 的返回结果都是 `Empty set`。

一条都查不到的 SQL，怎么会把两个事务锁到互相等待、最终被 InnoDB 强制回滚？

这是 MySQL InnoDB 里最经典的并发陷阱之一。它有个通俗的名字，叫「查不到就插入」。问题的根源不在业务逻辑，而在 InnoDB 的加锁机制——**查不到的数据，也可能被锁住**。

本文用一张订单表，把这个死锁从头到尾推演一遍。

> 前提约定：InnoDB 存储引擎、REPEATABLE READ（可重复读，简称 RR）隔离级别、MySQL 8.0 语境。这三个前提缺一不可，后面会说明换成 READ COMMITTED 会发生什么。

## 二、复现现场：一张订单表

先建表。`order_no` 是业务订单号，同时作为幂等键，上面建了唯一索引：

```sql
CREATE TABLE t_order (
  id        BIGINT        NOT NULL AUTO_INCREMENT,
  order_no  VARCHAR(32)   NOT NULL COMMENT '业务订单号，幂等键',
  user_id   BIGINT        NOT NULL,
  amount    DECIMAL(10,2) NOT NULL,
  status    TINYINT       NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  UNIQUE KEY uk_order_no (order_no)
) ENGINE = InnoDB;
```

插入三条数据。**注意订单号是跳着的**，这个数据分布是后面所有推理的关键：

```sql
INSERT INTO t_order VALUES
 (10, 'NO1001', 1001,  99.00, 1),
 (20, 'NO1003', 1003,  59.00, 1),
 (30, 'NO1005', 1005, 199.00, 0);
```

在唯一索引 `uk_order_no` 上，这三条记录按字典序排列，记录之间形成了若干**间隙（Gap）**：

```
(-∞, NO1001)   [NO1001]   (NO1001, NO1003)   [NO1003]   (NO1003, NO1005)   [NO1005]   (NO1005, +∞)
    间隙         记录           间隙             记录           间隙             记录         间隙
                                                        ↑
                                            NO1004 应该落在这里
```

`NO1004` 在索引上不存在，但它的「位置」是确定的——按字典序，它应该排在 `NO1003` 和 `NO1005` 之间。请记住这个位置。

## 三、那段看起来很对的代码

业务诉求很朴素：前端可能因为网络抖动自动重试，用户也可能狂点提交按钮，后端必须保证同一个 `order_no` 只落一条记录。

最自然的写法是**先锁、再查、后插**：

```sql
BEGIN;

-- 第一步：锁住这个订单号，防止别人同时插
SELECT id FROM t_order
 WHERE order_no = 'NO1004'
 FOR UPDATE;
-- 执行结果：Empty set，一条都没查到

-- 第二步：既然不存在，那就插进去
INSERT INTO t_order(order_no, user_id, amount, status)
VALUES ('NO1004', 1004, 88.00, 0);

COMMIT;
```

换成 Java 代码就是大家熟悉的样子：

```java
@Transactional(rollbackFor = Exception.class)
public void createOrder(String orderNo, Long userId, BigDecimal amount) {
    // 加锁查询，防止并发重复创建
    Order existing = orderMapper.selectForUpdate(orderNo);
    if (existing != null) {
        throw new BizException("订单已存在");
    }
    orderMapper.insert(new Order(orderNo, userId, amount));
}
```

代码评审时，这段逻辑通常挑不出毛病：加了事务、加了行锁、做了存在性判断、有唯一索引兜底。看起来是防御性编程的典范。

## 四、单事务视角：一切正常

单独跑一遍，完全没问题：

1. `SELECT ... FOR UPDATE` 返回 `Empty set`；
2. 判定订单不存在，走插入分支；
3. `INSERT` 成功；
4. `COMMIT`，数据库里多了一条 `NO1004`。

这里藏着一个几乎所有人都会有的**直觉假设**：

> 查询没有返回任何行，那就说明什么也没锁到。

这个假设是错的，而它错得很隐蔽——单元测试测不出来，一个人开着客户端手工点也测不出来。因为整段代码在**单事务串行执行**时，行为完全符合预期。

问题只在一个词上：**同时**。

## 五、把「同时」放进来

现在有两个用户几乎在同一时刻提交订单，`order_no` 都是 `NO1004`。事务 A 和事务 B 各自执行第三节那段代码。

时序如下：

| 时刻 | 事务 A                                          | 事务 B                                          | 说明                                            |
| ---- | ----------------------------------------------- | ----------------------------------------------- | ----------------------------------------------- |
| T1   | `SELECT ... WHERE order_no='NO1004' FOR UPDATE` |                                                 | 返回 Empty set，A 获得间隙锁                    |
| T2   |                                                 | `SELECT ... WHERE order_no='NO1004' FOR UPDATE` | 返回 Empty set，**B 也获得间隙锁，不被阻塞**    |
| T3   | `INSERT ... VALUES('NO1004', ...)`              |                                                 | 需要插入意向锁，被 B 的间隙锁挡住，**开始等待** |
| T4   |                                                 | `INSERT ... VALUES('NO1004', ...)`              | 需要插入意向锁，被 A 的间隙锁挡住，**开始等待** |
| T5   | 死锁                                            | 死锁                                            | InnoDB 检测到循环等待，回滚其中一个             |

T2 是全文最反直觉的一步：A 已经对 `NO1004` 执行过 `FOR UPDATE` 了，B 执行同样的语句，**居然没有被阻塞，也顺利拿到了锁**。

如果 `FOR UPDATE` 真的像它的名字那样「排他」，B 在 T2 就该乖乖排队等 A 提交，整个死锁根本不会发生。

那么问题来了：A 查不到任何记录，它锁住的到底是什么？为什么 B 能同时锁住同一个东西？

## 六、锁去哪了：InnoDB 的加锁基本单位

要回答这个问题，得先纠正一个普遍的误解：**InnoDB 的行锁不是加在「表里那一行数据」上的，而是加在索引记录上的**。

`SELECT ... WHERE order_no = 'NO1004' FOR UPDATE` 执行时，InnoDB 沿着唯一索引 `uk_order_no` 做 B+ 树查找，定位到 `NO1004` 本该落入的那个位置。

在 RR 隔离级别下，为了解决**幻读**问题，InnoDB 的加锁基本单位不是记录锁，而是 **Next-Key Lock（临键锁）**：

```
Next-Key Lock = Record Lock（记录锁） + Gap Lock（间隙锁）
```

它锁的是一个**左开右闭**区间 `(前一条记录, 当前记录]`——既锁住记录本身，也锁住记录前面那段间隙，这样别的事务就无法在这个区间里插入新行，幻读也就无从发生。

关键在于：**当目标记录根本不存在时，没有记录可以锁**。于是 Next-Key Lock 退化为纯粹的 **Gap Lock（间隙锁）**，只锁住「这条记录本该落入的那个间隙」。

回到我们的例子：

```sql
SELECT id FROM t_order WHERE order_no = 'NO1004' FOR UPDATE;
-- 返回 0 行
-- 但 InnoDB 加上了 Gap Lock，锁住区间 (NO1003, NO1005)
```

```
[NO1001]   (NO1001, NO1003)   [NO1003]   (NO1003, NO1005)   [NO1005]
                                   ↑════════ 被锁住 ════════↑
                                            NO1004 想插在这里

```

所以第四节那个直觉假设的正确版本是：

> 查询没有返回任何行，但 InnoDB 锁住了这一行**本该出现的位置**。

间隙锁的语义是「**不许任何人往这个区间里插入数据**」。它的存在正是为了防止另一个事务在 A 判断「不存在」到 A 真正插入之间，抢先把 `NO1004` 插进去。从防幻读的角度看，这个设计是必要且正确的。

## 七、加锁规则的降级：四种场景

间隙锁不是随便加的，InnoDB 有一套明确的降级规则。整理成表：

| 查询类型                   | 索引性质          | 记录是否存在 | 加锁结果                | 锁定范围               |
| -------------------------- | ----------------- | ------------ | ----------------------- | ---------------------- |
| 等值查询 `=`               | 唯一索引          | 存在         | 记录锁（Record Lock）   | 仅命中的那一行         |
| 等值查询 `=`               | 唯一索引          | **不存在**   | **间隙锁（Gap Lock）**  | 该值本应落入的间隙     |
| 等值查询 `=`               | 非唯一索引        | 存在         | 临键锁（Next-Key Lock） | 所有命中行及其前后间隙 |
| 范围查询 `>` `<` `BETWEEN` | 任意索引          | 不限         | 临键锁（Next-Key Lock） | 扫描到的所有行与间隙   |
| 任意查询                   | 无索引 / 索引失效 | 不限         | 全表所有记录间隙加锁    | 效果等同表锁           |

这里要澄清一个流传很广的错误说法：**「加了唯一索引就不会有间隙锁」**。

准确的表述是分两种情况：

- 唯一索引等值查询**命中已有记录**时，Next-Key Lock 会降级为记录锁，只锁那一行，锁范围极小，几乎不会引发间隙问题；
- 唯一索引等值查询**未命中**时，照样要加间隙锁——因为 InnoDB 必须防止别的事务紧接着把这个值插进来。

我们这个案例恰好是第二种。`uk_order_no` 是唯一索引，`NO1004` 不存在，于是锁住 `(NO1003, NO1005)`。

## 八、最关键的一环：间隙锁不排斥间隙锁

到这里，A 持有间隙锁已经解释清楚了。剩下的谜题是：**为什么 B 在 T2 没有被 A 阻塞？**

MySQL 官方文档对间隙锁有一句精准的定义：间隙锁是「**纯抑制性的**」（purely inhibitive），它唯一的作用是阻止其他事务往这个间隙里插入数据，因此**间隙锁之间可以共存**。<sup>[1]</sup>

这个设计乍看奇怪，细想很合理。间隙锁表达的是「不许往这里插」，两个事务同时声明「不许往这里插」并不矛盾——它们的目标一致，没有资源竞争。真正矛盾的场景是「不许往这里插」遇上「我要往这里插」。

而「我要往这里插」，在 InnoDB 里对应的是一种专门的锁：**插入意向锁（Insert Intention Lock）**。它是间隙锁的一种特殊变体，由 `INSERT` 操作在真正写入记录之前申请，用来标识「我打算在这个间隙的某个位置插入一行」。<sup>[2]</sup>

插入意向锁的巧妙之处在于：多个事务往**同一个间隙的不同位置**插入时，它们的插入意向锁互相兼容，可以并发执行，不必排队。这是 InnoDB 为插入性能做的专门优化。

但它与已存在的间隙锁**互斥**——因为如果间隙锁不阻挡插入意向锁，幻读保护就形同虚设了。

完整的兼容性关系：

| 已持有 \ 请求  | 记录锁 S | 记录锁 X | 间隙锁 Gap | 插入意向锁 |
| -------------- | -------- | -------- | ---------- | ---------- |
| **间隙锁 Gap** | 兼容     | 冲突     | **兼容**   | **冲突**   |
| **插入意向锁** | 兼容     | 冲突     | 冲突       | 兼容       |

两个加粗的格子，就是整个死锁的全部成因：

- **Gap ↔ Gap 兼容** → 事务 A 和事务 B 可以同时持有 `(NO1003, NO1005)` 这个间隙的间隙锁，谁都不阻塞谁；
- **Gap ↔ 插入意向锁 冲突** → A 想插 `NO1004`，必须等 B 释放间隙锁；B 想插 `NO1004`，必须等 A 释放间隙锁。

于是形成了完美的循环等待：

```
事务 A ──持有──> Gap Lock (NO1003, NO1005) <──持有── 事务 B
   │                                                    │
   │  INSERT NO1004                                     │  INSERT NO1004
   │  需要插入意向锁                                     │  需要插入意向锁
   ↓                                                    ↓
被 B 的间隙锁阻塞 ────────────── 互相等待 ────────────── 被 A 的间隙锁阻塞

                     循环等待成立，死锁

```

A 在等 B，B 在等 A，两个人手里都攥着对方需要的东西，谁也不会先放手。

> 顺便辨析一个高频混淆点：**插入意向锁（Insert Intention Lock）不是意向锁（IS/IX）**。意向锁是**表级**锁，在任何行锁之前自动添加，作用是让表锁和行锁能快速判断冲突；插入意向锁是**行级（间隙）**锁，只在 `INSERT` 时出现。名字长得像，本质完全不同。

## 九、InnoDB 如何收场

死锁不会永远卡住。InnoDB 内置了死锁检测机制，一旦发现等待图中出现环，会立刻介入：

1. 挑选一个**回滚代价较小**的事务作为牺牲品，通常依据是 undo log 量——改动越少的事务，回滚越便宜；
2. 强制回滚它，释放其持有的全部锁；
3. 被回滚的一方收到 `ERROR 1213 (40001): Deadlock found when trying to get lock; try restarting transaction`；
4. 另一个事务的等待解除，继续执行并提交成功。

也就是说，两个并发请求里，**必然有一个失败**。如果应用层没有针对 1213 的重试逻辑，这个失败会直接抛到用户面前，表现为「下单失败，请重试」。

相关参数：

- `innodb_deadlock_detect`（默认 ON）：是否主动检测死锁。关闭后只能靠 `innodb_lock_wait_timeout`（默认 50 秒）超时打破僵局，代价是请求长时间挂起，一般不建议关。
- `innodb_print_all_deadlocks`（默认 OFF）：设为 ON 后，所有死锁都会写入 error log，而不只是保留最后一次。排查偶发死锁时很有用，但日志量会增大。

## 十、即使不死锁，也有代价

死锁只是这个问题最显眼的症状。还有三个连带影响，同样值得警惕。

**代价一：并发插入被大面积阻塞。** 一个事务持有间隙锁期间，**所有**落在该间隙内的 `INSERT` 都要排队，不只是插 `NO1004` 的那些。如果你的事务里夹了慢查询、外部 RPC 调用、消息发送，间隙锁的持有时间会被拉长到几百毫秒甚至几秒，这段时间内该区间的所有写入全部停摆。这类问题的表现不是报错，而是**接口 RT 突然升高、请求堆积**，且死锁日志里干干净净，非常难查。

**代价二：间隙大小取决于数据分布，且往往极不均匀。** 间隙 `(NO1003, NO1005)` 只覆盖一个订单号，看起来很安全。但真实业务里，索引记录的分布是跳跃的：

```
NO1001 ─── NO1003 ─── NO1010 ─── NO1100 ─── NO2000
    │  小  │   中   │    大     │    很大    │

```

间隙越大，两个并发请求恰好落进同一个间隙的概率就越高，冲突概率随之上升。号段分配、券码生成、按日期分片的业务尤其容易踩到这一点。

**代价三：WHERE 条件不走索引，等于锁全表。** 如果 `WHERE` 的列上没有可用索引，InnoDB 无法定位目标间隙，只能对聚簇索引上**每一条记录的间隙**都加锁，效果与表锁无异。这是最坏的情况，高并发下会直接雪崩。

### 换成 READ COMMITTED 会怎样

RC 隔离级别下，InnoDB 基本不使用间隙锁（仅唯一键冲突检查和外键检查等少数场景例外）。对未命中的等值查询，一般不加 Gap Lock，本文讨论的死锁**自然消失**。

代价是失去了 RR 的幻读保护——同一事务内两次相同的范围查询，可能读到不同的行数。收益是锁范围大幅缩小，并发度和吞吐量明显提升。这也是很多互联网业务把默认隔离级别从 RR 改成 RC 的原因。

改用 RC 有个硬性前提：`binlog_format` 必须是 `ROW`，不能是 `STATEMENT`。因为在 RC 下没有间隙锁，同一条 SQL 在主从上的执行结果可能不同，STATEMENT 格式记录语句会导致主从数据不一致。<sup>[3]</sup>

## 十一、六种解法

按推荐程度排序。

### 方案一：用数据库能力替代「先查再插」（首选）

既然唯一索引已经存在，就让数据库自己处理冲突，一条语句搞定，根本不需要预先加锁：

```sql
-- 存在则更新，不存在则插入
INSERT INTO t_order(order_no, user_id, amount, status)
VALUES ('NO1004', 1004, 88.00, 0)
ON DUPLICATE KEY UPDATE
  amount = VALUES(amount),
  status = VALUES(status);

-- 或者：存在就静默忽略
INSERT IGNORE INTO t_order(order_no, user_id, amount, status)
VALUES ('NO1004', 1004, 88.00, 0);

```

`INSERT IGNORE` 之后可以通过受影响行数判断结果：返回 1 表示插入成功，返回 0 表示因唯一键冲突被忽略。

**适用场景**：绝大多数「防重复创建」需求。
**注意**：`INSERT ... ON DUPLICATE KEY UPDATE` 在表上有**多个唯一索引**时，MySQL 5.7 的某些版本存在已知缺陷，并发下仍可能产生间隙锁死锁，5.7.35 及以上版本已修复。<sup>[4]</sup> 生产环境建议确认版本。

### 方案二：代码层捕获唯一键冲突

不做任何预检查，直接插入，靠异常分支处理重复：

```java
public void createOrder(String orderNo, Long userId, BigDecimal amount) {
    try {
        orderMapper.insert(new Order(orderNo, userId, amount));
    } catch (DuplicateKeyException e) {
        // 唯一键冲突，说明订单已存在
        log.warn("订单重复提交，orderNo={}", orderNo);
        // 按业务决定：抛业务异常 / 返回已有订单 / 静默忽略
    }
}

```

**适用场景**：重复提交概率低、希望代码路径最简的情况。
**优点**：正常路径只有一次写入，没有额外的查询开销，也没有间隙锁。
**注意**：必须精确捕获 `DuplicateKeyException`，不要笼统 `catch (Exception e)`，否则会吞掉其他真实故障。

### 方案三：预置记录，让行永远存在

对于计数器、日汇总、库存扣减这类场景，可以提前把行建好，业务代码里只做「加锁 + 更新」，永远查得到，也就永远不会退化成间隙锁：

```sql
-- 初始化阶段：按维度提前建行
INSERT INTO t_daily_stat(stat_date, biz_type, cnt) VALUES ('2026-11-15', 'ORDER', 0);

-- 业务阶段：行一定存在，加的是记录锁
BEGIN;
SELECT cnt FROM t_daily_stat
 WHERE stat_date = '2026-11-15' AND biz_type = 'ORDER'
 FOR UPDATE;                        -- 命中已有记录 → 记录锁，不是间隙锁
UPDATE t_daily_stat SET cnt = cnt + 1
 WHERE stat_date = '2026-11-15' AND biz_type = 'ORDER';
COMMIT;

```

**适用场景**：维度可枚举、可预先初始化的统计表与计数器。
**优点**：`SELECT ... FOR UPDATE` 命中已有记录时只加记录锁，锁粒度最小，语义也最清晰。

### 方案四：先快照读，再决定

如果业务流程上必须先判断再操作，那就把「判断」和「加锁」拆开。普通的 `SELECT` 走 MVCC 快照读，**完全不加锁**：

```sql
-- 第一步：快照读，不加任何锁，只做存在性判断
SELECT id FROM t_order WHERE order_no = 'NO1004';

-- 分支一：查到了 → 再对这个确实存在的行加锁
SELECT id FROM t_order WHERE order_no = 'NO1004' FOR UPDATE;
UPDATE ...;

-- 分支二：没查到 → 直接插入，靠唯一索引兜底，不要 FOR UPDATE
INSERT INTO t_order(...) VALUES (...);

```

**关键点**：不存在时**绝不要**执行 `FOR UPDATE`。快照读不产生间隙锁，插入失败时捕获唯一键冲突即可。

### 方案五：降低隔离级别到 RC

如前所述，RC 下没有间隙锁，这类死锁基本消失。

**适用场景**：业务能容忍幻读、追求高并发吞吐的互联网场景。
**前提**：`binlog_format = ROW`。
**代价**：失去 RR 的一致性读语义，对账、财务等强一致场景需谨慎评估。

### 方案六：应用层分布式锁

用 Redis 或 ZooKeeper 按业务 key 加锁，把并发压成串行：

```java
String lockKey = "order:create:" + orderNo;
if (redisLock.tryLock(lockKey, 3, TimeUnit.SECONDS)) {
    try {
        // 临界区内的数据库操作不再有并发竞争
        createOrderInDb(orderNo, userId, amount);
    } finally {
        redisLock.unlock(lockKey);
    }
} else {
    throw new BizException("操作过于频繁，请稍后重试");
}

```

**适用场景**：临界区包含复杂业务逻辑、无法用单条 SQL 表达，或需要跨多个数据源保持一致。
**代价**：引入额外中间件，需要处理锁超时、续期、误删、Redis 主从切换导致的锁失效等一系列问题。**能用前五种方案解决的，不要上分布式锁。**

### 一条底线要求

无论采用哪种方案，都要确保 `WHERE` 条件能走索引。这不只是为了性能，更是为了避免「无索引 → 全表间隙加锁 → 等同表锁」这个最坏情况。

## 十二、排查手册

### 第一现场：死锁日志

死锁发生后，最完整的信息在这里：

```sql
SHOW ENGINE INNODB STATUS\G

```

找到 `LATEST DETECTED DEADLOCK` 段落，它会打印两个事务各自持有和等待的锁、执行的 SQL、以及最终回滚了哪一个。典型的间隙锁死锁长这样：

```
------------------------
LATEST DETECTED DEADLOCK
------------------------
*** (1) TRANSACTION:
TRANSACTION 78601472, ACTIVE 0 sec inserting
mysql tables in use 1, locked 1
LOCK WAIT 3 lock struct(s), heap size 1136, 2 row lock(s)
MySQL thread id 12, OS thread handle 140234, query id 88 update
INSERT INTO t_order(order_no, user_id, amount, status) VALUES ('NO1004', 1004, 88.00, 0)

*** (1) WAITING FOR THIS LOCK TO BE GRANTED:
RECORD LOCKS space id 42 page no 4 n bits 72 index uk_order_no of table `shop`.`t_order`
  trx id 78601472 lock_mode X locks gap before rec insert intention waiting

*** (2) TRANSACTION:
TRANSACTION 78601476, ACTIVE 0 sec inserting
...
*** (2) HOLDS THE LOCK(S):
RECORD LOCKS space id 42 page no 4 n bits 72 index uk_order_no of table `shop`.`t_order`
  trx id 78601476 lock_mode X locks gap before rec

*** (2) WAITING FOR THIS LOCK TO BE GRANTED:
RECORD LOCKS ... lock_mode X locks gap before rec insert intention waiting

*** WE ROLL BACK TRANSACTION (1)

```

**识别要点**：看到 `lock_mode X locks gap before rec insert intention waiting`，就可以确认是间隙锁阻塞了插入意向锁。如果两个事务的 `HOLDS` 里都是 `lock_mode X locks gap before rec`，`WAITING` 里都是 `insert intention waiting`，那正是本文讨论的「查不到就插入」死锁。

想拿到更详细的锁信息，可以临时开启：

```sql
SET GLOBAL innodb_status_output_locks = ON;
SET GLOBAL innodb_print_all_deadlocks = ON;

```

这两项会让日志显著变大，线上排查完记得关掉。

### 实时观察：三张视图

死锁是瞬时的，但锁等待是持续的。想看当前正在发生什么：

```sql
-- 当前所有活跃事务
SELECT trx_id, trx_state, trx_started, trx_mysql_thread_id, trx_query
  FROM information_schema.INNODB_TRX;

-- 当前持有的锁（MySQL 8.0）
SELECT ENGINE_LOCK_ID, ENGINE_TRANSACTION_ID, INDEX_NAME,
       LOCK_TYPE, LOCK_MODE, LOCK_STATUS, LOCK_DATA
  FROM performance_schema.data_locks;

-- 谁在等谁
SELECT REQUESTING_ENGINE_TRANSACTION_ID AS waiting_trx,
       BLOCKING_ENGINE_TRANSACTION_ID   AS blocking_trx
  FROM performance_schema.data_lock_waits;

```

在 `data_locks` 里，如果看到 `LOCK_TYPE = RECORD` 且 `LOCK_MODE = X,GAP`，那就是间隙锁。`LOCK_DATA` 字段会告诉你锁在哪条索引记录之前。

### 一个能亲手复现的实验

开两个 MySQL 客户端窗口，按顺序执行，可以稳定复现本文的死锁：

```sql
-- 窗口 A
BEGIN;
SELECT id FROM t_order WHERE order_no = 'NO1004' FOR UPDATE;
-- 返回 Empty set，先不要 COMMIT

-- 窗口 B
BEGIN;
SELECT id FROM t_order WHERE order_no = 'NO1004' FOR UPDATE;
-- 同样返回 Empty set，且不被阻塞 ← 在这里停下来体会一下

-- 窗口 A
INSERT INTO t_order(order_no, user_id, amount, status) VALUES ('NO1004', 1004, 88.00, 0);
-- 卡住，开始等待

-- 窗口 B
INSERT INTO t_order(order_no, user_id, amount, status) VALUES ('NO1004', 1004, 88.00, 0);
-- 立刻报错：ERROR 1213 (40001): Deadlock found when trying to get lock

```

## 十三、一页速查清单

**原理层面**

- InnoDB 的行锁加在索引记录上，不是加在数据行上。
- RR 隔离级别下，加锁基本单位是 Next-Key Lock（记录锁 + 间隙锁，左开右闭）。
- `SELECT ... FOR UPDATE` 查询不存在的记录时，Next-Key Lock 降级为纯 Gap Lock，锁住该值本应落入的间隙。
- 间隙锁是「纯抑制性」的，只阻止插入，**间隙锁之间互相兼容**。
- `INSERT` 前需申请插入意向锁，它与其他插入意向锁兼容，但**与间隙锁互斥**。
- 插入意向锁 ≠ 表级意向锁 IS/IX。

**风险层面**

- 「查不到就插入」的两个事务必然死锁，InnoDB 回滚 undo 量较小的那个，报 1213。
- 即使不死锁，间隙锁持有期间该区间所有 INSERT 全部排队，表现为 RT 升高。
- 间隙大小取决于数据分布，间隙越大冲突概率越高。
- WHERE 列无索引或索引失效 → 全表间隙加锁 → 等同表锁。
- 「加了唯一索引就没有间隙锁」是错的，未命中时照样加。

**解法层面**

- 首选 `INSERT ... ON DUPLICATE KEY UPDATE` 或 `INSERT IGNORE`，让唯一索引做幂等。
- 次选直接 INSERT + 捕获 `DuplicateKeyException`。
- 统计/计数类场景预置记录，保证行永远存在。
- 必须先判断时用普通快照读，不存在时**不要** `FOR UPDATE`。
- 高并发业务可评估切换到 RC 隔离级别，前提是 `binlog_format = ROW`。
- 分布式锁是最后手段，能不用就不用。

**排查层面**

- `SHOW ENGINE INNODB STATUS\G` 看 `LATEST DETECTED DEADLOCK`。
- 关键标识：`lock_mode X locks gap before rec insert intention waiting`。
- 实时锁情况看 `performance_schema.data_locks`，`LOCK_MODE = X,GAP` 即间隙锁。
- 偶发死锁开 `innodb_print_all_deadlocks` 落全量日志。

------

一句话总结整篇文章：**`SELECT ... FOR UPDATE` 查一条不存在的记录，锁住的不是数据，而是这条数据本该出现的位置。这个位置可以被多个事务同时锁住，却只能被一个事务插入——死锁就诞生在这个不对称里。**

## 参考文献

[1] MySQL Reference Manual - InnoDB Locking. https://dev.mysql.com/doc/refman/9.0/en/innodb-locking.html

[2] MySQL Reference Manual - Locks Set by Different SQL Statements in InnoDB. https://dev.mysql.com/doc/refman/8.0/en/innodb-locks-set.html

[3] MySQL Reference Manual - SET TRANSACTION / binlog_format 与隔离级别约束. https://dev.mysql.com/doc/refman/8.0/en/set-transaction.html

[4] MySQL Bug #50413 - Concurrent INSERT ... ON DUPLICATE KEY UPDATE 导致 binlog 异常（5.7.35 修复）. https://bugs.mysql.com/bug.php?id=50413

------
