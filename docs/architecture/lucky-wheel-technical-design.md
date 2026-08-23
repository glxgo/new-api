# 幸运大转盘技术设计

## 1. 文档目的

本文把《幸运大转盘产品需求文档》映射到当前 `new-api` 的真实代码结构，定义数据模型、事务边界、接口、随机算法、奖励订阅、消耗排序、暂停恢复、迁移、测试和发布方案。

关联文档：

- `docs/product/lucky-wheel-prd.md`
- `docs/product/lucky-wheel-public-rules-draft.md`
- `CONTEXT.md`
- `CODEX_MEMORY.md`

本文只描述设计，不表示功能已经实现、提交或上线。

## 2. 现有系统边界

### 2.1 可以直接复用

| 现有能力 | 主要位置 | 复用方式 |
|---|---|---|
| 套餐模板与订单快照 | `model/subscription.go` | 新增幸运卡权益字段并进入现有 `PlanSnapshot` |
| 用户订阅实例 | `model/subscription.go` | 活动奖品仍使用 `UserSubscription`，通过来源字段与禁发卡字段隔离 |
| 具体实例 Key 绑定 | `model/subscription_binding.go` | 全额重置时原子迁移绑定；绑定 Key 不读取用户全局排序 |
| 单线续费链 | `model/subscription_renewal.go` | 全额重置插入当前实例与已购后继之间 |
| 周期重置任务 | `service/subscription_reset_task.go` | 统一重置事件并以周期唯一键发卡 |
| 钱包本金/赠金双池 | `model/user.go`、`service/dual_pool.go` | 钱包奖品进入 `gift_quota`；订阅余额支付继续只读 `quota` |
| 订阅预扣幂等 | `model/subscription.go` | 未绑定 Key 的候选顺序增加用户优先级 |
| 用户通知和日志 | 现有日志/通知能力 | 事务后发送，不影响抽奖提交 |
| AutoMigrate | `model/main.go` | 新表和新增字段走三数据库兼容迁移 |

### 2.2 不能直接复用

现有 `recharge_credits` 不能直接作为幸运卡充值进度账本：

1. 它服务于账号并发/RPM容量；
2. 历史和当前逻辑会计入外部支付订阅；
3. 会计入结构化后台加额；
4. 幸运卡充值阶梯只允许钱包真实充值；
5. 幸运卡还需要门槛编号、退款冲销、发卡关联和暂停快照。

因此必须建立独立的幸运活动充值事件与进度表，不能通过筛选 `recharge_total_cents` 猜测。

### 2.3 已存在的重要保护

`PurchaseSubscriptionWithBalanceRenewal` 当前只检查并扣减 `users.quota`，不读取 `gift_quota`。这已经满足“赠金不可购买订阅”的核心方向，但仍需增加回归测试，防止未来统一余额展示时误把赠金加入订阅可支付余额。

## 3. 总体架构

```text
支付/重置事件
    |
    v
资格快照与幂等发卡
    |
    v
lucky_cards（绑定来源、规则版本、到期时间）
    |
    v
POST /api/lucky-wheel/draws
    |
    +-- 锁定幸运卡
    +-- 服务端 CSPRNG
    +-- 按不可变规则版本选奖
    +-- 同一事务发放奖品
    +-- 卡状态 available -> consumed
    |
    v
lucky_draws + 奖励订阅/赠金流水
```

分层约束：

- Router：只注册路由和鉴权中间件；
- Controller：参数校验、权限和统一响应；
- Service：抽奖、暂停恢复、奖励发放编排；
- Model：GORM 模型、事务内低层原语和查询；
- Frontend：只展示服务端结果，不参与随机数或奖品映射。

周期重置的现有实现位于 Model，且请求预扣可能触发惰性重置。为避免 `model -> service` 循环依赖，事务内发卡原语放在 `model/lucky_card.go`，Service 负责跨模型编排和外部通知。

## 4. 数据模型

所有 JSON 配置使用 `TEXT` 存储并通过 `common.Marshal` / `common.Unmarshal*` 处理。不得在业务代码直接调用 `encoding/json` 的编解码函数。

### 4.1 `lucky_campaigns`

单活动控制表。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 主键 |
| `code` | varchar(64), unique | 固定 `lucky-wheel` |
| `name` | varchar(128) | 幸运大转盘 |
| `active_rule_set_id` | bigint, index | 当前新发卡规则版本 |
| `issuance_paused` | bool | 是否暂停发卡 |
| `draw_paused` | bool | 是否暂停抽奖 |
| `draw_pause_started_at` | bigint | 当前暂停开始时间 |
| `settings_version` | bigint | 管理端乐观锁版本 |
| `created_at` | bigint | 创建时间 |
| `updated_at` | bigint | 更新时间 |

约束：

- `code` 唯一；
- 只有 Root 可以激活新规则版本；
- 暂停与恢复必须使用事务并写审计。

### 4.2 `lucky_rule_sets`

不可变规则版本。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 主键 |
| `campaign_id` | bigint, index | 活动 |
| `version` | int | 递增版本 |
| `status` | varchar(32) | draft/active/retired |
| `subscription_pool` | text | 订阅来源奖池 JSON |
| `recharge_pool` | text | 充值来源奖池 JSON |
| `threshold_config` | text | 充值阶梯 JSON |
| `recharge_bonus_usd_micros` | bigint | 已废弃，固定为 0；奖项配置金额即实际发放金额 |
| `recharge_card_valid_seconds` | bigint | 30 天 |
| `recharge_reward_valid_seconds` | bigint | 30 天 |
| `crazy_card_valid_seconds` | bigint | 5 小时 |
| `crazy_card_quota_usd_micros` | bigint | 600 美元 |
| `activity_group` | varchar(64) | 充值套餐奖与狂蹬卡限定分组 |
| `checksum` | char(64) | 规范化配置 SHA-256 |
| `published_at` | bigint | 公示时间 |
| `effective_at` | bigint | 生效时间 |
| `created_by` | int | 操作者 |
| `created_at` | bigint | 创建时间 |

唯一索引：

- `(campaign_id, version)`。

激活门禁：

- 两套奖池每套权重必须恰好为 `1000000`；
- 奖项代码不可重复；
- 金额、有效期、分组必须有效；
- 激活后禁止 UPDATE，只能新建版本。

### 4.3 `lucky_cards`

一张物理幸运卡一行。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 卡号 |
| `user_id` | int, index | 所有者 |
| `campaign_id` | bigint, index | 活动 |
| `rule_set_id` | bigint, index | 发卡时规则 |
| `pool_type` | varchar(32) | subscription/recharge |
| `source_type` | varchar(32) | subscription_purchase/subscription_reset/recharge_threshold/admin_compensation |
| `source_ref` | varchar(255) | 订单、门槛或补偿引用 |
| `source_order_id` | int, index | 可空 |
| `source_subscription_id` | int, index | 订阅来源必填 |
| `source_cycle_key` | varchar(128) | initial/reset timestamp/threshold ordinal |
| `source_snapshot` | text | 来源权益冻结快照 |
| `source_effective_end_time` | bigint | 奖励计算使用的来源结束时间 |
| `grant_key` | varchar(255), unique | 发卡幂等键 |
| `status` | varchar(32), index | available/consumed/expired/revoked/review_required |
| `issued_at` | bigint | 发放时间 |
| `expires_at` | bigint, index | 当前到期时间 |
| `pause_extension_seconds` | bigint | 累计冻结顺延 |
| `consumed_at` | bigint | 抽奖时间 |
| `revoked_at` | bigint | 撤销时间 |
| `revoke_reason` | varchar(255) | 撤销原因 |
| `created_at` | bigint | 创建时间 |
| `updated_at` | bigint | 更新时间 |

索引：

- 唯一 `grant_key`；
- `(user_id, status, expires_at)`；
- `(source_subscription_id, status)`；
- `(campaign_id, status, expires_at)`。

`grant_key` 示例：

- 充值门槛：`recharge:{event_id}:stage:{ordinal}`
- 套餐购买：`purchase:{order_id}:slot:{1..n}`
- 周期重置：`reset:{subscription_id}:{reset_epoch}`
- 人工补偿：`compensation:{ticket_id}:slot:{1..n}`

### 4.4 `lucky_draws`

一张卡最多一条成功抽奖。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 抽奖编号 |
| `user_id` | int, index | 用户 |
| `card_id` | bigint, unique | 幸运卡 |
| `rule_set_id` | bigint, index | 实际规则 |
| `idempotency_key` | varchar(64) | 客户端幂等键 |
| `random_value` | int | `0..999999` |
| `prize_type` | varchar(32) | 奖项代码 |
| `display_usd_micros` | bigint | 转盘展示金额 |
| `actual_usd_micros` | bigint | 实际发放金额 |
| `awarded_quota` | bigint | 最终平台额度 |
| `reward_subscription_id` | int, index | 奖励实例 |
| `gift_quota_awarded` | bigint | 钱包赠金额度 |
| `rule_checksum` | char(64) | 规则校验 |
| `status` | varchar(32) | awarded/review_required/reversed |
| `awarded_at` | bigint | 发奖时间 |
| `created_at` | bigint | 创建时间 |
| `updated_at` | bigint | 更新时间 |

唯一索引：

- `card_id`
- `(user_id, idempotency_key)`

抽奖事务失败时不提交 `lucky_draws`，幸运卡保持 `available`。不保留一个“扣卡成功但奖品 pending”的正常状态。

### 4.5 `lucky_recharge_events`

幸运活动专用充值事件。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 事件 |
| `user_id` | int, index | 用户 |
| `source_type` | varchar(32) | wallet_topup/refund/chargeback |
| `source_ref` | varchar(255) | 支付单号 |
| `direction` | int | `1` 增加，`-1` 冲销 |
| `amount_cents` | bigint | 绝对金额 |
| `rule_set_id` | bigint | 订单资格快照 |
| `occurred_at` | bigint | 支付完成时间 |
| `created_at` | bigint | 创建时间 |

唯一索引：

- `(source_type, source_ref, direction)`。

外部支付订阅订单、后台加额、赠金和兑换码不写入本表。

### 4.6 `lucky_recharge_progress`

每用户一行。

| 字段 | 类型 | 说明 |
|---|---|---|
| `user_id` | int PK | 用户 |
| `eligible_cents` | bigint | 冲销后的当前合格累计 |
| `highest_awarded_stage` | bigint | 历史已发最高门槛，只增不减 |
| `next_threshold_cents` | bigint | 下一门槛缓存 |
| `updated_at` | bigint | 更新时间 |

退款可以降低 `eligible_cents`，但不降低 `highest_awarded_stage`，防止“充值—退款—再充值”重复获得历史门槛卡。

### 4.7 `lucky_reward_buckets`

订阅来源套餐额度奖的聚合映射。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 主键 |
| `user_id` | int, index | 用户 |
| `source_subscription_id` | int, index | 来源实例 |
| `effective_end_time` | bigint | 奖励有效结束时间 |
| `reward_subscription_id` | int, unique | 奖励实例 |
| `created_at` | bigint | 创建时间 |
| `updated_at` | bigint | 更新时间 |

唯一索引：

- `(user_id, source_subscription_id, effective_end_time)`。

充值来源套餐额度每次抽奖创建独立 30 天实例，不使用本表聚合。

### 4.8 `lucky_pause_periods`

活动暂停审计。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 主键 |
| `campaign_id` | bigint, index | 活动 |
| `pause_type` | varchar(16) | issuance/draw |
| `started_at` | bigint | 开始 |
| `ended_at` | bigint | 恢复 |
| `duration_seconds` | bigint | 时长 |
| `reason` | varchar(255) | 原因 |
| `operator_id` | int | 操作者 |
| `affected_cards` | bigint | 顺延卡数 |
| `status` | varchar(16) | active/resuming/completed |
| `created_at` | bigint | 创建时间 |
| `updated_at` | bigint | 更新时间 |

恢复抽奖时保持 `draw_paused=true`，先分批顺延卡片；全部成功后把 pause period 标记完成并开放抽奖。

### 4.9 `subscription_consumption_priorities`

未绑定 Key 的分组内消耗顺序。

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | bigint PK | 主键 |
| `user_id` | int, index | 用户 |
| `group_name` | varchar(64) | 请求分组 |
| `subscription_id` | int, index | 订阅实例 |
| `priority` | int | 从 1 开始 |
| `revision` | bigint | 乐观锁版本 |
| `updated_at` | bigint | 更新时间 |

唯一索引：

- `(user_id, group_name, subscription_id)`
- `(user_id, group_name, priority)`

保存时提交当前分组完整实例顺序；后端验证所有实例属于用户且兼容该分组。失效实例可以保留历史行，但读取候选时过滤。

## 5. 现有模型字段调整

### 5.1 `SubscriptionPlan`

新增：

```go
LuckyCardGrantCount   int  // 购买成功立即发 n 张
LuckyCardOnReset      bool // 每个真实额度重置周期发 1 张
Purchasable           bool // 系统奖励模板不可购买
SystemCode            *string // 可空，系统模板唯一代码
```

规则：

- `LuckyCardGrantCount >= 0`；
- 系统奖励模板 `Purchasable=false`、`Enabled=false`；
- `GetSubscriptionPlans` 只返回 `enabled=true AND purchasable=true`；
- 管理员不能删除或改成可购买的系统模板；
- SQLite 手工 `subscription_plans` 迁移必须同步新增字段。

系统模板：

- `lucky-recharge-quota`：充值来源套餐额度；
- `lucky-crazy-5h`：5 小时狂蹬卡。

### 5.2 `SubscriptionOrder`

新增资格快照：

```go
LuckyRuleSetId       int64
LuckyGrantEligible   bool
LuckyGrantCount      int
LuckyGrantOnReset    bool
```

下单时冻结。套餐后来编辑、活动暂停或规则切换不追溯有效待支付订单。支付完成后，订阅实例创建和 `n` 张幸运卡必须在同一事务中完成。

### 5.3 `TopUp`

新增：

```go
LuckyRuleSetId         int64
LuckyRechargeEligible  bool
```

创建充值订单时冻结资格。暂停发卡后新创建的订单不具备资格；暂停前已向用户展示资格的有效订单，完成后继续履行。

### 5.4 `UserSubscription`

新增：

```go
LuckyCardDisabled            bool
PromotionOriginDrawId        int64
PromotionSourceSubscriptionId int
SupersededById               *int
```

约束：

- 所有奖励订阅 `LuckyCardDisabled=true`；
- 奖励订阅 `PaidRevenueQuota=0`；
- 奖励订阅 `DividendState=SubscriptionDividendSkippedSource`；
- 奖励订阅仍累计真实渠道成本，但不产生分红；
- `SupersededById` 用于全额重置的可审计替代关系；
- `Source` 使用稳定代码：`lucky_quota`、`lucky_double`、`lucky_full_reset`、`lucky_crazy_5h`。

管理员订阅统计应区分商业购买与活动奖励，不能把活动奖励计为付费购买数。

## 6. 充值门槛算法

```go
func LuckyThresholdCents(stage int64) int64 {
    if stage < 1 {
        stage = 1
    }
    return stage * 3_000
}
```

支付完成事务：

1. 锁定充值订单；
2. 完成钱包本金充值；
3. 以 `(wallet_topup, trade_no, 1)` 插入 `lucky_recharge_events`；
4. 锁定或创建 `lucky_recharge_progress`；
5. 增加 `eligible_cents`；
6. 从 `highest_awarded_stage+1` 开始计算；
7. 每跨一个门槛创建一张 `lucky_cards`；
8. 更新最高阶段和下一门槛；
9. 提交事务；
10. 失效用户缓存并发送通知。

支付回调重试时，唯一索引使步骤 3 返回未创建，后续进度与发卡不得重复。

退款：

1. 插入方向为 `-1` 的唯一事件；
2. 减少 `eligible_cents`，最低为 0；
3. 不降低 `highest_awarded_stage`；
4. 撤销该充值事件直接产生且仍未使用的幸运卡；
5. 已使用卡标记关联抽奖 `review_required`；
6. 不自动扣除已消费奖励，进入人工退款规则。

## 7. 订阅购卡与周期发卡

### 7.1 下单资格快照

所有订阅支付入口统一调用：

```go
SnapshotLuckySubscriptionGrant(plan, campaign, ruleSet) LuckyGrantSnapshot
```

外部支付与钱包本金支付使用同一快照。条件：

- 活动未暂停发卡；
- 套餐可购买；
- 订单金额或实际扣除本金大于 0；
- 套餐 `LuckyCardGrantCount > 0` 或 `LuckyCardOnReset=true`。

### 7.2 支付完成

必须先拿到新创建的 `UserSubscription`，再调用：

```go
GrantPurchaseLuckyCardsTx(tx, order, subscription)
```

每张卡：

- `pool_type=subscription`
- `source_type=subscription_purchase`
- `source_subscription_id=subscription.Id`
- `expires_at=subscription.EndTime`
- `source_effective_end_time=subscription.EndTime`
- `grant_key=purchase:{order.Id}:slot:{i}`

奖励订单或 `LuckyCardDisabled=true` 的实例直接跳过。

### 7.3 周期重置

当前 `maybeResetUserSubscriptionWithPlanTx` 只返回错误，且可能一次跨过多个周期。需要重构为：

```go
AdvanceSubscriptionResetTx(...) ([]ResetEpoch, error)
```

返回每个实际跨越的重置周期。背景任务和请求惰性重置都必须调用同一函数。

每个 `ResetEpoch`：

- 完成额度重置；
- 若计划快照允许、实例不是活动奖励、活动在该周期未暂停发卡，则创建一张卡；
- 绑定该周期实际发生时间点已经生效的规则版本，而不是任务恢复时的当前版本；
- `grant_key=reset:{subscription.Id}:{epoch.Timestamp}`。

发卡失败必须使该次重置事务回滚，不能形成已重置但永久漏卡。唯一键保证背景任务与惰性重置竞争时只发一张。

现有维护任务先执行 `ExpireDueSubscriptions` 再执行 `ResetDueSubscriptions`。实现周期补发时必须调整边界：仍处于有效期内的实例先推进并记录到期前应发生的重置周期，再处理到期；如果服务中断一直跨过来源实例结束时间，不创建一张生成即过期的卡，而是写入补偿审核事件，由管理员按审计结果处理。

## 8. 随机算法

### 8.1 随机源

使用 Go 标准库 `crypto/rand`，禁止：

- `math/rand`
- 前端随机数
- 根据用户、时间或卡号取模
- 抽中不适用奖项后再次重抽

生成：

```go
n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
randomValue := int(n.Int64()) // 0..999999
```

### 8.2 奖项选择

规则版本激活时把奖项按固定顺序规范化。选择使用左闭右开累计区间：

```text
[0, w1)
[w1, w1+w2)
...
```

两套奖池分别完整定义。充值来源卡直接使用充值奖池，不使用“抽到双倍/重置后重抽”的拒绝采样。

### 8.3 可测试性

Service 接受内部 `RandomSource` 接口：

```go
type RandomSource interface {
    Intn(max int64) (int64, error)
}
```

生产实现包装 `crypto/rand`；测试注入确定值，覆盖每个区间边界。

只保存最终 `random_value`、规则版本和校验和，不保存可预测后续结果的随机种子。

## 9. 抽奖事务

接口请求：

```json
{
  "card_id": 12345,
  "idempotency_key": "客户端生成的随机UUID"
}
```

事务步骤：

1. 按 `(user_id, idempotency_key)` 查询已有抽奖；存在则返回原结果。
2. `FOR UPDATE` 锁定 `lucky_cards`。
3. 校验所有者、`available`、数据库时间未到期、活动未暂停抽奖。
4. 读取并校验卡片绑定的不可变规则版本与 checksum。
5. 生成 `random_value` 并选奖。
6. 创建 `lucky_draws`。
7. 在同一事务中发放奖品。
8. 更新 `lucky_cards.status=consumed` 和 `consumed_at`。
9. 提交。
10. 事务外失效用户/订阅缓存，发送通知和运营事件。

任何第 4 至第 8 步错误都会回滚。客户端重试时仍可使用原卡。

抽奖接口增加：

- 用户鉴权；
- Critical rate limit；
- 单用户短时抽奖速率限制；
- 请求体大小限制；
- IDOR 所有权校验。

## 10. 奖品发放

### 10.1 美元到额度

奖品规则使用 `usd_micros`，发奖时按活动规则版本冻结的单位换算为整型平台额度：

```go
quota = decimal.NewFromInt(usdMicros).
    Div(decimal.NewFromInt(1_000_000)).
    Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
    Round(0).
    IntPart()
```

`lucky_draws` 同时保存规则美元金额和最终整型额度，避免以后 `QuotaPerUnit` 变化导致历史显示漂移。

### 10.2 订阅来源套餐额度

1. 从卡片 `source_snapshot` 读取来源 PlanSnapshot 和 AllowedGroup；
2. 生成无重置、禁发卡、实付收入 0 的奖励快照；
3. 按 `(user, source_subscription_id, source_effective_end_time)` 锁定奖励桶；
4. 不存在则创建奖励订阅和桶；
5. 已存在则原子增加 `AmountTotal`；
6. `AmountCap=0`，只用生命周期额度；
7. 奖励实例结束时间为 `source_effective_end_time`。

如果结束时间已过且仅因为抽奖暂停顺延，使用卡片已经顺延的 `source_effective_end_time`，不修改原来源实例。

### 10.3 充值来源套餐额度

1. `actual_usd = display_usd + 60`；
2. 使用系统模板 `lucky-recharge-quota`；
3. 每次抽奖创建独立实例；
4. `StartTime=draw time`；
5. `EndTime=draw time + 30 days`；
6. `AllowedGroup=ruleSet.activity_group`；
7. 无重置、禁发卡、实付收入 0。

### 10.4 钱包赠金

事务内执行：

```sql
gift_quota = gift_quota + awarded_quota
```

要求：

- 使用 GORM 表达式；
- 提交后失效用户缓存；
- 记录抽奖号作为资金来源；
- 不写 `recharge_credits`；
- 不增加 `recharge_total_cents`；
- 订阅余额支付继续只检查 `quota`。

建议增加事务版原语：

```go
IncreaseUserGiftQuotaTx(tx, userId, quota)
```

### 10.5 套餐双倍卡

1. 读取来源实例和冻结快照；
2. 克隆完整 `TotalAmount`、`AmountCap`、AllowedGroup 和重置规则；
3. `AmountUsed=0`、`AmountCapUsed=0`；
4. `StartTime=draw time`；
5. `EndTime=card.source_effective_end_time`；
6. 周期重置时间沿用来源实例的日历锚点，而不是重新赠送额外周期；
7. 不复制任何 Token 绑定；
8. `LuckyCardDisabled=true`；
9. `DividendState=skipped_source`。

若剩余有效期不大于 0，抽奖事务失败并保留卡片；正常情况下暂停顺延已保证有效结束时间大于抽奖时间。

### 10.6 套餐全额重置卡

不能直接覆盖旧行的消费和成本字段。

事务步骤：

1. 锁定来源实例、直接续费后继链和相关 Token；
2. 校验来源属于用户且未被退款/作废；
3. 把来源状态改为 `superseded`；
4. 创建同套餐快照的新赠送实例：
   - 完整周期时长；
   - 完整周期额度和总额度；
   - `RenewedFromId=source.Id`；
   - `LuckyCardDisabled=true`；
   - `PaidRevenueQuota=0`；
   - `DividendState=skipped_source`；
5. 如果来源已有付费后继：
   - 暂时解除其 `RenewedFromId`；
   - 把赠送实例插入来源与付费后继之间；
   - 把后继及其后续链整体顺延到赠送实例结束之后；
6. 把所有当前绑定来源的 Token 原子改绑到赠送实例；
7. 保留原计划中的付费后继为 `PlannedSubscriptionId`，更新生效时间；
8. 写 Token 归属变更历史；
9. 设置来源 `SupersededById`；
10. 提交后失效 Token 与用户缓存。

旧实例的在途预扣继续按旧 `subscription_id` 结算。旧实例分红必须等待其未决预扣完成，不能因抽奖事务同步触发慢查询或提前分红。

现有延迟分红扫描只面向到期订阅；实现时需要让被全额重置替代的 `superseded` 实例进入同一“无未决预扣后再结算”流程，或者增加等价的独立扫描，避免旧实付周期永久停留在 `pending`。

### 10.7 5 小时狂蹬卡

使用系统模板 `lucky-crazy-5h`：

- `DurationUnit=custom`
- `CustomSeconds=18000`
- `TotalAmount=600 USD` 对应平台额度
- `AmountCap=0`
- `QuotaResetPeriod=never`
- `AllowedGroup=ruleSet.activity_group`
- `LuckyCardDisabled=true`
- `PaidRevenueQuota=0`
- `DividendState=skipped_source`

## 11. 暂停与恢复

### 11.1 暂停发卡

开启后：

- 新充值订单资格快照为 false；
- 新订阅订单幸运卡快照为 false；
- 新发生的周期重置不发卡；
- 已在暂停前冻结资格的有效待支付订单仍可发卡；
- 不对暂停期间的新交易或周期进行恢复后补发。

### 11.2 暂停抽奖

暂停事务：

1. `draw_paused=true`；
2. 写入 active `lucky_pause_periods`；
3. 记录开始时间和原因。

恢复流程：

1. 把 pause period 标记 `resuming`；
2. 计算数据库时间差；
3. 按主键分批更新 `available` 且暂停开始时未到期的卡：
   - `expires_at += duration`
   - `source_effective_end_time += duration`（仅订阅来源）
   - `pause_extension_seconds += duration`
4. 每批使用确定性游标，记录最后处理 ID；
5. 全部完成后记录 `affected_cards`；
6. 标记 pause period `completed`；
7. 清空 `draw_pause_started_at`；
8. 最后设置 `draw_paused=false`。

恢复任务可重入。应在 pause period 保存处理游标或单独任务状态，避免进程重启重复顺延。

## 12. 订阅消耗顺序

### 12.1 读取

新增：

```text
GET /api/subscription/self/consumption-order?group={group}
```

响应包含：

- 当前分组；
- 当前 revision；
- 已手动排序实例；
- 未排序但兼容的有效实例；
- 每个实例的到期时间、剩余额度、来源和绑定 Key 数量。

### 12.2 保存

```text
PUT /api/subscription/self/consumption-order
```

请求：

```json
{
  "group": "套餐专用分组",
  "revision": 12,
  "subscription_ids": [31, 28, 40]
}
```

校验：

- 实例全部属于当前用户；
- 当前有效；
- `AllowedGroup=''` 或等于请求分组；
- ID 不重复；
- revision 与服务器一致。

保存使用事务，删除当前分组旧行并按完整顺序写入新行；revision 冲突返回 409，前端刷新后让用户重新确认。

### 12.3 候选排序

修改 `subscriptionCandidatesTx` 的未绑定分支：

1. `FOR UPDATE` 查询所有兼容有效实例；
2. 读取当前用户/分组优先级映射；
3. 在 Go 内稳定排序：
   - 有人工 priority 的排在前面；
   - priority 小的优先；
   - 未排序实例按 `end_time asc, id asc`；
4. 继续使用现有额度充足校验；
5. 绑定模式分支完全不读取该表。

避免在 SQL 中使用数据库特有的 `FIELD()`、数组函数或 JSON 排序。

## 13. API 设计

### 13.1 用户接口

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | `/api/lucky-wheel/status` | 活动状态、规则版本、可用卡数、充值进度 |
| GET | `/api/lucky-wheel/rules` | 当前及用户持卡涉及的规则版本 |
| GET | `/api/lucky-wheel/cards` | 分页查询我的卡 |
| POST | `/api/lucky-wheel/draws` | 使用指定卡抽奖 |
| GET | `/api/lucky-wheel/draws` | 我的抽奖记录 |
| GET | `/api/lucky-wheel/draws/:id` | 单次结果与奖励详情 |
| GET | `/api/subscription/self/consumption-order` | 读取分组顺序 |
| PUT | `/api/subscription/self/consumption-order` | 保存分组顺序 |

所有返回金额同时提供：

- 稳定整型平台额度；
- 面向用户的美元金额；
- 格式化由前端统一工具完成。

### 13.2 管理员接口

| 方法 | 路径 | 权限 | 用途 |
|---|---|---|---|
| GET | `/api/lucky-wheel/admin/overview` | Admin | 运营指标 |
| GET | `/api/lucky-wheel/admin/cards` | Admin | 卡片审计 |
| GET | `/api/lucky-wheel/admin/draws` | Admin | 抽奖审计 |
| POST | `/api/lucky-wheel/admin/cards/compensate` | Root | 人工补卡 |
| POST | `/api/lucky-wheel/admin/pause-issuance` | Root | 暂停发卡 |
| POST | `/api/lucky-wheel/admin/resume-issuance` | Root | 恢复发卡 |
| POST | `/api/lucky-wheel/admin/pause-draw` | Root | 暂停抽奖 |
| POST | `/api/lucky-wheel/admin/resume-draw` | Root | 顺延并恢复 |
| GET | `/api/lucky-wheel/admin/rule-sets` | Admin | 规则列表 |
| POST | `/api/lucky-wheel/admin/rule-sets` | Root | 新建草稿 |
| POST | `/api/lucky-wheel/admin/rule-sets/:id/activate` | Root | 校验并激活 |
| POST | `/api/lucky-wheel/admin/refund-review/:draw_id` | Root | 退款争议处理 |

概率配置、直接奖品、暂停恢复和人工补偿均使用 Root 权限。

## 14. 前端设计

### 14.1 Default 前端

新增：

```text
web/default/src/routes/_authenticated/lucky-wheel/index.tsx
web/default/src/features/lucky-wheel/
  api.ts
  types.ts
  constants.ts
  hooks/
  components/
  lib/
  index.tsx
```

核心组件：

- `LuckyWheelPage`
- `LuckyCardSelector`
- `WheelStage`
- `RechargeProgressCard`
- `LuckyCardList`
- `PrizePoolDialog`
- `DrawResultDialog`
- `DrawHistoryTable`
- `CampaignPausedBanner`
- `LuckyRulesDialog`

订阅排序放入现有“我的订阅”区域：

- `web/default/src/features/wallet/components/my-subscriptions-detail.tsx`
- 新增 `subscription-consumption-order-dialog.tsx`

套餐卡展示修改：

- `web/default/src/features/wallet/components/subscription-plans-card.tsx`
- `web/default/src/features/subscriptions/components/subscriptions-mutate-drawer.tsx`
- `web/default/src/features/subscriptions/types.ts`

要求：

- 桌面和移动端均可选择具体幸运卡；
- 转盘结果只能来自 POST 响应；
- 动画中断可从 GET draw 恢复；
- reduced-motion 直接显示结果；
- 文案进入全部六种 Default 前端语言，中文为产品验收基准；
- 不在 initial shell 静态导入重型转盘动画依赖，活动路由动态加载。

### 14.2 Classic 前端

Classic 至少提供功能等价的：

- 幸运卡列表；
- 卡片来源；
- 抽奖按钮；
- 结果；
- 概率规则；
- 抽奖记录；
- 套餐消耗顺序。

转盘动画可以简化，但不能因主题不同改变概率、资格或奖品。

### 14.3 动画

优先使用现有 CSS/轻量 SVG，不新增大型 WebGL 依赖。服务端返回结果后，前端根据奖项代码旋转到对应视觉区域；动画角度不参与奖项判定。

## 15. 缓存与一致性

1. 幸运卡、抽奖和资格进度以主数据库为准。
2. 抽奖事务不依赖 Redis 成功。
3. 用户余额/赠金发奖提交后失效用户缓存。
4. 新建或变更奖励订阅后失效订阅信息缓存。
5. 全额重置后失效相关 Token 缓存。
6. 活动状态可使用短 TTL 内存缓存，但暂停操作必须主动失效。
7. 抽奖校验必须在事务中再次读取活动状态，不能只信缓存。

## 16. 审计与合规留存

至少保留两年：

- 规则版本及 checksum；
- 公示内容快照；
- 发卡来源；
- 充值资格事件；
- 抽奖随机值和规则版本；
- 奖品发放结果；
- 暂停/恢复；
- 人工补偿；
- 退款/拒付处理；
- 管理员操作。

禁止在审计中保存：

- 完整 API Key；
- 支付密钥；
- Cookie；
- 私钥；
- 与活动无关的请求内容。

活动表不跟随用户普通日志清理任务做短期删除。用户软删除后保留必要财务/合规记录并按隐私政策做身份最小化。

## 17. 监控与告警

指标：

- `lucky_cards_issued_total{source}`
- `lucky_cards_available`
- `lucky_cards_expired_total`
- `lucky_draws_total{pool,prize}`
- `lucky_draw_failures_total{reason}`
- `lucky_award_quota_total{kind}`
- `lucky_gift_quota_total`
- `lucky_pause_resume_pending`
- `lucky_refund_reviews_pending`
- `lucky_probability_deviation{prize}`

告警：

- 抽奖事务错误率持续升高；
- 某奖项实际概率显著偏离理论值；
- 卡 consumed 但缺少 draw（理论上不应存在）；
- draw awarded 但奖励引用缺失；
- 暂停恢复任务长时间未完成；
- 同一支付引用出现资格冲突；
- 活动成本超过日/周预算阈值。

预算阈值只能告警或暂停新发卡，不能静默改变已公示概率。

## 18. 安全设计

1. 所有用户接口验证资源所有权，防止 IDOR。
2. 抽奖使用 CSPRNG。
3. 规则激活使用 RootAuth。
4. 管理员不能修改已经完成的随机值。
5. 人工奖励与随机中奖分表意或至少分类型展示。
6. 卡、抽奖、发奖全部使用数据库唯一约束。
7. 金额和概率使用整数，不用 float64 参与资格、概率或额度计算。
8. 客户端提交的奖项、概率、实际金额和来源快照一律忽略。
9. 批量卡查询和历史查询分页并限制最大页大小。
10. 所有暂停、补偿、退款操作写管理员审计。

## 19. 迁移方案

### 19.1 数据库

1. 新增所有活动表；
2. 给 `subscription_plans`、`subscription_orders`、`topups`、`user_subscriptions` 增加字段；
3. 更新 SQLite 的 `ensureSubscriptionPlanTableSQLite`；
4. AutoMigrate 支持 MySQL、PostgreSQL、SQLite；
5. 创建两个不可购买的系统奖励模板；
6. 规则切换时将历史充值进度清零，用户从 0 元按新规则重新累计；
7. 不给历史订阅订单补发幸运卡；
8. 不把历史周期重置追溯为幸运卡。

活动资格从正式启用时的新订单/新周期开始。

### 19.2 兼容

- 所有新增字段默认关闭或为 0；
- 活动开关默认“暂停发卡 + 暂停抽奖”；
- 旧版本应用忽略新表；
- 回滚应用时保留新表和数据，不做破坏性降级；
- 现有未绑定 Key 在没有排序数据时继续按 `end_time asc, id asc`。

## 20. 测试计划

### 20.1 Model

- 两套奖池权重校验；
- 充值门槛公式和大阶段；
- 单订单跨多门槛；
- 回调幂等；
- 退款不重复跨门槛；
- 购买 `n` 张唯一键；
- 周期重置与惰性重置竞争；
- 停机跨多周期；
- 暂停期不发卡；
- 暂停恢复多次幂等；
- 三数据库迁移；
- SQLite 历史表新增字段。

### 20.2 Service

- 每个随机区间首尾边界；
- 并发 100 次同卡只成功一次；
- 幂等键重试；
- 发奖错误完整回滚；
- 订阅/充值两套奖池隔离；
- 充值额度固定加 60；
- 钱包赠金不进入充值账本；
- 套餐额度桶聚合；
- 双倍卡不复制 Key；
- 全额重置迁移 Key 和续费链；
- 5 小时准确到期；
- 在途旧实例结算不丢失。

### 20.3 Controller/Router

- 未登录、普通用户、管理员、Root 权限矩阵；
- IDOR；
- 参数和分页边界；
- Critical rate limit；
- 409 revision 冲突；
- 暂停状态错误码。

### 20.4 Frontend

- 概率表与服务端配置一致；
- 卡片来源和到期时间；
- 默认最早到期；
- 动画不影响结果；
- 网络中断恢复；
- reduced-motion；
- 桌面、平板、390px 移动端；
- 亮色、暗色、跟随系统；
- 消耗顺序拖拽和 revision 冲突；
- 六语言 JSON 与 TypeScript。

### 20.5 统计验证

使用注入式伪随机序列验证精确区间；另运行大量离线抽样，仅作为分布烟雾测试，不能用统计抽样替代边界单元测试。

## 21. 发布计划

### 阶段 A：只发布结构

- 本地完成迁移测试和构建；
- 生产以活动双暂停状态执行加法迁移；
- 验证旧充值、订阅、重置和 Key 扣费完全不变。

### 阶段 B：后台与规则

- 创建正式规则版本；
- 配置活动专用分组；
- 配置套餐 `n` 和周期赠卡；
- 完成公开规则页与法律复核；
- 保持发卡和抽奖暂停。

### 阶段 C：用户端灰度

- 开放活动页但仍显示“即将开始”；
- 用隔离测试用户和人工补偿卡验证全部奖项；
- 验证赠金不能购买订阅；
- 验证绑定/未绑定 Key 的排序边界。

### 阶段 D：正式开启

1. 先恢复抽奖；
2. 再恢复发卡；
3. 观察发卡、抽奖、成本、错误和概率偏差；
4. 真实充值、真实订阅购买和真实模型消费各完成端到端验收。

发布必须遵守：

- 本地构建、测试和产物校验；
- 生产不编译、不安装依赖、不运行测试；
- 真实 MySQL/Redis 绿环境迁移；
- 3010 验证后切流；
- 连接自然排空；
- 归一回 3000；
- 三公网入口和真实业务验收；
- 保留旧镜像和数据库备份。

## 22. 回滚

紧急停止优先顺序：

1. 暂停发卡；
2. 暂停抽奖；
3. 等待在途抽奖事务完成；
4. 回滚应用镜像。

数据库迁移均为加法，不删除新表。旧应用运行时保留活动数据，恢复新版后继续读取。不得通过删除幸运卡或抽奖记录完成回滚。

如奖品规则错误：

- 不原地改已激活规则；
- 暂停发卡和抽奖；
- 新建修正规则版本；
- 识别受影响卡和抽奖；
- 通过人工补偿流程处理；
- 保留原始规则和结果审计。

## 23. 预期改动文件

后端新增：

```text
model/lucky_campaign.go
model/lucky_card.go
model/lucky_recharge.go
model/subscription_priority.go
service/lucky_draw.go
service/lucky_pause.go
controller/lucky_wheel.go
```

后端修改：

```text
model/main.go
model/subscription.go
model/recharge_capacity.go（仅共享换算原语时，不能混用账本）
model/topup.go
service/subscription_reset_task.go
controller/topup*.go
controller/subscription*.go
router/api-router.go
```

Default 前端：

```text
web/default/src/routes/_authenticated/lucky-wheel/
web/default/src/features/lucky-wheel/
web/default/src/features/wallet/components/subscription-plans-card.tsx
web/default/src/features/wallet/components/my-subscriptions-detail.tsx
web/default/src/features/subscriptions/
web/default/src/i18n/locales/*.json
```

Classic 前端：

```text
web/classic/src/components/lucky-wheel/
web/classic/src/components/topup/
web/classic/src/helpers/subscriptionFormat.js
```

测试文件与 OpenAPI 文档必须同步新增。
