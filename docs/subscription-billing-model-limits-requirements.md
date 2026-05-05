# 订阅分组扣费与模型限额需求文档

## 背景

当前系统已经通过“分组”决定请求使用的渠道和倍率。一个 API 请求进入系统后，会先根据用户分组、令牌分组和 `token.Group = auto` 的自动选组规则确定本次请求的最终使用组。最终使用组会影响渠道选择、倍率计算和消费日志。

现有订阅更接近一个用户级额度包。订阅扣费是否优先使用，主要由用户的扣费策略决定，而不是由最终使用组决定。这会导致两个问题：

- 订阅专属分组的请求在订阅不足时可能继续扣用户余额。
- 普通余额分组的请求可能消耗订阅额度。

同时，现有订阅只支持套餐总额度，不支持按模型限制额度，无法针对高成本模型做单独保护。

## 目标

本次改造目标是让订阅套餐可以作为明确的“分组权益 + 模型额度限制”机制使用。

目标行为：

- 请求先确定最终使用组。
- 根据最终使用组决定扣费来源。
- 订阅专属分组只扣订阅，不回退扣余额。
- 余额分组只扣余额，不消耗订阅。
- 订阅套餐支持按模型设置额度限制。
- 用户端只展示用户购买后实际可用的模型限额。

## 非目标

本需求不包含以下内容：

- 不改变现有渠道路由模型。
- 不把订阅模型限额绑定到具体渠道。
- 不用请求速率限制替代模型额度限制。
- 不要求立即移除现有用户扣费策略。
- 不要求重做 `token.Group = auto` 自动选组机制。

## 现有请求分组逻辑

当前请求的使用组确定逻辑保持不变：

- 如果 token 没有设置分组，则使用用户当前分组。
- 如果 token 设置了具体分组，则先检查用户当前分组是否允许访问该 token 分组。
- 如果不允许访问，则请求失败。
- 如果允许访问，且 token 分组不是 auto，则该 token 分组就是本次请求的使用组。
- 如果 token 分组是 `auto`，则系统根据 auto token 规则选择一个真实分组。
- `auto` 不是最终扣费分组，最终扣费应使用 auto token 选出的真实分组。

## 目标扣费逻辑

新增分组扣费类型。请求确定最终使用组后，系统按最终使用组的扣费类型决定资金来源。

分组扣费类型：

- 仅订阅：只允许扣订阅。没有有效订阅、订阅额度不足、模型额度不足时，请求失败，不允许回退扣余额。
- 仅余额：只允许扣用户余额，不消耗订阅额度。
- 保持现状：继续使用当前用户扣费策略，例如优先订阅、优先余额、仅订阅、仅余额。

仅订阅分组的特殊规则：

- 即使当前模型免费、分组倍率为 0、或预扣额度计算结果为 0，也必须要求用户存在有效订阅。
- 0 预扣只做订阅权益校验和订阅实例锁定，不应为了创建预扣记录而强行扣 1 个额度。
- 如果后续实际结算额度仍为 0，则不增加订阅总使用量和模型使用量。

推荐业务配置：

- 无限战争分组：仅订阅。
- 普通余额分组：仅余额。
- 未明确迁移的历史分组：保持现状。

## 订阅模型限额

订阅套餐需要新增模型限额配置。

模型限额含义：

- 限制某个模型在当前订阅重置周期内最多可消耗的订阅额度。
- 限额单位使用系统原生额度，不使用展示层货币字符串。
- 模型限额绑定平台对外模型名，不绑定渠道，也不绑定上游真实模型名。

模型限额匹配规则：

- 如果套餐配置了当前请求模型的限额，则使用该模型限额。
- 如果没有配置当前模型，但配置了默认模型限额，则使用默认模型限额。
- 如果二者都没有配置，则该模型不做单独限额，只受订阅总额度限制。

## 订阅扣费校验

当请求需要使用订阅扣费时，系统需要同时检查套餐总额度和模型额度。

预扣费阶段：

- 查找用户有效订阅。
- 如果订阅已到重置时间，先执行订阅重置。
- 检查订阅总额度是否足够。
- 检查当前模型额度是否足够。
- 两者都足够，才允许预扣。
- 任一不足，请求失败。

结算阶段：

- 根据实际消耗补扣或返还。
- 总额度和模型额度需要一起调整。
- 总额度和模型额度在预扣阶段作为准入硬检查；结算阶段允许本次请求的实际消耗超过预扣额度，并允许使用量因此超过套餐总额度或模型限额。
- 结算后超过总额度或模型限额的订阅，后续请求在预扣阶段应因为额度不足而失败。

退款阶段：

- 请求失败需要退回预扣额度。
- 总额度和模型额度需要一起退回。

重置阶段：

- 周期重置时清零订阅总使用量。
- 同时清零该订阅下的模型使用量。

## 数据记录需求

需要记录订阅下每个模型的使用量。

建议新增一类记录：

- 用户订阅 ID。
- 模型名。
- 已使用额度。
- 创建和更新时间。

订阅预扣记录需要保存模型名。否则后续补扣、返还和退款时无法知道应该调整哪个模型的使用量。

订阅预扣记录需要支持 0 预扣场景：

- 0 预扣记录用于证明本次请求已完成订阅权益校验，并锁定本次使用的订阅实例。
- 0 预扣记录不增加订阅总使用量和模型使用量。
- 如果结算阶段实际额度大于 0，应基于该记录锁定的订阅实例和模型名进行补扣。

建议所有订阅扣费请求都记录模型使用量，不只记录已配置模型限额的模型。这样管理员后续给某模型新增限额时，当前周期内已经发生的该模型订阅用量也能被正确纳入判断。

## 用户端展示

用户端展示订阅模型限额时，需要过滤不可用模型。

展示规则：

- 管理员端展示套餐完整配置。
- 用户端只展示用户购买后实际可用模型的限额。
- 如果套餐配置了升级分组，应先计算用户购买后的用户组。
- 如果套餐没有配置升级分组，则使用用户当前分组。
- 基于购买后的用户组，计算该用户组可访问的实际使用组。
- 只展示这些实际使用组里会走订阅扣费的模型限额。
- 用户不可用的模型限额不展示。

示例：

- 套餐配置了 gpt-5.5 限额。
- 用户购买后所在分组不能使用 gpt-5.5。
- 用户端不展示 gpt-5.5 限额。

## auto token 处理

`token.Group = auto` 只作为自动选组入口，不作为最终扣费分组。

处理规则：

- token 分组为 `auto` 时，先按现有 auto token 规则选出真实分组。
- 后续分组扣费类型、模型限额、日志展示均应基于真实分组。

建议限制：

- 不建议让同一个 auto token 同时覆盖仅订阅分组和仅余额分组。
- 更稳妥的方式是无限战争使用固定分组 token，不纳入普通 auto token 链路。
- 如果保留 auto token 跨分组重试，必须保证同一个 auto token 候选链路内的分组扣费类型一致。
- 如果无法保证候选链路内扣费类型一致，则请求预扣后必须锁定本次扣费类型，后续重试只能切换到相同扣费类型的真实分组，不能切换到不同扣费类型的分组。

## 失败返回

需要区分不同失败原因，便于用户理解：

- 无有效订阅。
- 订阅总额度不足。
- 当前模型订阅额度不足。
- 用户余额不足。
- 用户无权访问该分组。

仅订阅分组失败时，不应出现“已回退扣余额”的行为。

## 管理后台需求

订阅套餐管理需要新增：

- 模型限额配置入口。
- 支持从系统已知模型中选择模型。
- 支持默认模型限额。
- 支持输入原生额度。
- 支持在列表或详情中查看套餐模型限额。

分组管理需要新增：

- 分组扣费类型配置。
- 默认值应为保持现状，避免影响已有业务。

## 兼容性要求

为降低改造风险：

- 现有分组默认扣费类型为保持现状。
- 未配置模型限额的套餐保持现有总额度逻辑。
- 未配置订阅模型使用量记录的历史订阅不应失效。
- 数据库改造需要兼容 SQLite、MySQL 和 PostgreSQL。

## 验收标准

基础分组扣费：

- 无限战争分组配置为仅订阅后，请求只扣订阅。
- 无限战争订阅不足时，请求失败，不扣余额。
- 普通余额分组配置为仅余额后，请求只扣余额，不消耗订阅。

模型限额：

- 套餐总额度充足但当前模型限额不足时，请求失败。
- 当前模型限额充足但套餐总额度不足时，请求失败。
- 仅订阅分组即使预扣额度为 0，也必须要求有效订阅；校验成功后不应强行扣 1 个额度。
- 请求成功后，总使用量和模型使用量同时增加。
- 请求失败后，总使用量和模型使用量同时退回。
- 周期重置后，总使用量和模型使用量同时清零。
- 实际消耗超过预扣额度时，本次请求允许成功，结算后总使用量或模型使用量可以超过对应限额。
- 订阅或模型用量已超过限额后，后续请求在预扣阶段应失败。

展示：

- 管理员能看到套餐完整模型限额。
- 用户只能看到购买后实际可用模型的限额。
- 用户不可用模型不展示。

auto：

- auto 选中真实分组后，按真实分组扣费类型处理。
- auto 不应直接作为扣费类型判断对象。
- auto token 不应跨越不同扣费类型的真实分组重试。

## 分组可访问与可见性

现有系统里“用户可访问分组”和“创建 token 时可见分组”基本使用同一套配置。这个设计会影响订阅升级后的旧 token 兼容。

目标上需要区分两个概念：

- 可访问分组：用于请求鉴权，决定旧 token 是否还能继续调用。
- 可见分组：用于前端创建 token 时展示，决定用户能不能新建某个分组的 token。

典型场景：

- 用户原来有普通余额分组 token。
- 用户升级到无限战争用户组。
- 旧余额 token 应继续可用，并按余额分组扣费。
- 但用户新建 token 时，可以只展示无限战争分组，不展示旧余额分组。

如果不拆分这两个概念，只能在“旧 token 可用”和“旧分组不可见”之间二选一。

## 多订阅处理

现有订阅扣费逻辑会从用户 active subscriptions 中选择一个可用订阅扣费，不会把多个订阅的额度自动合并。

本需求默认沿用该逻辑：

- 每次请求只从一个订阅实例扣费。
- 订阅总额度和模型额度都在同一个订阅实例内判断。
- 多个订阅不会叠加成一个总额度池或模型额度池。

如果未来需要订阅额度叠加，需要另起需求设计。

## 套餐规则是否影响已购买订阅

本需求默认采用“运行时读取套餐模型限额配置”的方式：

- 用户订阅实例继续快照 `AmountTotal`。
- 模型限额暂不快照到 `UserSubscription`。
- 管理员修改套餐模型限额后，会影响该套餐下仍然 active 的订阅。

管理端需要提示这一点。

如果未来希望用户购买时权益完全固定，需要把模型限额也快照到 `UserSubscription`，这会增加数据结构和迁移复杂度。

## 后续代码阅读重点

后续需要重点阅读并确认以下模块：

- 请求鉴权与使用组确定。
- 渠道分发与 auto token 真实分组选择。
- 分组倍率配置和分组管理。
- 订阅套餐模型、用户订阅模型、预扣记录。
- 订阅预扣、结算、退款、重置任务。
- 用户端订阅展示和管理员套餐编辑页面。

## 实施改造说明

本节记录具体需要修改的位置、修改内容和目标行为。后续开发应以本节作为执行清单。

## 一、分组扣费类型

### 需要修改的位置

- `setting/ratio_setting/group_ratio.go`
- `model/option.go`
- `controller/option.go`
- `web/src/pages/Setting/Ratio/GroupRatioSettings.jsx`
- `web/src/pages/Setting/Ratio/components/GroupTable.jsx`
- `service/billing_session.go`
- `service/billing.go`
- `relay/helper/price.go`
- 必要时新增 `setting/ratio_setting/group_billing.go`

### 需要新增的配置

新增分组扣费类型配置，建议作为独立配置项保存，不直接混入分组倍率。

建议配置名：

- 后端配置项：`GroupBillingType`
- JSON 格式：`{"分组名":"扣费类型"}`

扣费类型值：

- `default`：保持现状，继续按用户 `billing_preference`。
- `subscription_only`：仅订阅。
- `wallet_only`：仅余额。

默认行为：

- 未配置的分组视为 `default`。
- 这样可以避免升级后影响已有业务。

示例：

```json
{
  "无限战争": "subscription_only",
  "account2": "wallet_only",
  "0.7折组": "wallet_only"
}
```

### 后端配置读写

需要实现：

- 读取完整配置。
- 通过 JSON 字符串更新配置。
- 根据分组名读取扣费类型。
- 校验扣费类型是否合法。

建议函数：

- `GetGroupBillingType(group string) string`
- `GroupBillingType2JSONString() string`
- `UpdateGroupBillingTypeByJSONString(jsonStr string) error`
- `CheckGroupBillingType(jsonStr string) error`

`model/option.go` 需要把新配置放进 `OptionMap`，并在更新配置时写回内存配置。

`controller/option.go` 需要在保存配置前校验 JSON。

### 前端分组设置

倍率设置页面需要增加分组扣费类型编辑入口。

建议展示在“倍率设置”或新增一个“扣费类型”标签页中。

管理员需要能配置：

- 分组名。
- 扣费类型。
- 说明。

可选值展示为：

- 保持现状。
- 仅订阅。
- 仅余额。

### 扣费入口改造

现有 `NewBillingSession` 只读取用户 `billing_preference`。需要在进入现有策略前，先根据最终使用组读取分组扣费类型。

目标逻辑：

- 如果最终使用组是 `subscription_only`，强制走订阅资金来源。
- 如果最终使用组是 `wallet_only`，强制走钱包资金来源。
- 如果最终使用组是 `default`，继续使用现有用户扣费策略。

注意：

- `auto` 不是最终使用组。
- `token.Group = auto` 时，必须先解析出真实分组，再读取真实分组的扣费类型。
- 预扣费后，本次请求的扣费分组应保持稳定。后续重试不应切换到不同扣费类型的分组。

### 最终使用组获取规则

扣费类型必须基于最终真实分组判断。

规则：

- 如果上下文存在 `auto_group`，使用 `auto_group`。
- 否则使用 `relayInfo.UsingGroup`。

建议新增一个统一函数，避免各处重复判断。

建议函数：

- `ResolveBillingGroup(c, relayInfo) string`

注意：

- 不允许直接用 token 分组判断扣费类型。
- 不允许直接用 `auto` 判断扣费类型。

### 与现有用户扣费策略的关系

分组扣费类型优先级高于用户扣费策略。

优先级：

1. 分组扣费类型是仅订阅或仅余额时，按分组强制执行。
2. 分组扣费类型是保持现状时，才读取用户 `billing_preference`。

## 二、订阅套餐模型限额配置

### 需要修改的位置

- `model/subscription.go`
- `model/main.go`
- `controller/subscription.go`
- `web/src/components/table/subscriptions/modals/AddEditSubscriptionModal.jsx`
- `web/src/components/table/subscriptions/SubscriptionsColumnDefs.jsx`
- `web/src/components/topup/SubscriptionPlansCard.jsx`
- `web/src/components/topup/modals/SubscriptionPurchaseModal.jsx`
- 可能需要新增前端模型限额编辑组件。

### 订阅套餐字段

在 `SubscriptionPlan` 上新增模型限额配置字段。

建议字段：

- `ModelAmountLimits string`
- JSON key：`model_amount_limits`
- 数据库类型：`text`

建议 JSON 结构：

```json
{
  "gpt-5.5": 6250000,
  "claude-sonnet-4.5": 3000000,
  "*": 1000000
}
```

说明：

- key 是平台对外模型名。
- value 是原生额度。
- `*` 表示默认模型限额。
- 空 JSON 或空字符串表示不启用模型限额。

### 为什么使用 JSON 字段

套餐模型限额是套餐规则，读取频率高，写入频率低。使用 JSON 字段可以让套餐配置和缓存保持简单。

注意：

- JSON 编解码必须使用 `common.Marshal` / `common.Unmarshal`。
- 不要直接调用 `encoding/json` 的 marshal/unmarshal。

### 后端校验

创建和更新套餐时需要校验：

- JSON 格式合法。
- 每个模型额度不能小于 0。
- 模型名不能为空。
- `*` 允许作为默认规则。
- 建议校验模型是否在系统已知模型中；如果保留高级手填模式，则至少要在前端提示“不可用模型不会展示给用户”。

### 管理端表单

套餐编辑侧边栏需要新增“模型限额”区域。

建议功能：

- 新增一行模型限额。
- 模型下拉选择系统已知模型。
- 支持添加默认规则 `*`。
- 输入额度，展示原生额度换算。
- 支持删除某个模型限额。

提交时：

- 将表单中的模型限额转换为 `model_amount_limits` JSON。
- 与其他 plan 字段一起提交。

编辑时：

- 从 `model_amount_limits` 解析回表单行。

### 管理端列表展示

套餐列表需要展示是否配置了模型限额。

建议展示：

- 未配置：不显示或显示“无模型限额”。
- 已配置：显示“已配置 N 个模型限额”。
- Popover 中展示完整模型限额。

### 用户端展示

用户购买页不能直接展示管理员配置的完整模型限额。

需要在后端或前端做过滤：

- 如果套餐有 `upgrade_group`，先得到购买后的用户组。
- 如果套餐没有 `upgrade_group`，使用用户当前分组。
- 基于该用户组计算可访问的实际使用组。
- 只考虑分组扣费类型为 `subscription_only` 或最终会使用订阅扣费的分组。
- 用户不可用模型不展示。

建议后端在 `/api/subscription/plans` 返回用户可见的模型限额，避免前端重复实现分组和模型过滤。

建议 DTO 增加：

- `visible_model_amount_limits`

管理员接口仍返回完整 `model_amount_limits`。

## 二点五、可访问分组与可见分组拆分

### 需要修改的位置

- `setting/user_usable_group.go`
- `service/group.go`
- `controller/group.go`
- `controller/token.go`
- `middleware/auth.go`
- `web/src/components/table/tokens/modals/EditTokenModal.jsx`
- `web/src/pages/Setting/Ratio/GroupRatioSettings.jsx`

### 当前问题

现有系统里，用户可访问分组和创建 token 时可选分组基本共用同一套配置。

这会导致：

- 如果无限战争用户组允许访问旧余额分组，用户新建 token 时也可能看到旧余额分组。
- 如果为了隐藏旧余额分组而移除访问权限，旧余额 token 会失效。

### 目标逻辑

拆成两套规则：

- 可访问分组：请求鉴权使用，决定 token 分组是否允许继续调用。
- 可见分组：前端创建或编辑 token 时使用，决定下拉框展示哪些分组。

目标场景：

- 用户升级到无限战争后，旧余额 token 继续可用。
- 旧余额 token 按余额分组扣费。
- 用户新建 token 时默认只看到无限战争分组，旧余额分组可以隐藏。

### 默认兼容

为兼容现有配置：

- 如果没有配置可见分组，则默认等于可访问分组。
- 这样旧系统升级后不会立即改变 token 创建行为。

### 建议函数

- `GetUserAccessibleGroups(userGroup string)`
- `GetUserVisibleGroups(userGroup string)`
- `GroupInUserAccessibleGroups(userGroup, groupName string)`
- `GroupInUserVisibleGroups(userGroup, groupName string)`

`middleware/auth.go` 应使用可访问分组。

token 创建/编辑接口和前端下拉框应使用可见分组。

## 三、订阅模型使用量记录

### 需要修改的位置

- `model/subscription.go`
- `model/main.go`

### 需要新增的数据模型

新增用户订阅模型使用量表。

建议模型名：

- `UserSubscriptionModelUsage`

建议字段：

- `Id`
- `UserSubscriptionId`
- `UserId`
- `ModelName`
- `AmountUsed`
- `CreatedAt`
- `UpdatedAt`

建议唯一约束：

- `user_subscription_id + model_name`

说明：

- `AmountUsed` 表示当前订阅周期内该模型已消耗的订阅额度。
- 周期重置时需要清零或删除对应记录。

### 为什么不放进 JSON

模型使用量会在请求扣费链路中高频更新，需要事务锁和原子更新。使用 JSON 字段不利于并发控制，也不利于退款和补扣。

## 四、订阅预扣记录增加模型名

### 需要修改的位置

- `model/subscription.go`

### 需要修改的结构

`SubscriptionPreConsumeRecord` 需要新增：

- `ModelName string`

JSON key：

- `model_name`

数据库类型：

- `varchar(255)`

目的：

- 请求失败退款时知道退回哪个模型的使用量。
- 后结算补扣或返还时知道调整哪个模型。
- 幂等预扣记录可以恢复完整订阅扣费上下文。

## 五、订阅预扣费逻辑

### 需要修改的位置

- `model/subscription.go`
- `service/funding_source.go`
- `service/billing_session.go`

### 当前逻辑

当前 `PreConsumeUserSubscription` 只检查订阅总额度。

### 目标逻辑

订阅预扣时需要同时检查：

- 订阅总额度。
- 当前模型限额。

目标流程：

1. 查找用户 active subscription。
2. 按过期时间排序，优先使用更早过期的订阅。
3. 如果到了重置时间，先重置订阅。
4. 检查总额度是否足够。
5. 读取套餐模型限额。
6. 匹配当前请求模型。
7. 如果该模型有限额，检查模型剩余额度是否足够。
8. 同时写入预扣记录、增加总使用量、增加模型使用量。
9. 任一步失败，事务回滚。

### 模型限额匹配

匹配顺序：

1. 当前请求模型名。
2. 归一化后的模型名，如果系统已有模型名归一化规则适用。
3. 默认 `*`。
4. 未匹配则不做模型单独限额。

注意：

- 模型限额应使用用户请求的模型名或平台模型名。
- 不应使用渠道上游映射后的模型名。

### 返回错误

需要区分：

- 没有有效订阅。
- 订阅总额度不足。
- 当前模型额度不足。

建议在 model 层定义明确错误，避免继续依赖字符串包含判断。

建议错误：

- `ErrNoActiveSubscription`
- `ErrSubscriptionQuotaInsufficient`
- `ErrSubscriptionModelQuotaInsufficient`

`service/billing_session.go` 根据错误类型转换为 API 错误。

## 六、订阅结算、补扣和退款

### 需要修改的位置

- `model/subscription.go`
- `service/funding_source.go`
- `service/billing_session.go`

### 当前逻辑

`PostConsumeUserSubscriptionDelta` 只根据 `user_subscription_id` 调整总使用量。

### 目标逻辑

结算时需要同时调整：

- 订阅总使用量。
- 当前模型使用量。

建议新增函数：

- `PostConsumeUserSubscriptionModelDelta(userSubscriptionId int, modelName string, delta int64) error`

或修改现有函数签名，使其支持模型名。

需要覆盖：

- 请求成功后的实际消耗补扣。
- 请求成功后的预扣返还。
- 请求失败退款。
- streaming 或长请求中的额外预留额度。

### 退款逻辑

`RefundSubscriptionPreConsume` 需要根据预扣记录中的 `model_name` 同时退回模型使用量。

退款必须保持幂等。

### 额外预留额度

`BillingSession.Reserve` 中订阅资金来源额外预留时，也需要同步增加模型使用量。

如果 token 预留失败并回滚资金来源，也要同步回滚模型使用量。

## 七、订阅周期重置

### 需要修改的位置

- `model/subscription.go`
- `service/subscription_reset_task.go`

### 当前逻辑

周期重置只把 `UserSubscription.AmountUsed` 置为 0。

### 目标逻辑

周期重置时需要同时清理该订阅下的模型使用量。

可选实现：

- 将该订阅的所有 `UserSubscriptionModelUsage.AmountUsed` 置为 0。
- 或删除该订阅的所有模型使用量记录。

建议使用置 0：

- 保留模型使用痕迹。
- 管理端后续可以展示历史配置相关模型。

## 八、订阅购买和用户组升级

### 需要修改的位置

- `model/subscription.go`
- `controller/subscription_payment_epay.go`
- `controller/subscription_payment_stripe.go`
- `controller/subscription_payment_creem.go`
- `controller/subscription.go`

### 当前逻辑

购买或管理员绑定订阅后，会从套餐创建用户订阅快照，并按套餐 `upgrade_group` 修改用户组。

### 目标逻辑

这一块整体保持现状。

需要确认：

- 购买时 `AmountTotal` 继续从套餐 `TotalAmount` 复制。
- 模型限额可以不复制到用户订阅实例，而是运行时读取套餐配置。
- 如果未来希望套餐修改不影响已购买用户，则需要把模型限额也快照到 `UserSubscription`。当前阶段建议先读取套餐配置，减少改动。

注意：

- 如果读取套餐配置，则管理员修改套餐模型限额会影响既有 active subscription。
- 这个行为需要在管理端文案提示。

## 九、用户端套餐模型限额过滤

### 需要修改的位置

- `controller/subscription.go`
- `model/ability.go`
- `service/group.go`
- `web/src/components/topup/SubscriptionPlansCard.jsx`
- `web/src/components/topup/modals/SubscriptionPurchaseModal.jsx`

### 目标行为

用户端只展示购买后实际可用的模型限额。

后端接口建议：

- `/api/subscription/plans` 返回每个套餐的完整 plan。
- 同时返回过滤后的 `visible_model_amount_limits`。

过滤规则：

1. 如果套餐有 `upgrade_group`，先得到购买后的用户组。
2. 如果套餐没有 `upgrade_group`，使用用户当前分组。
3. 基于该用户组计算可访问的实际使用组。
4. 只考虑分组扣费类型为 `subscription_only` 的实际使用组；如果保留 `default` 混合扣费，也必须明确该分组购买后会使用订阅，否则不展示为订阅权益。
5. 查询这些实际使用组支持的可用模型。
6. 只保留套餐模型限额中也在可用模型里的项。
7. `*` 默认限额可以展示为“其他可用模型默认限额”。

示例：

- 套餐配置 `gpt-5.5` 限额。
- 用户购买后分组没有 `gpt-5.5`。
- 用户端不展示 `gpt-5.5`。

## 十、模型来源和选择

### 需要修改的位置

- `controller/model.go`
- `controller/user.go`
- `model/ability.go`
- 前端订阅模型限额编辑组件。

### 管理端模型选择

管理员配置模型限额时，建议从系统已知模型中选择。

模型来源可以优先使用：

- `model.GetEnabledModels()`
- 或现有模型列表接口。

允许手填时需要提示：

- 不可用模型不会展示给用户。
- 不可用模型也不会被请求命中。

## 十一、速率限制保持独立

### 需要修改的位置

- 暂不要求修改 `middleware/model-rate-limit.go`

### 说明

速率限制不是模型额度限制。

本次模型限额通过订阅额度实现，不通过请求次数实现。

后续如需让速率限制也使用最终真实分组，需要单独改造，因为当前速率限制使用 token 分组或用户分组，不使用 auto 最终真实分组。

## 十二、日志和可观测性

### 需要修改的位置

- `service/log_info_generate.go`
- `service/text_quota.go`
- `service/task_billing.go`
- `model/subscription.go`

### 建议新增日志字段

订阅扣费日志建议记录：

- 资金来源。
- 订阅套餐 ID。
- 订阅套餐标题。
- 订阅预扣额度。
- 订阅后结算差额。
- 模型名。
- 模型限额。
- 模型使用量。
- 分组扣费类型。

目的：

- 管理员可以判断请求为什么扣订阅或扣余额。
- 用户反馈额度不足时，能查到是总额度不足还是模型额度不足。

## 十三、API 返回和错误提示

### 需要修改的位置

- `service/billing_session.go`
- `model/subscription.go`
- 可能需要扩展 `types/error_code.go`

### 错误语义

至少需要区分：

- 没有有效订阅。
- 订阅总额度不足。
- 当前模型额度不足。
- 分组强制仅订阅，不能回退余额。
- 分组强制仅余额，不能使用订阅。

用户可见提示建议：

- `当前分组需要有效订阅，请先购买或续费套餐`
- `订阅总额度不足`
- `当前模型订阅额度不足`
- `用户余额不足`

## 十四、数据库迁移

### 需要修改的位置

- `model/main.go`
- `model/subscription.go`

### 需要迁移的内容

新增字段：

- `subscription_plans.model_amount_limits`
- `subscription_pre_consume_records.model_name`

新增表：

- `user_subscription_model_usages`

迁移要求：

- 兼容 SQLite、MySQL、PostgreSQL。
- 新字段使用 `TEXT` 或 `varchar`，避免数据库专有类型。
- 新表使用 GORM AutoMigrate。
- 对已有数据提供默认值，不能影响历史订阅。

建议：

- 新表加入 AutoMigrate。
- 新字段可依赖 AutoMigrate 添加。
- 如 SQLite 需要特殊处理，参考现有 `ensureSubscriptionPlanTableSQLite` 模式。

## 十五、测试范围

### 后端测试

建议新增或更新测试覆盖：

- 分组扣费类型解析。
- 分组扣费类型覆盖用户扣费策略。
- 仅订阅分组无订阅时失败。
- 仅订阅分组订阅不足时不回退余额。
- 仅余额分组有订阅时不扣订阅。
- 订阅模型限额预扣成功。
- 订阅总额度不足失败。
- 订阅模型额度不足失败。
- 退款同时退回总额度和模型额度。
- 后结算补扣和返还同时调整模型额度。
- 周期重置清零模型使用量。

### 前端测试

建议验证：

- 管理员可以新增、编辑、删除模型限额。
- 管理员列表能看到模型限额摘要。
- 用户端只展示可用模型限额。
- 不可用模型不展示。
- 原有无模型限额套餐显示不受影响。

## 十六、推荐实施顺序

建议按以下顺序开发，降低风险：

1. 新增分组扣费类型配置，但默认全部保持现状。
2. 在扣费会话中接入分组扣费类型。
3. 新增订阅模型限额字段和模型使用量表。
4. 改造订阅预扣、结算、退款和重置。
5. 增加管理员套餐模型限额配置。
6. 增加用户端可见模型限额过滤展示。
7. 补充测试和错误提示。
8. 最后配置业务分组：无限战争为仅订阅，普通余额组为仅余额。
