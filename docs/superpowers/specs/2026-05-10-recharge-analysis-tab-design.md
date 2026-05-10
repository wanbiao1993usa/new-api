# 充值分析 Tab 设计文档

## 背景

现有“计费分析”页面只回答消费侧问题，事实源明确限定为 `logs` 表中的 `LogTypeConsume` 消费日志，不包含充值、订阅购买、失败订单、待支付订单等资金流入信息。

当前运营需要新增一块管理员专用视图，用于回答：

1. 在指定时间范围内，充值和订阅一共带来了多少资金流入。
2. 有多少用户付费过，有多少用户从未付费。
3. 付费用户当前账户里还剩多少总余额。
4. 哪些用户买了但没有使用，哪些用户余额高但长期不活跃，哪些用户已经形成较高付费价值。

这类问题与现有消费分析口径不同，不能继续塞进现有 `/api/billing/analysis` 的响应结构中，否则会把消费事实和资金流入口径混在一起。

## 目标

在现有“计费分析”页面中新增一个管理员专用的 `充值分析` tab，用独立接口和独立聚合逻辑回答以下问题：

1. 指定时间范围内成功充值金额、成功订阅金额、成功订单数、成功付费用户数。
2. 同一时间范围内待支付订单数、失败订单数，辅助观察转化漏斗上半段。
3. 全站所有用户中，哪些用户历史上付费过，哪些没有。
4. 每个用户在所选时间范围内的付费表现，以及其当前账户总余额、历史累计消耗、最近使用时间。
5. 支付方式、订单类型、订单状态的聚合分布。

## 非目标

第一版不做以下内容：

1. 不替代现有“计费分析”的消费分析逻辑。
2. 不尝试精确回答“充值来源余额还剩多少”“订阅来源余额还剩多少”。
3. 不重做完整财务利润报表，不计算供应商成本和净利润。
4. 不替代逐单审计页，不承载完整订单流水明细查询。
5. 不做退款净额、退款冲销后的最终净流入口径。
6. 不重构现有用户余额模型，不新增精细额度来源账本。

## 已确认范围与默认口径

### 访问权限

仅管理员可见。普通用户不展示该 tab，也没有对应接口。

### 统计范围

“充值分析”同时统计两类资金流入：

1. 钱包充值订单。
2. 订阅购买订单。

### 时间口径

页面提供可自定义时间范围，但不同状态采用不同时间字段：

1. 成功充值、成功订阅相关指标，按 `支付成功时间 / 完成时间` 统计。
2. 待支付、失败订单数量，按 `订单创建时间` 统计，并在前端文案中明确标注。

这样做的原因是：

1. 成功指标的业务目标是回答“这段时间实际进来了多少钱”，应以成功完成时间为准。
2. 待支付订单通常没有 `complete_time`，失败订单的完成时间也不稳定，必须按 `create_time` 统计才有一致性。

### 用户口径

用户明细表默认覆盖全站所有用户，而不是只显示付费用户。

“是否付费过”的定义为：历史上存在至少一笔成功充值订单或成功订阅订单。

页面提供付费状态筛选：

1. 全部用户
2. 仅付费用户
3. 仅未付费用户

### 余额口径

页面中的“还剩多少”统一展示 `当前账户总余额`，直接读取 `users.quota` 当前值。

前端文案必须明确写成“当前账户总余额”，不能写成“充值剩余额度”或“订阅剩余额度”，因为当前余额会被多来源共同影响，包括但不限于：

1. 在线充值
2. 订阅附带的余额型入账
3. 兑换码充值
4. 管理员调额
5. 邀请奖励
6. 签到奖励

### 成功与失败展示方式

首页以成功指标为主，同时补充待支付和失败订单数作为辅助指标，不把它们混进成功金额或成功用户数。

## 页面结构

页面位于现有“计费分析”页内部，新增一个 `充值分析` tab。现有消费分析 tab 保持不变。

### 1. 筛选区

筛选项：

1. 时间范围
2. 用户名
3. 用户 ID
4. 付费状态：全部 / 仅付费 / 仅未付费
5. 订单类型：全部 / 充值 / 订阅
6. 订单状态：全部 / 成功 / 待支付 / 失败
7. 支付方式：Stripe / Creem / Waffo / Epay / 其他

默认值：

1. 时间范围默认今天开始到当前时刻
2. 付费状态默认全部
3. 订单类型默认全部
4. 订单状态默认全部
5. 支付方式默认全部

### 2. 总览卡片区

第一排卡片：

1. 成功充值金额
2. 成功订阅金额
3. 成功订单数
4. 成功付费用户数
5. 待支付订单数（区间内创建）
6. 失败订单数（区间内创建）

第二排辅助卡片：

1. 付费用户当前账户总余额合计
2. 付费后有消费用户数
3. 付费后未消费用户数
4. 高余额未活跃用户数

卡片文案要求：

1. 所有金额都沿用系统当前额度显示规则。
2. 待支付和失败卡片必须显式标注“按创建时间统计”。
3. “当前账户总余额”必须显式标注为“当前”，避免被理解为区间期末余额。

### 3. 明细区

至少包含两个子 tab：

1. 用户明细
2. 订单汇总

#### 用户明细

一行一个用户，建议字段：

1. 用户 ID
2. 用户名
3. 当前分组
4. 注册时间
5. 最近登录时间
6. 是否付费过
7. 是否充值过
8. 是否订阅过
9. 首次付费时间
10. 最近付费时间
11. 区间成功充值金额
12. 区间成功订阅金额
13. 区间成功订单数
14. 当前账户总余额
15. 历史累计消耗额度
16. 历史消费日志数
17. 最近使用时间
18. 标签

标签第一版默认生成以下几类：

1. `paid_not_consume`: 历史付费过，但没有任何消费记录。
2. `paid_in_range_not_consume_after_pay`: 在所选时间范围内有成功付费，但最近一次区间付费后没有消费记录。
3. `high_balance_inactive`: 当前余额高于阈值，且最近使用时间距离当前超过阈值。
4. `high_value_user`: 历史累计成功付费金额高于阈值。
5. `low_balance_active`: 当前余额低于阈值，但最近仍有使用。

默认排序：

1. 最近付费时间降序

支持排序：

1. 当前账户总余额降序
2. 区间成功付费金额降序
3. 历史累计消耗额度降序
4. 最近使用时间降序

#### 订单汇总

第一版不做逐单列表，而做聚合汇总，至少包含：

1. 按支付方式汇总：成功金额、成功订单数、待支付订单数、失败订单数、成功付费用户数
2. 按订单类型汇总：充值 / 订阅
3. 按订单状态汇总：成功 / 待支付 / 失败

## 数据源与口径说明

### 充值订单

钱包充值来源于 `top_ups` 表。

字段见：

1. `money`
2. `status`
3. `payment_method`
4. `payment_provider`
5. `create_time`
6. `complete_time`

设计要求：

1. 钱包充值统计直接使用 `top_ups` 作为结构化订单事实源。
2. 不从 `LogTypeTopup` 日志反推充值金额。
3. 不使用 `top_ups.amount` 作为统一金额统计口径。

### 订阅订单

订阅购买来源于 `subscription_orders` 表。

字段见：

1. `money`
2. `status`
3. `payment_method`
4. `payment_provider`
5. `create_time`
6. `complete_time`

设计要求：

1. 订阅统计直接使用 `subscription_orders` 作为结构化订单事实源。
2. 不依赖订阅成功时生成的 `LogTypeTopup` 文本日志。
3. 不依赖订阅同步写入的 `top_ups` 镜像行。

### 用户余额

当前账户总余额读取 `users.quota`。

历史累计消耗额度读取 `users.used_quota`。

最近使用时间与消费日志数来自 `logs` 表中的 `LogTypeConsume`。

## 关键语义约束

### 为什么不能直接用现有计费分析扩展

现有计费分析只查消费日志 `LogTypeConsume`，其结果是消费事实，不包含结构化支付订单。把充值/订阅指标直接混入现有响应会产生以下问题：

1. 消费金额和资金流入金额处于不同事实表。
2. 充值成功、订阅成功、失败、待支付没有统一落在消费日志中。
3. 用户当前余额与消费日志不是一个时间维度上的同类事实。

因此必须单独设计聚合逻辑与接口。

### 为什么不能回答“充值来源剩余额度”

当前系统只有 `users.quota` 这一总余额字段，没有“充值余额桶”“订阅余额桶”“赠送余额桶”。消费时也没有按来源扣减的结构化台账，因此无法可靠回答：

1. 钱包充值买来的额度还剩多少
2. 订阅带来的额度还剩多少

第一版必须明确展示的是“当前账户总余额”。

## 后端接口设计

新增接口：

1. `GET /api/billing/recharge_analysis`

仅管理员可访问。

### 查询参数

1. `start_timestamp`
2. `end_timestamp`
3. `username`
4. `user_id`
5. `paid_status`
6. `order_type`
7. `order_status`
8. `payment_method`
9. `p`
10. `page_size`
11. `sort_by`
12. `sort_order`

### 响应结构

```json
{
  "summary": {
    "topup_success_money": 0,
    "subscription_success_money": 0,
    "success_order_count": 0,
    "success_paid_user_count": 0,
    "pending_order_count": 0,
    "failed_order_count": 0,
    "paid_users_current_quota_sum": 0,
    "paid_after_consume_user_count": 0,
    "paid_not_consume_user_count": 0,
    "high_balance_inactive_user_count": 0
  },
  "payment_method_summary": [],
  "order_type_summary": [],
  "status_summary": [],
  "users": {
    "items": [],
    "total": 0
  }
}
```

### 用户行结构

```json
{
  "user_id": 0,
  "username": "",
  "group": "",
  "created_at": 0,
  "last_login_at": 0,
  "has_paid": false,
  "has_topup": false,
  "has_subscription": false,
  "first_paid_at": 0,
  "last_paid_at": 0,
  "topup_success_money": 0,
  "subscription_success_money": 0,
  "success_order_count": 0,
  "current_quota": 0,
  "used_quota": 0,
  "consume_request_count": 0,
  "last_consume_at": 0,
  "tags": []
}
```

## 聚合逻辑设计

### 成功流入指标

1. 成功充值金额：所选时间范围内，`top_ups.status = success` 且 `complete_time` 落在区间内的钱包充值 `money` 总和。
2. 成功订阅金额：所选时间范围内，`subscription_orders.status = success` 且 `complete_time` 落在区间内的 `money` 总和。
3. 成功订单数：上述两类成功订单数量之和。
4. 成功付费用户数：所选时间范围内至少有一笔成功充值或成功订阅的去重用户数。

### 非成功辅助指标

1. 待支付订单数：`status = pending`，按 `create_time` 落区间统计。
2. 失败订单数：`status = failed` 或 `status = expired`，按 `create_time` 落区间统计。

### 用户付费事实

用户维度需要两类事实同时存在：

1. 历史事实：是否曾付费、首次付费时间、最近付费时间。
2. 区间事实：当前筛选时间范围内的成功充值金额、成功订阅金额、成功订单数。

### 用户消费与余额

1. `current_quota` 直接取 `users.quota`
2. `used_quota` 直接取 `users.used_quota`
3. `consume_request_count` 取 `logs` 中该用户的 `LogTypeConsume` 总数
4. `last_consume_at` 取该用户最近一条消费日志时间

### 行为标签

标签生成逻辑：

1. `paid_not_consume`: `has_paid = true` 且 `consume_request_count = 0`
2. `paid_in_range_not_consume_after_pay`: 所选时间范围内有成功付费，且最近一次区间付费后没有消费日志
3. `high_balance_inactive`: `current_quota >= balance_threshold` 且 `now - last_consume_at >= inactive_threshold`
4. `high_value_user`: 历史成功付费总金额 >= value_threshold
5. `low_balance_active`: `current_quota <= low_balance_threshold` 且最近使用时间在活跃阈值内

第一版阈值写死在后端常量即可，不做系统设置项。

## 实现策略

第一版采用“独立聚合型”实现：

1. 前端只是在现有“计费分析”页面里新增一个 tab。
2. 后端单独新增 recharge analysis 的 model/service/controller 逻辑。
3. 不复用现有 `GetBillingAnalysis` 响应结构。

### 建议代码落点

后端：

1. `model/recharge_analysis.go`
2. `controller/recharge_analysis.go`
3. `router/api-router.go` 新增管理员路由

前端：

1. `web/src/pages/BillingAnalysis/index.jsx` 新增顶层 tab 切换
2. 新增 `web/src/pages/BillingAnalysis/RechargeAnalysisTab.jsx`
3. 如有必要再拆 `SummaryCards`、`UsersTable`、`OrderSummaryTables`

## 分页、排序与性能

### 用户表

用户表必须服务端分页，不能一次性把全站用户全部下发到前端。

### 排序

排序字段只支持白名单，避免前端传任意 SQL 片段：

1. `last_paid_at`
2. `current_quota`
3. `topup_success_money`
4. `subscription_success_money`
5. `success_order_count`
6. `used_quota`
7. `last_consume_at`

### 聚合方式

推荐做法是：

1. 分别构建充值成功聚合子查询、订阅成功聚合子查询、消费聚合子查询。
2. 以 `users` 为主表做左连接。
3. summary 再单独跑无分页聚合。

这样可以同时满足：

1. 全站用户视图
2. 付费状态筛选
3. 服务端排序与分页

### 跨数据库约束

必须兼容 SQLite、MySQL、PostgreSQL：

1. 尽量使用 GORM 子查询和聚合能力。
2. 如需 raw SQL，必须保证三库语法兼容。
3. 不使用数据库方言专属的窗口函数或 JSON 语法作为第一版依赖。

## 文案要求

1. “还剩多少”统一写成“当前账户总余额”。
2. “充值分析”中的“成功充值金额”“成功订阅金额”是资金流入口径，不是消费口径。
3. 待支付/失败卡片需要显式说明是“区间内创建订单数”。
4. 用户标签尽量短而明确，避免解释型长句。

## 测试要求

后端测试至少覆盖：

1. 充值成功与订阅成功同时存在时的 summary 聚合。
2. 待支付/失败订单按 `create_time` 统计。
3. 成功订单按 `complete_time` 统计。
4. 全站用户视图下，未付费用户也能出现。
5. `paid_status` 三种筛选行为正确。
6. 订阅镜像写入 `top_ups` 不会被误计入钱包充值金额。
7. 当前余额显示为 `users.quota`，不会错误回推来源余额。
8. 排序和分页稳定。

前端测试/验收至少覆盖：

1. 管理员能看到新 tab，普通用户看不到。
2. 筛选项能驱动接口刷新。
3. summary 卡片与表格字段可正常展示空态、零值和长用户名。
4. 移动端和桌面端布局可用。

## 上线顺序建议

1. 先完成后端聚合与接口测试。
2. 再接前端 tab 与用户明细表。
3. 最后补支付方式/订单类型/订单状态聚合展示。

## 后续扩展

未来如果需要精确回答“充值买来的还剩多少”，需要单独立项补“额度来源账本”：

1. 每次入账记录来源类型与金额
2. 每次消费按来源扣减
3. 支持余额桶级别的剩余统计

这一扩展不属于本次第一版。
