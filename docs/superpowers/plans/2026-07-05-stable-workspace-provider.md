# Stable Workspace Provider Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Workspace the stable commercial object while fixing Tencent TKE machine-to-node identity so production compute allocation can complete reliably.

**Architecture:** Keep the current single OPL Console control-plane and package boundaries. Fabric owns Tencent/TKE/Kubernetes resource lifecycle, Console owns Workspace/resource state, and contracts describe the commercial truth. Workspace becomes the stable URL/storage/runtime pointer; ComputeAllocation and Attachment become replaceable runtime resources.

**Tech Stack:** Node.js test runner, React console UI, Go Tencent provisioner, Tencent TKE/CVM SDK, GitHub Actions production workflows.

---

### Task 1: Fix Tencent TKE Machine Identity

**Files:**
- Modify: `cmd/opl-tencent-provisioner/main.go`
- Test: `cmd/opl-tencent-provisioner/main_test.go`

- [ ] Write a failing Go test showing native TKE nodes expose a reliable machine name / provider instance label even when CVM private-IP lookup returns no result.
- [ ] Run `go test ./...` in `cmd/opl-tencent-provisioner`; expect failure at the new identity test.
- [ ] Implement minimal identity resolution that accepts TKE machine identity as the Tencent node deletion handle and records CVM identity only when CVM lookup succeeds.
- [ ] Run `go test ./...`; expect pass.

### Task 2: Harden Console Completion Semantics

**Files:**
- Modify: `packages/console/src/services/resource-provisioning-service.js`
- Test: `tests/domain/resource-provisioning.test.js`

- [ ] Write a failing test that a compute allocation can complete with `nodeName + machineName + privateIp` even if `instanceId` is absent for TKE native nodes, while still failing when node identity is absent.
- [ ] Run targeted test; expect failure.
- [ ] Implement minimal completion predicate and persisted fields.
- [ ] Run targeted test; expect pass.

### Task 3: Make Workspace the Stable URL Subject

**Files:**
- Modify: `packages/console/src/services/workspace-lifecycle-service.js`
- Modify: `packages/console/src/services/resource-provisioning-service.js`
- Modify: `packages/console/api/server.js`
- Test: `tests/e2e/local-business-chain.test.js`
- Test: `tests/domain/workspace-url-route.test.js`

- [ ] Write failing tests: destroying compute leaves Workspace URL active but suspended; rebuilding compute and reattaching the same storage updates the same Workspace record and URL.
- [ ] Run targeted tests; expect failure because current code creates a second Workspace entry.
- [ ] Implement stable Workspace fields: `currentComputeAllocationId`, `currentAttachmentId`, `runtimeStatus`, and keep `url/access.token` stable.
- [ ] Update gateway unavailable handling to show suspended/runtime unavailable instead of treating URL as gone.
- [ ] Run targeted tests; expect pass.

### Task 4: Clear Old Narrative and Contracts

**Files:**
- Modify: `packages/contracts/opl-cloud-business-object-contract.json`
- Modify: `packages/contracts/opl-cloud-route-api-contract.json`
- Modify: `packages/console/ui/routes/opl-routes.js`
- Test: `tests/contracts/*.test.js`
- Test: `tests/ui/*.test.js`

- [ ] Write/adjust contract tests so Workspace is the stable URL/storage/runtime pointer and attachment is a replaceable runtime binding.
- [ ] Run contract/UI tests; expect failure against old attachment-derived Workspace wording.
- [ ] Update active contracts and runtime route registry; remove incompatible wording rather than keeping aliases.
- [ ] Run contract/UI tests; expect pass.

### Task 5: Production Cleanup and Verification

**Files:**
- No app code unless diagnostics prove another root cause.

- [ ] Run read-only diagnostics to confirm stale resources and exact labels for `compute-lh251t`.
- [ ] Before any destructive `kubectl delete`, report the exact objects to delete and wait for explicit confirmation if not already covered by the user's request.
- [ ] Release/deploy fixed control-plane.
- [ ] Run production verifier once.
- [ ] If verifier fails, stop after collecting structured state/provider error; do not blindly retry.
