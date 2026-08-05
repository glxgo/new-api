# 多入口计费与虚拟会员技术实施规划

## 1. 文档目的

本文把《多入口计费与虚拟会员产品需求规划》映射到当前 `new-api` 的真实代码结构，定义可信入口识别、计费顺序、账务快照、双额度事务、支付复用、接口、前端、迁移、测试与发布顺序。

关联文档：

- `docs/product/multi-ingress-and-virtual-membership-prd.md`
- `pkg/billingexpr/expr.md`
- `CONTEXT.md`
- `CODEX_MEMORY.md`

本文是实现依据；实现、提交、推送和生产上线状态以文末及代码仓库当前证据为准。

> 实施状态（2026-08-05）：已在独立实现分支落地入口配置/Host 选择、固定点入口倍率、测速探针、虚拟会员额度账本、API Key 绑定、用户与管理端页面，并复用现有 Epay 客户端完成虚拟会员下单、异步通知、浏览器回跳、订单快照和幂等开通。后端定向编译、Linux/amd64 静态产物、默认前端 TypeScript/Rsbuild 和 Classic Rsbuild 已验证；生产 DNS/Nginx、支付网关配置和网站迁移仍待执行。虚拟会员同时保留钱包余额购买。

## 2. 现有系统结论

### 2.1 可以复用

| 现有能力 | 主要位置 | 规划中的复用方式 |
|---|---|---|
| 公共状态接口与系统设置 | `controller/misc.go`、`model/option.go`、`setting/` | 向前端提供公开入口列表，不复用 `ServerAddress` 表示多入口 |
| API Key 工作区 | `web/default/src/features/keys/components/api-keys-table.tsx` | 将单一 Base URL 改成双入口卡片和测速 |
| 固定/倍率/表达式计费 | `relay/helper/price.go`、`pkg/billingexpr/` | 在现有应扣结果之后统一应用入口倍率 |
| 统一预扣与结算 | `service/billing_session.go`、`service/funding_source.go` | 增加虚拟会员资金来源，复用预扣、差额和退款生命周期 |
| 订阅订单快照 | `model/subscription.go` | 复用“下单冻结、回调按快照签发”的原则和支付适配层 |
| API Key 具体实例绑定 | `model/subscription_binding.go` | 扩展为可区分普通订阅实例和虚拟会员实例的归属 |
| 订阅原子预扣记录 | `subscription_pre_consume_records` | 复用幂等设计，不与虚拟会员记录混表 |
| 侧栏模块控制 | `use-sidebar-data.ts`、`use-sidebar-config.ts`、Classic 对应文件 | 增加用户与管理端独立模块键 |
| 通知、审计与日志 | 现有服务 | 记录购买、绑定、额度不足和批量重置结果 |

### 2.2 不能直接复用

1. `ServerAddress` 同时用于 OAuth、Passkey、邮件链接和站点地址，不能改成多入口数组，也不能让海外直链 URL 取代它。
2. `UserSubscription` 只有一套 `amount_total/amount_used/next_reset_time`；虚拟会员需要周额度和可选 5 小时额度同时约束，不能只给原表增加别名字段。
3. 普通订阅的 `amount_cap_used` 是生命周期累计额度，周重置只清 `amount_used`；虚拟会员的两套短周期计数需要独立语义。
4. 入口折扣不是 `GroupRatio`。若把它写进分组倍率，会改变分组权限、价格展示、成本反推和历史配置含义。
5. 浏览器测速不能复用管理员“渠道测速”。渠道测速发生在服务器到上游，用户需要的是浏览器到两个公网入口的往返时间。
6. “自动成团”没有真实团成员关系，不应建立 group/member/invite/waiting 状态机。

## 3. 总体数据流

### 3.1 多入口计费

```text
用户请求
  -> 公网入口 Nginx 按 vhost 确认入口
  -> 覆盖写入可信入口标识
  -> New API 入口中间件解析配置快照
  -> 现有模型/分组/请求规则计算未折扣额度
  -> 入口倍率计算最终应扣额度
  -> 钱包 / 普通订阅 / 虚拟会员预扣
  -> 上游请求与最终用量结算
  -> 日志保存入口、倍率、未折扣与最终额度
```

### 3.2 虚拟会员消费

```text
API Key 鉴权与具体虚拟会员绑定
  -> 校验实例有效期、分组、模型
  -> 读取全站周周期版本并惰性归零旧周期
  -> 校验/滚动实例 5 小时窗口
  -> 同一事务锁定实例与幂等预扣记录
  -> 周额度和 5 小时额度同时增加预扣值
  -> 成功后按实际用量同时结算差额
  -> 失败时按 request_id 同时退款
```

## 4. 多入口设计

### 4.1 配置模型

建议新增 `api_ingress_profiles`：

| 字段 | 说明 |
|---|---|
| `id` | 主键 |
| `code` | 稳定标识，唯一，例如 `optimized`、`direct` |
| `display_name` | 前台名称 |
| `public_base_url` | 对外 Base URL，唯一 |
| `network_mode` | `optimized` / `direct` / 将来扩展值 |
| `billing_multiplier_ppm` | 固定点倍率，`1_000_000 = ×1.00` |
| `is_default` | 是否默认原价入口 |
| `enabled` | 是否接受计费请求 |
| `visible` | 是否在用户页展示 |
| `probe_enabled` | 是否允许公开测速 |
| `description` | 网络路径说明 |
| `sort_order` | 排序 |
| `config_version` | 乐观锁版本 |
| `created_at/updated_at` | 审计时间 |

约束：

- 倍率使用整数 PPM，禁止在账务路径使用浮点直接相乘；
- 首期倍率范围 `10_000–1_000_000`；
- 恰好一个启用的 `is_default=true`；
- URL 规范化后必须是 HTTPS、无 `/v1`、无查询参数、无尾斜杠；
- `ServerAddress` 继续保持站点主地址。

### 4.2 可信入口识别

不能信任客户端可提交的 `Host`、`X-Forwarded-Host` 或自定义入口头。

推荐边界：

1. 落地机应用仅监听回环或容器内网，不允许公网绕过 Nginx 直连；
2. 每个 Nginx vhost 先清除客户端同名头，再由服务器固定写入 `X-NewAPI-Ingress-ID`；
3. 三网线路机转发到落地机的专用 upstream/vhost，由落地机 Nginx 根据专用来源映射为 `optimized`；
4. 海外直链 vhost 由落地机 Nginx 映射为 `direct`；
5. New API 中间件只接受来自受控反向代理的映射结果；未知、禁用或倍率缺失的 relay 请求默认拒绝并告警；
6. `/api/status`、登录和普通网页路由不参与入口计费判定。

发布时先运行“只记录不改扣费”的观察模式，确认两个域名映射正确后再打开严格计费；不能直接以应用读取到的客户端 Host 作为财务事实。

### 4.3 计费顺序

定义：

```text
undiscounted_quota = 现有完整计费结果（含模型、分组、表达式、请求规则、FAST）
final_quota = round(undiscounted_quota × ingress_multiplier_ppm / 1_000_000)
```

入口倍率必须进入：

- `RelayInfo`；
- `PriceData`；
- tiered `BillingSnapshot`；
- 异步任务 `BillingContext`；
- 预扣、实际结算、退款和补扣；
- 同步、流式、WebSocket、图片、音频和任务计费；
- 用户日志与管理员日志。

FAST 或表达式请求规则应先计算，再打入口折扣，满足“在原来扣费基础上打折”。预扣和实际结算必须使用同一入口快照；请求执行期间管理员修改倍率不得改变该请求。

### 4.4 成本与利润

入口折扣降低用户售价，但不会自动降低上游成本。必须同时保存：

- `undiscounted_sale_quota`；
- `final_sale_quota`；
- `group_ratio`；
- `ingress_multiplier_ppm`；
- 最终渠道与 `channel_cost_ratio_ppm`。

平台成本仍按未折扣的官方基础成本计算。若从最终售价反推基础成本，除数必须同时包含分组倍率和入口倍率，不能因为售价打折而把上游成本也按比例打折。订阅/虚拟会员到期利润、分红或报表必须读取这一快照。

### 4.5 日志字段

正常消费日志至少增加：

- `ingress_code`；
- `ingress_name`；
- `ingress_multiplier_ppm`；
- `undiscounted_quota`；
- `final_quota`；
- `ingress_config_version`。

日志不保存内部 origin 地址、签名密钥、Nginx 私有 upstream 或用户完整 API Key。

### 4.6 测速探针

每个入口提供相同的公开轻量接口，例如 `GET /api/ingress/ping`：

- 不要求 API Key，不产生计费；
- 返回入口 `code`、服务器时间和固定小响应；
- `Cache-Control: no-store`；
- 支持网站域名跨域访问，`credentials: omit`；
- 独立速率限制，不进入上游渠道选择；
- 由 Nginx 和应用共同确认返回的入口 code 与 vhost 一致。

浏览器测量：

1. 两入口并行；
2. 每入口 1 次预热加 5 次有效样本；
3. 每次带随机查询参数防缓存，3 秒超时；
4. 展示 5 次样本的中位数，可附带抖动范围；
5. 任一失败不取消另一入口；
6. 不携带用户 API Key，不发真实模型请求；
7. 结果只保存在当前页面内存，默认不上传为用户网络画像。

## 5. 虚拟会员数据模型

### 5.1 `virtual_membership_plans`

| 字段 | 说明 |
|---|---|
| `id/code/title/subtitle/description` | 方案身份与文案 |
| `duration_seconds` | 有效期 |
| `weekly_quota_total` | 方案完整周额度 |
| `five_hour_enabled` | 是否启用 5 小时额度 |
| `five_hour_quota_total` | 方案完整 5 小时额度 |
| `allowed_group` | 适用分组，首期单值以复用现有权限模型 |
| `allowed_models` | 规范化模型匹配配置 |
| `allow_balance_pay` | 是否允许钱包本金支付 |
| `max_purchase_per_user` | 用户购买上限 |
| `recommended/enabled/sort_order` | 展示控制 |
| `created_at/updated_at` | 时间 |

管理员界面可用美元输入额度，但后端应通过 decimal 和当前 `QuotaPerUnit` 转成整数额度后保存/快照，禁止 float 累积。

### 5.2 `virtual_membership_variants`

| 字段 | 说明 |
|---|---|
| `id/plan_id` | 所属方案 |
| `group_size` | `1/2/3/4` |
| `price_amount/currency` | 管理员设置的价格 |
| `enabled` | 是否可购买 |
| `sort_order` | 展示顺序 |
| `created_at/updated_at` | 时间 |

唯一索引 `(plan_id, group_size)`。额度不在该表手填，统一由方案完整额度除以 `group_size` 计算。

### 5.3 `virtual_membership_orders`

保存：

- 用户、方案和档位；
- 支付方式、交易号、状态、金额；
- 完整方案快照；
- 完整档位快照；
- 计算后的实例周额度和 5 小时额度；
- 幂等创建键；
- 创建与完成时间。

支付回调只按订单快照创建实例，不能重新读取已被管理员修改的当前方案。

### 5.4 `user_virtual_memberships`

| 字段 | 说明 |
|---|---|
| `id/user_id/plan_id/order_id` | 归属 |
| `plan_snapshot/variant_snapshot` | 不可变权益快照 |
| `plan_title/group_size` | 常用展示字段 |
| `weekly_quota/weekly_used/weekly_epoch` | 周额度状态 |
| `five_hour_enabled` | 实例快照开关 |
| `five_hour_quota/five_hour_used` | 5 小时额度状态 |
| `five_hour_started_at/five_hour_next_reset_at` | 实例窗口 |
| `allowed_group/allowed_models` | 权益范围快照 |
| `start_time/end_time/status` | 生命周期 |
| `paid_revenue_quota/cost_accumulator` | 财务快照，是否分红由业务开关决定 |
| `created_at/updated_at` | 时间 |

### 5.5 `virtual_membership_weekly_epochs`

全站周周期使用事件版本，不对所有实例做一个长事务批量清零：

| 字段 | 说明 |
|---|---|
| `id/epoch` | 单调递增周期号 |
| `reset_at/next_reset_at` | 本次及下次时间 |
| `operator_id/reason` | 操作者与原因 |
| `idempotency_key` | 防重复提交 |
| `affected_count` | 操作时有效实例数快照 |
| `status/config_version` | 状态与版本 |
| `created_at` | 时间 |

重置事务只创建并激活一个新 epoch。实例读取或扣费时若 `weekly_epoch < active_epoch`，在实例锁内把 `weekly_used=0` 并同步 epoch。这样全站重置可快速原子生效，避免批量更新期间一部分用户已重置、另一部分尚未重置。

### 5.6 `virtual_membership_pre_consume_records`

每个 request_id 一行，保存：

- 用户、虚拟会员实例；
- 周额度预扣；
- 5 小时额度预扣；
- 最终销售额度、渠道成本快照；
- `reserved/provisional/final/refunded` 状态；
- 周 epoch 与 5 小时窗口开始时间；
- 创建与更新时间。

该表承担幂等预扣、差额结算和退款；不能只在日志 JSON 中记录后再扫描计算。

### 5.7 绑定与审计

推荐扩展 Token 的资金归属类型，而不是用同一个整数 ID 猜表：

- `entitlement_mode`: `auto/subscription/virtual_membership`；
- `subscription_id` 保留现有含义；
- `virtual_membership_id` 保存虚拟实例；
- 保存时保证两个实例 ID 不同时生效；
- 现有 Token 默认值保持旧行为。

归属历史增加前后权益类型和实例 ID，但不保存完整 API Key。

## 6. 虚拟会员事务

### 6.1 购买签发

1. 创建订单时锁定方案与档位，计算均分额度并写快照；
2. 支付成功回调按交易号和订单状态幂等；
3. 在同一事务把订单从 pending 改为 success，并创建唯一虚拟会员实例；
4. 钱包支付只能扣钱包本金，不得使用赠金；
5. 支付成功但通知失败不回滚权益，通知走事务后重试。

### 6.2 预扣

单事务步骤：

1. 根据 API Key 绑定锁定虚拟会员实例；
2. 校验状态、有效期、模型和分组；
3. 同步全站周 epoch，必要时归零周已用；
4. 处理 5 小时窗口：未开始则启动，已到期则开启新窗口；
5. 检查 request_id 是否已有记录；
6. 周剩余额度必须覆盖预扣；
7. 若启用 5 小时，5 小时剩余额度也必须覆盖预扣；
8. 创建幂等记录，同时增加周已用和 5 小时已用；
9. 返回实例和两套额度快照给 `RelayInfo`。

### 6.3 结算与退款

- 实际用量高于预扣：周额度和启用的 5 小时额度同时原子补扣；任一不足时进入现有欠费/错误处理，不能只更新一套。
- 实际用量低于预扣：两套额度按相同差额退回。
- 请求失败：按 request_id 幂等退款两套额度。
- 跨周重置或跨 5 小时窗口的长请求：结算仍归入预扣时冻结的周期记录，不能把差额写入新周期后造成旧预扣无法对账；新旧周期用量需由记录快照解释。
- 一个请求只消费一个虚拟会员实例，不跨实例拆分。

## 7. API 规划

### 7.1 公开与用户接口

```text
GET  /api/status                                  公开入口摘要
GET  /api/ingress/ping                            当前 vhost 测速探针
GET  /api/virtual-membership/announcement         独立公告
GET  /api/virtual-membership/plans                可购买方案与档位预览
GET  /api/virtual-membership/self                 我的有效/历史实例
GET  /api/virtual-membership/self/:id/keys        实例绑定 Key
PUT  /api/virtual-membership/self/:id/keys        确认替换绑定
POST /api/virtual-membership/balance/pay          钱包本金购买
POST /api/virtual-membership/{provider}/pay       外部支付创建订单
```

支付 provider 以现有已启用支付方式为准；必须抽取共用支付适配服务，不能复制多份签名/回调逻辑后各自漂移。

### 7.2 管理接口

```text
GET/POST/PUT/PATCH /api/ingress/admin/profiles
GET/POST/PUT/PATCH /api/virtual-membership/admin/plans
GET                 /api/virtual-membership/admin/instances
GET/PUT              /api/virtual-membership/admin/announcement
POST                 /api/virtual-membership/admin/weekly-reset/preview
POST                 /api/virtual-membership/admin/weekly-reset/execute
GET                  /api/virtual-membership/admin/weekly-reset/history
POST                 /api/virtual-membership/admin/instances/:id/invalidate
```

`execute` 必须携带 preview 返回的配置版本、幂等键和确认信息，防止管理员页面停留期间受影响范围变化后误操作。

## 8. 前端实施范围

### 8.1 Default

- API Key 桌面工作区和移动列表增加统一 `ApiIngressCard`；
- 新建 `/virtual-membership` 用户路由；
- 新建 `/virtual-memberships` 管理路由；
- 侧栏、模块开关、路由树和国际化同步；
- 复用现有语义色、卡片、进度条、对话框和 Drawer，不引入参考图的独立品牌色体系；
- 所有价格、额度和时间使用 tabular numbers，避免刷新时布局抖动。

### 8.2 Classic

- API Key 页、侧栏、购买页、实例卡和管理页提供同等功能；
- 可以复用 Classic 组件体系，但接口、字段、确认文案与 Default 一致；
- 不要求两套主题像素完全相同，要求业务行为和可见信息一致。

### 8.3 可访问性与响应式

- 交互目标不小于 44×44px；
- 图标使用现有 Lucide/Semi 图标，不用 emoji 充当结构图标；
- 测速、购买、重置均有 loading、disabled、成功和可恢复错误状态；
- 进度条同时提供文字百分比、已用/总额和下次重置时间；
- 错误区使用 `aria-live`/`role=alert`；
- 支持键盘操作、可见焦点、减少动态效果和明暗主题；
- 验证宽度至少覆盖 375、768、1024、1440。

## 9. 分阶段开发顺序

### 阶段 0：业务确认与隔离工作树

- 确认产品文档第 9 节的 6 个业务口径；
- 当前主工作树存在多组未提交改动，实施时从确认的精确提交创建独立 worktree/分支；
- 先整理并单独提交已完成的公告弹窗/钱包预览范围，再决定是否合并到新分支；
- 不在当前脏工作树中混写大规模数据库、支付和计费改造。

### 阶段 1：入口识别与影子观测

- 新表、管理接口、公开状态接口；
- Nginx 可信 vhost 映射方案；
- 中间件只记录入口，不改变扣费；
- 通过两入口真实请求核对 ingress code、渠道、模型和旧扣费一致。

### 阶段 2：入口倍率计费

- 固定点倍率进入同步、流式和异步账务快照；
- 修正平台成本反推；
- 增加日志和报表解释字段；
- 本地及隔离环境验证后，再从影子模式切为实际折扣。

### 阶段 3：双入口 UI 与测速

- API Key 桌面/移动双入口卡；
- 公开 ping、CORS、并行采样、中位数和错误状态；
- Default 与 Classic 验收。

### 阶段 4：虚拟会员核心账务

- 新表与三数据库迁移；
- 周 epoch、5 小时窗口、预扣/结算/退款；
- API Key 显式绑定和日志资金来源；
- 不接支付，先用测试/管理员签发覆盖完整账务回归。

### 阶段 5：商品、支付与后台

- 方案/档位/公告管理；
- 下单快照、钱包本金支付、现有外部支付适配；
- 全站周重置预览、执行和审计；
- 实例查询、作废和绑定管理。

### 阶段 6：用户页面与完整验收

- 购买卡、自动成团选择、购买确认；
- 我的虚拟会员双进度卡；
- 侧栏与模块权限；
- 真实支付沙箱和失败恢复；
- 本地构建、隔离数据库迁移、候选环境和生产近零中断发布。

## 10. 测试矩阵

### 10.1 多入口

- 客户端伪造 Host/Forwarded/Ingress 头不能改变倍率；
- 未知、禁用、缺失倍率入口按策略拒绝；
- `×1.00` 与旧计费逐项一致；
- `×0.95` 在固定价、倍率价、tiered expression、FAST、工具调用、流式、异步任务中一致；
- 预扣、补扣、少扣退款和失败退款使用同一入口快照；
- 中途修改配置不影响在途请求；
- 平台成本不随用户折扣错误下降；
- 钱包、普通订阅、虚拟会员消耗均按最终额度；
- ping 无计费、无鉴权泄露、无缓存污染并受限流保护；
- 浏览器跨域、超时和单线路失败可恢复。

### 10.2 虚拟会员

- 1/2/3/4 人额度向下均分及极小额度边界；
- 方案编辑不改变订单快照和现有实例；
- 支付回调重试只创建一个实例；
- 钱包赠金不能购买；
- 周 epoch 重置立即生效且重复提交幂等；
- 重置与并发预扣、退款、结算竞争时账目一致；
- 5 小时关闭时完全不读写该限制；
- 首次消费开启窗口，过期后懒重置；
- 周够但 5 小时不足、5 小时够但周不足都必须拒绝；
- 结算差额和退款同时调整两套额度；
- 到期、作废、模型不兼容和分组不兼容均拒绝；
- 旧 Key、普通订阅和钱包优先级无回归；
- 用户隔离、管理员权限和批量重置权限正确；
- 日志能显示资金来源、实例、两套额度和入口倍率，但不泄露完整 Key。

### 10.3 构建与兼容

- SQLite、MySQL、PostgreSQL 迁移与核心事务测试；
- model/service/controller 定向测试及 race 测试；
- Go 完整编译；
- Default TypeScript、ESLint、国际化同步和生产构建；
- Classic 生产构建；
- `git diff --check`；
- Linux/amd64 静态产物在本地构建，不在生产服务器编译。

## 11. 迁移与发布

1. 新表全部为空上线，不迁移普通订阅或历史订单。
2. 初始化 `optimized` 为 `×1.00`；`direct` 先配置为 `×1.00` 并影子观测，确认入口识别后再改目标倍率。
3. 虚拟会员功能默认关闭，先完成管理员测试签发和隔离数据库验收。
4. 数据库变更保持 additive，旧版本应用应能在回滚时忽略新表/新列。
5. 发布前备份数据库和 Nginx 配置，记录 SHA-256；候选实例连接真实 MySQL/Redis 做只读及测试账号验收。
6. 先发布应用兼容层，再切 Nginx 入口标识，最后启用折扣；每步都可独立回退。
7. 切流采用候选端口、健康检查、公网双入口检查和自然排空，表述为“近零中断/接近无感”，不强断长流。
8. 上线后重点监控：未知入口数、各入口请求/扣费、倍率分布、退款失败、虚拟会员预扣状态、周 epoch、5 小时拒绝率、支付回调幂等和利润异常。

## 12. 完成定义

只有同时满足以下条件才视为功能完成：

- 产品待确认项已由运营者确认并写入文档；
- 两入口身份不可由客户端伪造；
- 所有计费路径和成本账务通过回归；
- 虚拟会员周/5 小时额度在并发、退款和跨周期场景下原子且幂等；
- 单买与自动成团档位的订单快照和支付回调可靠；
- Default/Classic、桌面/移动、明暗主题均验收；
- 旧用户、旧 Key、钱包和普通订阅无行为变化；
- 本地构建与测试通过，生产只部署已验证产物；
- 生产双入口、真实测试 Key、日志和余额差异经过持续观察。
