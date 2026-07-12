# 资源计费生命周期设计

## 目标结果

计算和存储的开通流程在 Console、Control Plane、Fabric 和 Ledger 之间形成可恢复的
商业闭环。开通成功时只扣首小时费用，并保留该资源恰好 7 天费用的冻结金额。开通
失败不扣费。销毁会停止后续计费，并在确认云资源已经删除后，只释放该资源自身 Hold
的剩余金额。

## 现有基础

提交 `3f7923e` 已经提供了可复用的补偿能力：

- Control Plane 会为终态资源释放 Hold；
- 单个资源结算失败不会中断其他资源的周期结算；
- Workspace Runtime 创建后，如果 Ledger receipt 写入失败，会执行补偿清理；
- Provider 对账不会覆盖商业投影字段。

这些改动继续保留，但还没有完成整个生命周期闭环。Ledger 目前仍然针对账户级冻结
总额结算，释放时仍信任调用方提交的金额，Fabric 也无法恢复所有异步操作或 Provider
部分执行成功的情况。

已经删除的 `gateway/g0-g4` 分支及其旧计费代码不是本次工作的依赖，也不作为合并
来源。

## 方案比较

### 采用方案：Ledger 按 Hold 管理状态，复用现有对账 Worker

Ledger 管理每个 Hold 的剩余金额，并根据该 Hold 校验开通、结算和释放操作。Control
Plane 继续承担 Saga 协调职责，现有 Fabric operation 持久化和对账 Worker 用于恢复
未完成的操作。

这是能够从资金权威源头修复问题、同时复用仓库现有服务边界的最小方案。

### 不采用：保留账户级冻结总额，只加强 Control Plane 记账

Control Plane 可以估算 Hold 剩余金额，并在释放时提交这个估算值。但它无法阻止其他
资源消费该 Hold，并发 Ledger 请求仍可能对同一个钱包产生超额消费。该方案会错误地
把投影层当成资金权威，无法闭合 Ledger 边界。

### 不采用：增加工作流引擎或分布式事务协调器

工作流引擎能够描述所有崩溃点，但会新增基础设施和第二套 operation 模型，而 Fabric
和 Control Plane 已经持久化了幂等操作。持久化 claim、严格的 Ledger 状态转换以及
对账机制已经能够覆盖所需失败场景，代码和运行组件也更少。

## 资金不变量

1. 每个可计费的计算资源或存储卷只能有一个归属 Hold。归属由 `hold_id`、
   `account_id`、`workspace_id`、`resource_type` 和 `resource_id` 共同确定。
2. 开通时先预留 7 天保证金额加首小时费用。这是资金授权，不是扣款。
3. Provider 验证成功后，以原子操作消费首小时部分并激活 Hold。操作完成后，冻结
   余额恰好为 7 天费用。
4. 开通失败绝不产生扣款。确认云端不存在残留后，释放全部剩余授权金额。
5. 周期结算先消费账户可用余额；不足部分只能消费当前资源自身 Hold 的剩余金额。
6. 如果可用余额加当前资源 Hold 仍不足以支付下一小时，结算不进行任何部分资金
   变更，并触发该资源的暂停或销毁流程。禁止消费其他资源的 Hold。
7. 销毁请求一旦被接受，计费立即进入不可计费的 `stopping` 状态。Provider 清理
   会持续重试，直到确认资源不存在。
8. 确认销毁后，计费变为 `stopped`，并释放归属 Hold 的剩余金额。释放操作绝不
   扣减钱包余额。
9. 每个幂等键对应的 Hold 消费或释放只能执行一次。累计消费金额加累计释放金额
   不能超过最初授权金额。
10. 钱包汇总和 Hold 状态必须在同一个 Ledger 事务内、持有钱包行锁时变更。并发
    开通、结算和释放不能造成更新丢失。

对于金额为 `charge` 的周期结算，Ledger 执行：

```text
available_part = min(charge, wallet.available)
hold_part      = charge - available_part

require hold_part <= resource_hold.remaining
wallet.balance          -= charge
wallet.frozen           -= hold_part
resource_hold.remaining -= hold_part
resource_hold.consumed  += hold_part
wallet.available = wallet.balance - wallet.frozen
```

释放 Hold 时执行：

```text
release = resource_hold.remaining
wallet.frozen -= release
resource_hold.remaining = 0
resource_hold.released += release
wallet.balance 保持不变
```

## Ledger 设计

现有 Hold 成为其生命周期的资金权威。它记录最初授权金额、剩余金额、已消费金额和
已释放金额，并记录 `reserved`、`active`、`exhausted` 或 `released` 等状态。
现有账户钱包汇总字段继续作为聚合视图。

Ledger 提供边界明确的幂等状态转换，不再接受 Control Plane 计算的释放金额：

- 为一个资源预留开通资金；
- Provider 验证成功后激活 Hold，并消费首小时费用；
- 使用可用余额和指定资源的 Hold 结算该资源；
- 为开通失败或已经销毁的资源释放全部剩余金额。

每个请求都必须包含 Hold 身份和资源身份。对于身份缺失或不匹配、Hold 已进入终态、
金额非正数、币种不一致以及幂等键冲突的请求，Ledger 一律拒绝。释放金额由 Ledger
内部计算，调用方提交的 Hold 金额不再具有资金权威。

PostgreSQL 在读取余额之前锁定钱包行，在变更 Hold 之前锁定对应 Hold。内存 Store
实现相同的状态转换，避免测试和本地行为与生产行为不一致。

## 开通流程

Control Plane 首先使用调用方幂等键和规范化请求哈希持久化或 claim 一条 execution
request。相同哈希的重试会恢复操作或返回已经保存的结果；使用相同幂等键提交不同
请求时返回幂等冲突。资源 ID 从已 claim 的请求稳定生成，不随重试变化。

```text
claim 请求
-> Ledger 预留 7 天费用加首小时费用
-> 保存带 hold_id 的 provisioning 投影
-> Fabric claim 并启动 Provider operation
-> 验证真实 Provider 身份和就绪状态
-> Ledger 消费首小时费用并激活 Hold
-> 保存 running/available 投影，计费变为 active
```

Provider 成功必须包含可用的 Provider 证据，不能只依赖 `OK` 响应：

- 计算资源必须存在预期 Machine，并具有非空的 Machine 或 Node 身份；
- 存储资源必须存在预期 PVC 身份且状态可用；
- 返回的身份必须属于请求中的资源和 Workspace。

如果 Control Plane 在任意步骤之后崩溃，对账会根据持久化 claim 和下游幂等状态转换
恢复操作。尤其是 Provider 已成功但 Ledger 尚未激活的场景，只会补做一次激活，不会
创建第二个云资源。

## 开通失败

Fabric 将超时、没有 Machine、没有 Node 身份、没有 PVC、Provider 证据格式错误以及
Provider 部分变更成功都视为开通失败。在报告可补偿的终态失败之前，Fabric 会发现并
删除该 operation 创建的所有资源，并确认这些资源已经不存在。

```text
Provider 失败
-> 查找 operation 残留
-> 重试清理，直到确认资源不存在
-> 将 Fabric operation 标记为 failed 且 cleaned
-> Ledger 释放全部剩余授权金额
-> Control Plane 保存 failed/stopped 终态投影
```

如果清理无法确认资源不存在，operation 保持可对账状态，Hold 继续冻结。此时不扣款，
但在可能仍存在付费云资源时也不释放 Hold，从而同时避免客户被错误收费和平台承担
无资金保障的 Provider 成本。

## 销毁流程

Control Plane claim 销毁请求，并在调用 Fabric 前把计费状态从 `active` 改为
`stopping`。周期结算输入会排除 `stopping`、`stopped`、失败和已经销毁的资源，
因此不会再产生后续小时账期。

Fabric 删除必须向上返回 Provider 和 `kubectl` 错误。成功销毁要求确认计算资源的
Machine/Node 或存储资源的 PVC 已经不存在，不能只依据删除命令成功退出。对账会使用
相同 operation key 重试未完成的删除。

只有确认资源不存在后，Control Plane 才请求 Ledger 释放归属 Hold 的剩余金额，并
持久化 `billing_status=stopped`。如果 Ledger 已释放但投影持久化失败，重试会返回
同一条释放结果，由对账修复投影。

销毁请求被接受后，如果 Provider 清理延迟或失败，产生的 Provider 成本由平台承担；
客户不会被继续计费。

## Control Plane 与 Console

Control Plane 投影用于展示商业状态，但不成为资金权威。投影至少保留 operation ID、
Hold ID、Provider 状态、`billing_status`、最近 settlement ID、release ID 和可安全
展示给客户的失败详情。

复用现有 Console 资源轮询和结果展示。界面显示后端提供的 `provisioning`、
`running` 或 `available`、`destroying`、`failed` 和 `destroyed` 状态，并
显示 `pending`、`active`、`stopping` 或 `stopped` 计费状态。Console 不在
本地计算 Hold 剩余金额，也不根据 HTTP 已接受响应推断开通成功。

Workspace 投影写入失败时，根据持久化 execution request、Fabric operation、Ledger
receipt 和资源投影执行修复。现有 receipt 失败后的 Runtime 清理继续保留。

## 对账与 Hold 耗尽

现有周期结算 Worker 在单个账户或资源失败后继续处理其他项目。资源资金不足时，
Ledger 为 Control Plane 返回持久化失败结果；Control Plane 随后将计费移出 `active`
状态，并使用稳定幂等键请求暂停或销毁。重试不能重复扣除失败账期。

对账覆盖以下未完成状态：

- 请求已 claim，但 Ledger 尚未预留；
- Ledger 已预留，但 Fabric operation 尚未创建；
- Fabric operation 已启动，但尚未进入终态；
- Provider 资源已验证，但 Ledger 尚未激活；
- Provider 失败且可能存在残留；
- 已请求销毁，但尚未确认云资源删除；
- Ledger 已释放，但 Control Plane 投影仍然过期。

Worker 聚合错误，并继续处理其他资源。

## 数据迁移

Ledger 迁移为 Hold 增加生命周期字段，以及资源归属和幂等释放所需的约束与索引。
对于现有 active Hold，只有能够明确归属的历史消费才会从原始金额中扣除并回填剩余
金额。对于存在歧义的历史 Hold，不会自动释放，而是标记为需要对账，避免凭空增加
钱包资金。

本方案不增加依赖、队列、工作流引擎或通用抽象。

## 验证

实现需要在每个边界留下范围明确、可运行的检查：

- Ledger 单元测试和 PostgreSQL 测试覆盖激活、可用余额优先结算、自身 Hold 兜底、
  释放计算、身份拒绝、幂等重放，以及并发结算和释放的串行化；
- Fabric 测试覆盖崩溃恢复、无 Machine/Node/PVC、部分创建后的清理、严格删除错误
  和资源不存在确认；
- Control Plane 失败注入测试覆盖开通和销毁的每个边界、稳定请求身份、投影修复、
  不可计费状态过滤和 Hold 耗尽；
- Console 合同测试证明界面展示后端生命周期状态，不执行本地余额计算；
- 跨服务测试覆盖成功开通、开通失败、小时结算、Hold 耗尽、销毁和幂等重放；
- 完整运行现有 Node、Control Plane、Fabric 和 Ledger 测试套件。

## 完成标准

只有证明以下条件全部成立，才能认为生命周期已经闭环：

- 成功开通只留下一个真实资源、一笔首小时扣款和一个 active 的 7 天 Hold；
- 开通失败不留下云端残留、不产生扣款，也不保留冻结金额；
- 小时结算先使用可用余额，不足时只使用资源自身 Hold；
- Hold 耗尽会停止资源，且不会触碰其他 Hold；
- 销毁会阻止后续账期，在确认云端删除后只释放归属 Hold 的剩余金额，不扣余额；
- 进程崩溃和重复请求最终收敛到同一资金结果和 Provider 结果；
- Console 对所有进行中和终态状态都展示经过对账的后端事实。
