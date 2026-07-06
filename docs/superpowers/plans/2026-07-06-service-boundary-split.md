# Service Boundary Split Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split the current Node Console control-plane monolith into a React + TypeScript Console UI, Go Control Plane API, Go Ledger service backed by PostgreSQL, and Go Fabric service backed by Tencent Cloud SDK and Kubernetes client-go.

**Architecture:** Console UI is a browser application and never owns persistence. Control Plane owns product orchestration and calls Ledger and Fabric over typed APIs. Ledger owns all money, wallet, receipt, audit, reconciliation, and idempotency data in PostgreSQL; Fabric owns cloud resource operations through Tencent Cloud SDK and Kubernetes APIs. There is no long-lived Node compatibility layer: after callers move, old Node API/store/service files are deleted rather than proxied.

**Tech Stack:** React, TypeScript, Go 1.22+, PostgreSQL, OpenAPI/JSON Schema contracts, Tencent Cloud Go SDK, Kubernetes client-go.

---

## Target Boundary

```text
apps/console-ui
  React + TypeScript browser UI.
  Calls only services/control-plane public API.
  Does not import pg, Tencent SDK, Ledger internals, or Fabric internals.

services/control-plane
  Go HTTP API and product orchestration.
  Owns auth, users, organizations, workspaces, support tickets, operation requests, and UI projections.
  Calls services/ledger and services/fabric through typed clients.
  Does not write ledger_entries, wallet_transactions, manual_topups, evidence_receipts, or cloud provider resources directly.

services/ledger
  Go HTTP API backed by PostgreSQL.
  Owns wallets, holds, topups, ledger entries, wallet transactions, evidence receipts, audit events, idempotency keys, and reconciliation reports.
  All write APIs require Idempotency-Key.
  Evidence stores references and redacted URLs only, never raw tokens, secrets, signed URLs, object keys, or provider credentials.

services/fabric
  Go HTTP API or internal Go service backed by optional PostgreSQL.
  Owns resource catalog, compute/storage/attachment/runtime operations, cloud resource mappings, provider request ids, and retryable operations.
  Tencent Cloud SDK and Kubernetes client-go live only here.

packages/contracts
  Machine-readable contracts: service boundaries, OpenAPI specs, route surface, data redaction rules, and deployment contracts.
```

## Migration Rule

Do not build a compatibility proxy from old Node API routes to new Go services. The only temporary bridge allowed is a one-time data migration that reads the old PostgreSQL shape and writes the new service-owned tables. Once a route or write path moves, delete the old Node route/service/store write path in the same task.

## File Structure Map

- Create: `packages/contracts/opl-cloud-service-boundary-contract.json`
  - Durable contract for the target service ownership model.
- Modify: `tests/architecture/package-boundaries.test.js`
  - Enforce service-boundary contract before implementation begins.
- Create: `services/ledger/go.mod`
  - Go module for Ledger service.
- Create: `services/ledger/internal/ledger/types.go`
  - Ledger domain types and idempotency errors.
- Create: `services/ledger/internal/ledger/store.go`
  - Ledger store interface.
- Create: `services/ledger/internal/ledger/postgres_store.go`
  - PostgreSQL-backed Ledger store with append-first writes and unique idempotency constraints.
- Create: `services/ledger/internal/http/server.go`
  - Ledger HTTP API handlers.
- Create: `services/ledger/cmd/ledger/main.go`
  - Ledger service entrypoint.
- Create: `services/fabric/go.mod`
  - Go module for Fabric service.
- Create: `services/fabric/internal/fabric/types.go`
  - Fabric resource operation types.
- Create: `services/fabric/internal/tencent/client.go`
  - Tencent Cloud SDK and Kubernetes client boundary.
- Create: `services/fabric/internal/http/server.go`
  - Fabric HTTP API handlers.
- Create: `services/fabric/cmd/fabric/main.go`
  - Fabric service entrypoint.
- Create: `services/control-plane/go.mod`
  - Go module for Control Plane.
- Create: `services/control-plane/internal/clients/ledger.go`
  - Typed Ledger client.
- Create: `services/control-plane/internal/clients/fabric.go`
  - Typed Fabric client.
- Create: `services/control-plane/internal/http/server.go`
  - Control Plane routes consumed by Console UI.
- Create: `services/control-plane/cmd/control-plane/main.go`
  - Control Plane entrypoint.
- Create: `apps/console-ui/package.json`
  - React + TypeScript UI package.
- Create: `apps/console-ui/src/api/client.ts`
  - Generated or hand-written typed Control Plane client until OpenAPI generation is introduced.
- Move: `packages/console/ui/**` to `apps/console-ui/src/**`
  - UI becomes a pure browser application.
- Delete after replacement: `packages/console/api/**`
  - Node API routes are not retained as a compatibility layer.
- Delete after replacement: `packages/console/src/store.js`
  - Node PostgreSQL state store is removed after Ledger and Control Plane stores are live.

---

### Task 1: Lock The Boundary Contract

**Files:**
- Create: `packages/contracts/opl-cloud-service-boundary-contract.json`
- Modify: `tests/architecture/package-boundaries.test.js`
- Modify: `docs/architecture.md`

- [ ] **Step 1: Write the failing contract test**

Add a test to `tests/architecture/package-boundaries.test.js`:

```js
test("target service boundaries assign persistence, cloud SDKs, and UI responsibilities", async () => {
  const boundary = JSON.parse(await readFile(new URL("../../packages/contracts/opl-cloud-service-boundary-contract.json", import.meta.url), "utf8"));
  assert.equal(boundary.state, "current");
  assert.equal(boundary.services.consoleUi.persistence, "none");
  assert.equal(boundary.services.ledger.persistence, "postgresql");
  assert.equal(boundary.services.fabric.cloudSdkOwner, true);
  assert.equal(boundary.services.controlPlane.calls.ledger, "http");
  assert.equal(boundary.services.controlPlane.calls.fabric, "http");
  assert.equal(boundary.migration.compatibilityLayer, "forbidden");
  assert.ok(boundary.forbiddenRuntimeMarkers.consoleUi.includes("pg"));
  assert.ok(boundary.forbiddenRuntimeMarkers.controlPlane.includes("tencentcloud"));
  assert.ok(boundary.secretPolicy.forbiddenEvidenceMarkers.includes("token"));
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run:

```bash
node --test tests/architecture/package-boundaries.test.js
```

Expected: FAIL because `opl-cloud-service-boundary-contract.json` does not exist.

- [ ] **Step 3: Add the boundary contract**

Create `packages/contracts/opl-cloud-service-boundary-contract.json`:

```json
{
  "schemaVersion": 1,
  "owner": "OPL Platform Architecture",
  "purpose": "Target service ownership contract for splitting Console UI, Control Plane, Ledger, and Fabric.",
  "state": "current",
  "services": {
    "consoleUi": {
      "path": "apps/console-ui",
      "language": "typescript",
      "framework": "react",
      "persistence": "none",
      "calls": { "controlPlane": "http" }
    },
    "controlPlane": {
      "path": "services/control-plane",
      "language": "go",
      "persistence": "postgresql",
      "owns": ["auth", "organizations", "memberships", "workspaces", "supportTickets", "operationRequests"],
      "calls": { "ledger": "http", "fabric": "http" }
    },
    "ledger": {
      "path": "services/ledger",
      "language": "go",
      "persistence": "postgresql",
      "owns": ["wallets", "holds", "manualTopups", "ledgerEntries", "walletTransactions", "evidenceReceipts", "auditEvents", "idempotencyKeys", "reconciliationReports"],
      "writeProtocol": "append_first_with_idempotency"
    },
    "fabric": {
      "path": "services/fabric",
      "language": "go",
      "persistence": "optional_postgresql_for_async_operations",
      "owns": ["resourceCatalog", "computeAllocations", "storageVolumes", "storageAttachments", "workspaceRuntimes", "providerRequests"],
      "cloudSdkOwner": true,
      "cloudSdks": ["tencentcloud-sdk-go", "kubernetes-client-go"]
    }
  },
  "migration": {
    "compatibilityLayer": "forbidden",
    "allowedBridge": "one_time_state_migration_only",
    "retireNodeApi": true,
    "retireNodeStore": true
  },
  "forbiddenRuntimeMarkers": {
    "consoleUi": ["pg", "postgres", "DATABASE_URL", "tencentcloud", "kubernetes", "SecretId", "SecretKey"],
    "controlPlane": ["tencentcloud", "kubernetes/client-go", "ledger_entries INSERT", "manual_topups INSERT", "wallet_transactions INSERT"],
    "ledger": ["tencentcloud", "kubernetes/client-go", "workspace runtime deployment"],
    "fabric": ["manual_topups", "wallet_transactions", "ledger_entries"]
  },
  "secretPolicy": {
    "forbiddenEvidenceMarkers": ["token", "secret", "signedUrl", "presignedUrl", "objectKey", "SecretId", "SecretKey", "kubeconfig"],
    "allowedEvidenceReferences": ["workspaceId", "tokenVersion", "redactedUrl", "providerRequestId", "ledgerEntryId", "operationId"]
  }
}
```

- [ ] **Step 4: Update architecture documentation**

Replace the "Current Implementation Slice" section in `docs/architecture.md` with language that says the repository is moving from one Node control-plane process to the target four-boundary shape, and that old Node API compatibility routes are forbidden.

- [ ] **Step 5: Run the test to verify it passes**

Run:

```bash
node --test tests/architecture/package-boundaries.test.js
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add packages/contracts/opl-cloud-service-boundary-contract.json tests/architecture/package-boundaries.test.js docs/architecture.md
git commit -m "docs: define service boundary split contract"
```

### Task 2: Create Ledger Go Service Skeleton

**Files:**
- Create: `services/ledger/go.mod`
- Create: `services/ledger/internal/ledger/types.go`
- Create: `services/ledger/internal/ledger/store.go`
- Create: `services/ledger/internal/ledger/memory_store.go`
- Create: `services/ledger/internal/ledger/store_test.go`
- Create: `services/ledger/internal/http/server.go`
- Create: `services/ledger/cmd/ledger/main.go`

- [ ] **Step 1: Write Ledger idempotency tests**

Create `services/ledger/internal/ledger/store_test.go`:

```go
package ledger

import (
	"context"
	"errors"
	"testing"
)

func TestManualTopUpReplayReturnsExistingReceipt(t *testing.T) {
	store := NewMemoryStore()
	input := ManualTopUpInput{
		AccountID:      "acct-alpha",
		AmountCents:    20000,
		Currency:       "CNY",
		OperatorUserID: "usr-admin",
		IdempotencyKey: "topup-once",
		Reason:         "operator_credit",
	}
	first, err := store.ManualTopUp(context.Background(), input)
	if err != nil {
		t.Fatalf("first topup failed: %v", err)
	}
	second, err := store.ManualTopUp(context.Background(), input)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if !second.Replayed {
		t.Fatalf("expected replayed result")
	}
	if first.LedgerEntry.ID != second.LedgerEntry.ID {
		t.Fatalf("expected same ledger entry on replay")
	}
	wallet, err := store.Wallet(context.Background(), "acct-alpha")
	if err != nil {
		t.Fatalf("wallet failed: %v", err)
	}
	if wallet.BalanceCents != 20000 {
		t.Fatalf("balance = %d, want 20000", wallet.BalanceCents)
	}
}

func TestManualTopUpSameKeyDifferentPayloadConflicts(t *testing.T) {
	store := NewMemoryStore()
	input := ManualTopUpInput{
		AccountID:      "acct-alpha",
		AmountCents:    20000,
		Currency:       "CNY",
		OperatorUserID: "usr-admin",
		IdempotencyKey: "topup-once",
	}
	if _, err := store.ManualTopUp(context.Background(), input); err != nil {
		t.Fatalf("topup failed: %v", err)
	}
	input.AmountCents = 30000
	_, err := store.ManualTopUp(context.Background(), input)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd services/ledger && go test ./...
```

Expected: FAIL because service files do not exist yet.

- [ ] **Step 3: Add Ledger module and minimal implementation**

Create a Go module and implement `ManualTopUp`, `Wallet`, `ErrIdempotencyConflict`, and memory store with replay detection. Use append-first domain objects: `Wallet`, `LedgerEntry`, `WalletTransaction`, `ManualTopUp`, and `ManualTopUpResult`.

- [ ] **Step 4: Add Ledger HTTP skeleton**

Expose:

```text
GET /healthz
POST /ledger/topups
GET /ledger/accounts/{accountId}/wallet
```

The `POST /ledger/topups` handler must reject missing `Idempotency-Key`.

- [ ] **Step 5: Run Ledger tests**

Run:

```bash
cd services/ledger && go test ./...
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add services/ledger
git commit -m "feat: add ledger service boundary"
```

### Task 3: Add Ledger PostgreSQL Store

**Files:**
- Create: `services/ledger/internal/ledger/postgres_store.go`
- Create: `services/ledger/internal/ledger/postgres_store_test.go`
- Modify: `services/ledger/go.mod`

- [ ] **Step 1: Write PostgreSQL DDL contract test**

Test that schema creation includes:

```sql
CREATE TABLE IF NOT EXISTS wallets
CREATE TABLE IF NOT EXISTS ledger_entries
CREATE TABLE IF NOT EXISTS wallet_transactions
CREATE TABLE IF NOT EXISTS manual_topups
CREATE TABLE IF NOT EXISTS idempotency_keys
CREATE UNIQUE INDEX IF NOT EXISTS manual_topups_idempotency_key_idx
```

- [ ] **Step 2: Implement PostgreSQL store**

Use `database/sql` and `github.com/lib/pq` or `pgx` consistently. `ManualTopUp` must run in one transaction, insert `idempotency_keys`, and return existing result on replay.

- [ ] **Step 3: Run Ledger tests**

Run:

```bash
cd services/ledger && go test ./...
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add services/ledger
git commit -m "feat: add ledger postgres store"
```

### Task 4: Create Fabric Go Service Boundary

**Files:**
- Create: `services/fabric/go.mod`
- Create: `services/fabric/internal/fabric/types.go`
- Create: `services/fabric/internal/fabric/service.go`
- Create: `services/fabric/internal/tencent/client.go`
- Create: `services/fabric/internal/http/server.go`
- Create: `services/fabric/cmd/fabric/main.go`
- Create: `services/fabric/internal/fabric/service_test.go`

- [ ] **Step 1: Write Fabric boundary tests**

Test that Fabric returns catalog data and records provider request ids for compute allocation dry runs without touching Ledger types.

- [ ] **Step 2: Implement Fabric service**

Implement:

```text
GET /healthz
GET /fabric/catalog
POST /fabric/compute-allocations
POST /fabric/compute-allocations/{id}/destroy
POST /fabric/storage-volumes
POST /fabric/storage-attachments
POST /fabric/workspace-runtimes
GET /fabric/workspace-runtimes/{workspaceId}/status
```

- [ ] **Step 3: Move Tencent SDK ownership**

The existing `cmd/opl-tencent-provisioner` logic becomes Fabric-owned code. Tencent Cloud SDK imports must live under `services/fabric` after this task.

- [ ] **Step 4: Run Fabric tests**

Run:

```bash
cd services/fabric && go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/fabric cmd/opl-tencent-provisioner
git commit -m "feat: add fabric service boundary"
```

### Task 5: Create Go Control Plane API

**Files:**
- Create: `services/control-plane/go.mod`
- Create: `services/control-plane/internal/clients/ledger.go`
- Create: `services/control-plane/internal/clients/fabric.go`
- Create: `services/control-plane/internal/domain/workspace.go`
- Create: `services/control-plane/internal/http/server.go`
- Create: `services/control-plane/cmd/control-plane/main.go`
- Create: `services/control-plane/internal/http/server_test.go`

- [ ] **Step 1: Write orchestration test**

Test create Workspace flow:

```text
request reaches Control Plane
Control Plane calls Ledger hold API
Control Plane calls Fabric compute/storage/runtime APIs
Control Plane calls Ledger evidence API
response returns Workspace projection
```

- [ ] **Step 2: Implement typed clients**

Ledger client methods:

```go
ManualTopUp(ctx context.Context, input ManualTopUpInput, idempotencyKey string) (ManualTopUpResult, error)
CreateHold(ctx context.Context, input HoldInput, idempotencyKey string) (HoldResult, error)
RecordEvidence(ctx context.Context, input EvidenceInput, idempotencyKey string) (EvidenceReceipt, error)
```

Fabric client methods:

```go
CreateComputeAllocation(ctx context.Context, input ComputeAllocationInput, idempotencyKey string) (ComputeAllocation, error)
CreateStorageVolume(ctx context.Context, input StorageVolumeInput, idempotencyKey string) (StorageVolume, error)
CreateWorkspaceRuntime(ctx context.Context, input WorkspaceRuntimeInput, idempotencyKey string) (WorkspaceRuntime, error)
```

- [ ] **Step 3: Implement Control Plane HTTP**

Expose UI-facing routes only:

```text
POST /api/auth/login
GET /api/me
GET /api/overview
GET /api/workspaces
POST /api/workspaces
GET /api/billing/summary
GET /api/admin/diagnostics
```

- [ ] **Step 4: Run tests**

Run:

```bash
cd services/control-plane && go test ./...
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add services/control-plane
git commit -m "feat: add control plane service boundary"
```

### Task 6: Move Console UI To React + TypeScript App

**Files:**
- Create: `apps/console-ui/package.json`
- Create: `apps/console-ui/tsconfig.json`
- Create: `apps/console-ui/vite.config.ts`
- Create: `apps/console-ui/src/main.tsx`
- Create: `apps/console-ui/src/api/client.ts`
- Move: `packages/console/ui/**` to `apps/console-ui/src/**`
- Modify: root `package.json`

- [ ] **Step 1: Add UI boundary tests**

Add a test that scans `apps/console-ui/src` and fails on:

```text
pg
DATABASE_URL
tencentcloud
kubernetes
packages/ledger
packages/fabric
```

- [ ] **Step 2: Create TypeScript Vite app**

Add TS config and point root build scripts at `apps/console-ui`.

- [ ] **Step 3: Move UI code**

Move all UI modules from `packages/console/ui` to `apps/console-ui/src`. Convert `.jsx` files touched by route shell to `.tsx` first, then convert remaining files incrementally.

- [ ] **Step 4: Remove ConsolePage god file**

Replace manual path switch with a route registry:

```ts
type RouteComponent = (ctx: ConsoleRouteContext) => JSX.Element;
const routeComponents: Record<string, RouteComponent> = { ... };
```

- [ ] **Step 5: Run UI checks**

Run:

```bash
npm run build
node --test tests/ui/*.test.js
sentrux check .
```

Expected: build passes; UI tests pass; `ConsolePage` fan-out violation removed.

- [ ] **Step 6: Commit**

```bash
git add apps/console-ui packages/console/ui package.json tests/ui tests/architecture
git commit -m "feat: split console ui into typescript app"
```

### Task 7: Delete Node API And Store Without Compatibility Layer

**Files:**
- Delete: `packages/console/api/**`
- Delete: `packages/console/src/store.js`
- Delete: Node-only API tests that are replaced by Go tests.
- Modify: `Dockerfile`
- Modify: `.github/workflows/deploy-tke-production.yml`
- Modify: `deploy/tke/opl-cloud.k8s.json`
- Modify: `package.json`
- Modify: `README.md`
- Modify: `DEV_GUIDE.md`

- [ ] **Step 1: Write no-compatibility test**

Add a contract test that fails if these paths exist:

```text
packages/console/api/server.js
packages/console/api/routes
packages/console/src/store.js
```

The test must also fail if root `package.json` has `"start": "node packages/console/api/server.js"`.

- [ ] **Step 2: Move deployment to Go services**

Update deployment to run:

```text
services/control-plane/cmd/control-plane
services/ledger/cmd/ledger
services/fabric/cmd/fabric
```

Use separate deployments or one multi-process supervisor only for local development. Production should use separate Kubernetes Deployments.

- [ ] **Step 3: Delete old Node API/store**

Remove `packages/console/api/**` and `packages/console/src/store.js`. Do not add proxy routes from old Node routes to Go services.

- [ ] **Step 4: Update docs**

README and DEV_GUIDE must say:

```text
Console UI is React + TypeScript.
Control Plane, Ledger, and Fabric are Go services.
Ledger owns PostgreSQL money/evidence tables.
Fabric owns Tencent Cloud SDK and Kubernetes client usage.
No Node compatibility API remains.
```

- [ ] **Step 5: Run full verification**

Run:

```bash
npm test
npm run build
cd services/ledger && go test ./...
cd ../fabric && go test ./...
cd ../control-plane && go test ./...
sentrux check .
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "refactor: remove node control plane compatibility layer"
```

---

## Self-Review

- Spec coverage: all seven user-approved steps are represented. Step 7 explicitly deletes old Node API/store and forbids compatibility proxies.
- Placeholder scan: no `TBD`, `TODO`, or deferred compatibility layer language remains.
- Type consistency: service names are consistently `console-ui`, `control-plane`, `ledger`, and `fabric`; persistence ownership is consistently PostgreSQL for Ledger, optional PostgreSQL for Fabric async operations, and none for UI.
