# Manual Operations Audit Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OPL Cloud safe for manual commercial operations by making Ledger and Fabric the auditable facts, Control Plane a projection/orchestration layer, and Console a backend-truth display.

**Architecture:** Ledger owns money facts: top-up, hold, release, settlement, wallet transaction, reconciliation, and evidence receipt records. Fabric owns resource operation facts: compute, storage, attachment, runtime, provider request IDs, provider responses, retries, and final status. Control Plane owns auth, workspace lifecycle, support mapping, admin workflow, and read models that can be rebuilt from Ledger/Fabric facts.

**Tech Stack:** Go services in `services/control-plane`, `services/ledger`, and `services/fabric`; React/TypeScript Console in `apps/console-ui`; machine contracts in `packages/contracts`; Node/Vitest rollout and architecture evals in `tests/**`.

---

## Non-Negotiable Phase Gate

Every phase starts with the same scan and ends with the same proof:

- [ ] Run the repo conflict scan before editing:

```bash
rg -n "demo|fallback|compat|compatibility|legacy|TODO|FIXME|audit\": \\[\\]|evidenceLedger\": \\[\\]|runtimeOperations\": \\[\\]|manual top|人工充值|self-service|payment order|usage_log|usage logs|ComputeAllocation|StorageVolume|StorageAttachment|one-person-lab" docs packages services apps tests
```

- [ ] Classify scan hits:
  - Active code or active docs conflicting with current truth must be removed or rewritten in the phase.
  - `docs/history/**` may keep old records only when the file clearly says it is superseded.
  - Tests that enforce old compatibility paths must be deleted or rewritten.

- [ ] Run the phase eval before claiming the phase passed.
- [ ] Run structural checks after the phase:

```bash
npm test -- --run tests/architecture/package-boundaries.test.ts
sentrux check .
git diff --check
```

If the same blocker fails three times, pause implementation and report the blocker with the failing command and current diff.

## Product Truth

The resource chain is:

```text
Account/User
 -> Wallet/Ledger
 -> ComputeAllocation
 -> StorageVolume
 -> StorageAttachment
 -> Workspace URL
 -> one-person-lab-app runtime
```

`one-person-lab-app` is the default runtime template image. It is not a billable resource, storage owner, audit source, or lifecycle owner. Compute, storage, attachment, workspace URL, Ledger rows, and Fabric operation rows are the commercial truth.

## Audit Model

Do not copy Sub2API's request `usage_logs` model directly. Sub2API audits API requests and payment orders; OPL Cloud audits cloud resource operations and money movement.

OPL should keep these fact surfaces:

- Ledger facts, already present or expected under `services/ledger`: `ledger_entries`, `wallet_transactions`, `manual_topups`, `holds`, `hold_releases`, `resource_settlements`, `evidence_receipts`, `reconciliation_reports`, and `idempotency_keys`.
- Fabric facts, required under `services/fabric`: `fabric_operations` or the existing Fabric store equivalent, with operation ID, actor or caller service, resource kind, resource ID, provider, provider request ID, idempotency key, request hash, redacted provider payload, status, error code, retryable flag, startedAt, finishedAt.
- Control Plane facts, required under `services/control-plane`: admin/operator audit events and support ticket external mappings. Control Plane read model is not the audit fact; it is a rebuildable projection.

Archival rule: fact rows are append-only for the retention window. Archive means export or cold storage, not rewriting the facts into a mutable summary. Projections can be compacted because they are rebuildable.

---

## Phase 1: API Guard

**Purpose:** Backend route behavior must match the route contract: authenticated routes require a session, mutating routes require CSRF, admin routes require admin.

**Files:**
- Modify: `services/control-plane/internal/server/server.go`
- Modify: `services/control-plane/internal/server/runtime.go`
- Test: `services/control-plane/internal/server/server_test.go`
- Eval: `tests/architecture/package-boundaries.test.ts`

**Current Risk:** Console sends CSRF headers, route contracts describe auth/admin requirements, but backend route registration can still expose protected mutations unless every handler enforces the same guard.

**Implementation:**
- [ ] Add failing tests for unauthenticated write routes returning `401`.
- [ ] Add failing tests for authenticated writes without `x-opl-csrf` returning `403`.
- [ ] Add failing tests for non-admin users calling admin routes returning `403`.
- [ ] Add one guard helper in `server.go` and wrap protected route registration there.
- [ ] Keep only health/login/operator token bootstrap public.

**Eval:**

```bash
cd services/control-plane
go test ./internal/server -run 'Test.*Auth|Test.*CSRF|Test.*Admin|TestActiveConsoleAPIRoutesReachControlPlane'
```

## Phase 2: Login Rate Limit

**Purpose:** Manual operations still need brute-force protection for admin and owner login.

**Files:**
- Modify: `services/control-plane/internal/server/auth.go`
- Modify: `services/control-plane/internal/server/runtime.go`
- Test: `services/control-plane/internal/server/server_test.go`

**Current Risk:** Password hash/session support exists, but login attempts need a shared path that blocks repeated failures.

**Implementation:**
- [ ] Add failing test: repeated failed login attempts for the same email/IP return `429`.
- [ ] Add a minimal in-process limiter keyed by IP and email. No new dependency.
- [ ] Reset the limiter on successful login.
- [ ] Keep Redis out until multi-replica production requires it.

**Eval:**

```bash
cd services/control-plane
go test ./internal/server -run 'Test.*Login.*Rate|Test.*Auth'
```

## Phase 3: High-Risk Admin Confirmation

**Purpose:** Admin money/resource/destructive actions must carry explicit operator intent, not accidental clicks.

**Files:**
- Modify: `services/control-plane/internal/server/server.go`
- Modify: `apps/console-ui/src/api/console-api.ts`
- Modify: `apps/console-ui/src/pages/admin/AdminOverviewPage.tsx`
- Modify: `apps/console-ui/src/pages/resources/ResourceProvisioningPages.tsx`
- Test: `services/control-plane/internal/server/server_test.go`
- Test: `tests/ui/commercial-console-surface.test.ts`

**Current Risk:** Some actions have boolean confirms, but the backend must reject missing confirmation for top-up, settlement, user deletion, compute destroy, storage destroy, and access cleanup.

**Implementation:**
- [ ] Add failing backend tests for missing confirmation on each high-risk route.
- [ ] Enforce existing confirmation fields at the API boundary.
- [ ] Keep UI layout stable; only send required confirmation fields and show backend errors.

**Eval:**

```bash
cd services/control-plane
go test ./internal/server -run 'Test.*Confirm|Test.*Delete|Test.*TopUp|Test.*Destroy'
npm test -- --run tests/ui/commercial-console-surface.test.ts
```

## Phase 4: Ledger Audit Closure

**Purpose:** Money movement must be explainable from Ledger rows without relying on Console projection.

**Files:**
- Modify: `services/ledger/internal/ledger/store.go`
- Modify: `services/ledger/internal/ledger/types.go`
- Modify: `services/ledger/internal/ledger/memory_store.go`
- Modify: `services/ledger/internal/ledger/postgres_store.go`
- Modify: `services/ledger/internal/http/server.go`
- Test: `services/ledger/internal/ledger/store_test.go`
- Test: `services/ledger/internal/ledger/postgres_store_test.go`
- Test: `services/ledger/internal/http/server_test.go`

**Current Risk:** Ledger already has top-up, hold, release, settlement, wallet transaction, evidence, reconciliation, and idempotency structures. The phase passes only if every money mutation proves its ledger entry, wallet transaction, idempotency replay, and conflict behavior.

**Implementation:**
- [ ] Scan Ledger code for any mutation that writes wallet state without `ledger_entries` and `wallet_transactions`.
- [ ] Remove any mutable summary path that can change balances without a fact row.
- [ ] Add or tighten tests for manual top-up, create hold, release hold, settlement, evidence, and reconciliation replay/conflict.
- [ ] Ensure `hold_releases` releases frozen funds without debiting balance.
- [ ] Ensure `resource_settlements` debits balance and reduces frozen funds.

**Eval:**

```bash
cd services/ledger
go test ./internal/ledger ./internal/http
```

## Phase 5: Fabric Durable Operations

**Purpose:** Compute/storage/attachment/runtime operations need their own audit trail because provider operations are the product's real operational risk.

**Files:**
- Modify: `services/fabric/internal/fabric/types.go`
- Modify: `services/fabric/internal/fabric/service.go`
- Modify: `services/fabric/internal/http/server.go`
- Create or modify: `services/fabric/internal/fabric/operation_store.go`
- Test: `services/fabric/internal/fabric/service_test.go`
- Test: `services/fabric/internal/http/server_test.go`

**Current Risk:** Fabric resource maps track current state, but operation facts are not durable enough for "who created/destroyed which compute/storage and what did Tencent return".

**Implementation:**
- [ ] Add failing test: create compute records a `fabric_operation` with resource kind, resource ID, provider request ID, idempotency key, status, and timestamps.
- [ ] Add failing tests for create/destroy compute, create/destroy storage, attach/detach, create runtime.
- [ ] Add a minimal operation store and use it from the shared Fabric service path, not per-handler copies.
- [ ] Store redacted provider evidence only; no secrets.
- [ ] Expose a read-only admin operation list through Control Plane projection later, not directly from Console to Fabric.

**Eval:**

```bash
cd services/fabric
go test ./internal/fabric ./internal/http
```

## Phase 6: Control Plane Projection Boundary

**Purpose:** Control Plane read model is a cached view for UI, not the source of audit truth.

**Files:**
- Modify: `services/control-plane/internal/server/runtime.go`
- Modify: `services/control-plane/internal/server/read_model_store.go`
- Modify: `services/control-plane/internal/controlplane/service.go`
- Test: `services/control-plane/internal/server/server_test.go`
- Test: `services/control-plane/internal/controlplane/service_test.go`

**Current Risk:** `/api/state` and `/api/management/state` still include empty `audit`, `evidenceLedger`, and `runtimeOperations` values in some paths, which hides whether facts exist.

**Implementation:**
- [ ] Add failing test: management state is built from persisted projection rows that include Ledger and Fabric facts.
- [ ] Remove empty-array default facts when the backend does not have data.
- [ ] Keep projection rows explicitly labeled as projection fields.
- [ ] Make mutation success refresh or write the projection from returned Ledger/Fabric facts.

**Eval:**

```bash
cd services/control-plane
go test ./internal/server ./internal/controlplane
```

## Phase 7: Manual Top-Up Formalization

**Purpose:** Current product mode is manual top-up only. Do not pretend self-service payment exists.

**Files:**
- Modify: `packages/contracts/opl-cloud-billing-ledger-contract.json`
- Modify: `packages/contracts/opl-cloud-route-api-contract.json`
- Modify: `apps/console-ui/src/pages/admin/AdminOverviewPage.tsx`
- Modify: `apps/console-ui/src/pages/billing/BillingPage.tsx`
- Test: `tests/architecture/package-boundaries.test.ts`
- Test: `tests/ui/commercial-console-surface.test.ts`

**Current Risk:** Sub2API has payment orders because it supports payment channels. OPL Cloud manual operations should not add payment-order scaffolding until self-service payment is actually in scope.

**Implementation:**
- [ ] Scan for "self-service payment", "payment order", and "pay now" active UI/contracts.
- [ ] Delete or rewrite active self-service payment claims.
- [ ] Keep manual top-up tied to operator, account, reason, idempotency key, ledger entry, and wallet transaction.
- [ ] UI displays "人工充值" and audit facts, not "支付订单".

**Eval:**

```bash
npm test -- --run tests/architecture/package-boundaries.test.ts tests/ui/commercial-console-surface.test.ts
```

## Phase 8: Reconciliation And Blocking

**Purpose:** Reconciliation should protect new workspaces when Ledger/provider facts disagree.

**Files:**
- Modify: `services/ledger/internal/ledger/postgres_store.go`
- Modify: `services/control-plane/internal/controlplane/service.go`
- Modify: `services/control-plane/internal/server/server.go`
- Test: `services/ledger/internal/ledger/store_test.go`
- Test: `services/control-plane/internal/controlplane/service_test.go`
- Test: `services/control-plane/internal/server/server_test.go`

**Current Risk:** Reconciliation data exists, but workspace/resource creation must read the guard and block when status is not `ok`.

**Implementation:**
- [ ] Add failing test: reconciliation status `blocked` prevents creating new workspace runtime.
- [ ] Add failing test: reconciliation `ok` allows creation.
- [ ] Persist reconciliation reports as Ledger facts.
- [ ] Surface the block reason in admin state.

**Eval:**

```bash
cd services/ledger && go test ./internal/ledger -run 'Test.*Reconciliation'
cd services/control-plane && go test ./internal/controlplane ./internal/server -run 'Test.*Reconciliation|Test.*Workspace'
```

## Phase 9: External Support Mapping API

**Purpose:** Do not build a helpdesk. Store enough mapping to connect external tickets to OPL accounts/resources.

**Files:**
- Modify: `services/control-plane/internal/server/runtime.go`
- Modify: `services/control-plane/internal/server/read_model_store.go`
- Modify: `services/control-plane/internal/server/server.go`
- Modify: `packages/contracts/opl-cloud-route-api-contract.json`
- Test: `services/control-plane/internal/server/server_test.go`
- Test: `tests/ui/commercial-console-surface.test.ts`

**Current Risk:** Support UI must not imply a full internal ticket system. It should show external ticket mappings and resource context.

**Implementation:**
- [ ] Keep required fields: `externalSystem`, `externalTicketId`, `externalUrl`, `accountId`, `userId`, `workspaceId`, `resourceIds`, `operationId`, `status`.
- [ ] Reject mappings without external ticket ID or account ID.
- [ ] Admin can list mappings; owner can see mappings scoped to their account.
- [ ] No iframe/provider-specific integration until the external system is selected.

**Eval:**

```bash
cd services/control-plane
go test ./internal/server -run 'Test.*Support'
npm test -- --run tests/ui/commercial-console-surface.test.ts
```

## Phase 10: Rollout E2E

**Purpose:** Prove the manual operation chain works in a real browser and, when configured, against real staging cloud resources.

**Files:**
- Modify only if eval exposes a real bug:
  - `tests/production/**`
  - `scripts/**`
  - service/UI files touched by the failed behavior

**Current Risk:** Source-level tests can prove routes and text, but not that mutation, refresh, projection, Ledger facts, Fabric facts, and Workspace URL behavior line up.

**Implementation:**
- [ ] Run local unit and UI tests.
- [ ] Run production verifier in the configured environment.
- [ ] Capture screenshots for login, admin billing/audit, resource provisioning, workspace URL, and cleanup state.
- [ ] Treat any mismatch between UI state and backend facts as a bug in projection or API wiring, not a UI copy issue.

**Eval:**

```bash
npm test
npm run build
npm run staging:readiness
OPL_CONFIRM_REAL_CLOUD_E2E=1 npm run staging:e2e
OPL_CONSOLE_ORIGIN=https://<console-domain> npm run verify:production
```

## Done Criteria

- [ ] No active compatibility layer or old product narrative remains outside clearly superseded `docs/history/**`.
- [ ] Ledger can explain every balance/frozen/spent change from fact rows.
- [ ] Fabric can explain every compute/storage/attachment/runtime operation from operation rows.
- [ ] Control Plane can rebuild Console state from backend facts/projections.
- [ ] Console does not invent status, money, audit, or provider truth.
- [ ] Auth/CSRF/admin guard protects all active Control Plane API routes.
- [ ] Manual top-up is formal and auditable; self-service payment is not claimed.
- [ ] External support mapping is ready for a future helpdesk integration without pretending the helpdesk exists.
- [ ] Rollout E2E screenshots and command outputs prove the chain.
