# 资源开通证据与异步销毁实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 持久化每轮腾讯 Node Pool 扩容证据，并让计算销毁立即返回、后台确认云资源删除后才释放 Hold。

**Architecture:** 复用 Fabric 现有 operation 的 `resource.providerData` 保存 pool reconcile 证据，不增加表。Fabric 将销毁 operation 置为 `started` 后在 goroutine 内串行清理；Control Plane 保持 `billingStatus=stopping`，由现有 provider reconcile 重试销毁并在 `destroyed` 后幂等释放 Hold。

**Tech Stack:** Go、现有 Fabric operation store、腾讯云 TKE SDK、Control Plane provider reconcile worker。

---

### Task 1: 持久化 Node Pool 每轮证据

**Files:**
- Modify: `services/fabric/cmd/opl-tencent-provisioner/main.go`
- Modify: `services/fabric/cmd/opl-tencent-provisioner/main_test.go`
- Modify: `services/fabric/internal/fabric/types.go`
- Modify: `services/fabric/internal/fabric/tencent_provider.go`
- Modify: `services/fabric/internal/fabric/tencent_provider_test.go`
- Modify: `services/fabric/internal/fabric/pool_allocator.go`
- Modify: `services/fabric/internal/fabric/pool_allocator_test.go`

- [ ] **Step 1: 写失败测试**

断言成功扩容响应包含 `nodePoolId`、`currentReplicas`、`desiredReplicas`、`scaleNodePoolRequestId`、`describeMachinesRequestId` 和 `machineStates`；失败响应保留同一上下文及腾讯错误。断言 allocator 将 attempt、provider request 和上述数据写入 pending create operation 的资源 `providerData`。

- [ ] **Step 2: 验证测试按预期失败**

Run: `cd services/fabric && go test ./cmd/opl-tencent-provisioner ./internal/fabric -run 'Test.*(ReconcileEvidence|PersistsPoolEvidence)' -count=1`

Expected: FAIL，因为 `ComputePoolState` 不携带 `ProviderData`，allocator 也未写 evidence operation。

- [ ] **Step 3: 实现最小证据链**

给 `ComputePoolState` 增加 `ProviderData map[string]string`。`TencentProvider.ReconcileComputePool` 无论成功或 provider 失败都把响应上下文转换为 state；allocator 每轮调用后把 state、attempt 和 error 合并到 pending resource 的 `ProviderData`，沿用原 operation ID 追加 `started` 事实。

Provisioner 只增加已经拿到的 SDK 字段，不发额外腾讯请求：

```go
ProviderData: map[string]string{
    "nodePoolId": nodePoolID,
    "currentReplicas": strconv.FormatInt(current, 10),
    "desiredReplicas": strconv.FormatInt(desired, 10),
    "scaleNodePoolRequestId": scaleRequestID,
    "describeMachinesRequestId": describeRequestID,
    "machineStates": strings.Join(machineStates, ","),
}
```

- [ ] **Step 4: 运行聚焦测试并提交**

Run: `cd services/fabric && go test ./cmd/opl-tencent-provisioner ./internal/fabric -run 'Test.*(ReconcileEvidence|PersistsPoolEvidence)' -count=1`

Expected: PASS。

Commit: `fix(fabric): persist compute pool reconcile evidence`

### Task 2: Fabric 计算销毁异步化且保持幂等

**Files:**
- Modify: `services/fabric/internal/fabric/service.go`
- Modify: `services/fabric/internal/fabric/service_test.go`
- Modify: `services/fabric/internal/http/server_test.go`

- [ ] **Step 1: 写失败测试**

用阻塞 provider 证明 `DestroyComputeAllocation` 在云清理完成前返回 `destroying`；同一资源的重复请求不启动第二个清理；后台成功后返回 `destroyed`，后台失败后保留失败 operation 并允许下一次重试。

- [ ] **Step 2: 验证测试按预期失败**

Run: `cd services/fabric && go test ./internal/fabric ./internal/http -run 'Test.*AsyncDestroy' -count=1`

Expected: FAIL/超时保护触发，因为当前销毁同步阻塞。

- [ ] **Step 3: 实现最小异步状态机**

在 service 内复用相同 pool lock 和现有 operation store：首次请求记录 `started`、将内存资源置为 `destroying`、启动 goroutine；`started` 重放直接返回 `destroying`；`succeeded` 重放返回 `destroyed`；`failed` 重放启动一次新重试。后台仍执行“精确删除 -> ownership released -> pool 收敛确认 -> create cancel finalization”，全部成功后才记录 `succeeded`。

- [ ] **Step 4: 运行聚焦测试并提交**

Run: `cd services/fabric && go test ./internal/fabric ./internal/http -run 'Test.*AsyncDestroy' -count=1`

Expected: PASS。

Commit: `fix(fabric): run compute destroy asynchronously`

### Task 3: Control Plane 只在销毁终态释放 Hold

**Files:**
- Modify: `services/control-plane/internal/controlplane/service.go`
- Modify: `services/control-plane/internal/controlplane/service_test.go`
- Modify: `services/control-plane/internal/server/provider_reconcile_worker.go`
- Modify: `services/control-plane/internal/server/server_test.go`

- [ ] **Step 1: 写失败测试**

断言 Fabric 返回 `destroying` 时 Control Plane 不调用 Ledger release，返回 `destroyed` 时只释放一次；provider reconcile 会处理 `billingStatus=stopping` 的资源，重试 Fabric destroy，并在终态保存 `stopped` 和 release IDs。

- [ ] **Step 2: 验证测试按预期失败**

Run: `cd services/control-plane && go test ./internal/controlplane ./internal/server -run 'Test.*(DestroyingDoesNotRelease|ReconcileCompletesDestroy)' -count=1`

Expected: FAIL，因为当前 service 对任意无错误响应释放 Hold，worker 跳过 `destroying`。

- [ ] **Step 3: 实现终态门禁**

`DestroyComputeAllocation` 仅在 `destroyed/deleted/missing/external_deleted` 时调用 `ReleaseResourceHold`。provider reconcile 对 `billingStatus=stopping && desiredStatus=destroyed` 调用同一幂等 destroy 路径并保存结果；非终态继续保持 `stopping`。

- [ ] **Step 4: 运行聚焦测试并提交**

Run: `cd services/control-plane && go test ./internal/controlplane ./internal/server -run 'Test.*(DestroyingDoesNotRelease|ReconcileCompletesDestroy)' -count=1`

Expected: PASS。

Commit: `fix(control-plane): release compute hold after async destroy`

### Task 4: 全量验证与集成

**Files:**
- Verify only

- [ ] **Step 1: 运行服务全量测试**

Run: `cd services/fabric && go test ./...`

Run: `cd services/control-plane && go test ./...`

Run: `cd services/ledger && go test ./...`

- [ ] **Step 2: 运行 race 与静态检查**

Run: `cd services/fabric && go test -race ./internal/fabric ./internal/http`

Run: `cd services/fabric && go vet ./...`

Run: `cd services/control-plane && go vet ./...`

- [ ] **Step 3: 检查 diff、合并并清退 worktree**

只在验证通过后合并到 `main`；部署后先读取新的 pool evidence，再决定是否执行一次付费 E2E。清理临时诊断产物和本 worktree。
