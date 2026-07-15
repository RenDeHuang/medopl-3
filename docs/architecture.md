# Architecture

## Request Path

```text
Browser Console
  -> Control Plane product API
       -> Sub2API management API: live balance and exact adjustment
       -> Fabric API: CVM, CBS, attachment, runtime, provider facts
       -> Ledger API: receipts and review evidence
```

Sub2API is external and remains the only spendable-balance, API-key, routing,
and request-usage owner. The repository does not mirror those records.

## Service Ownership

`apps/console-ui` owns presentation only. It has no persistence and never calls
Fabric, Ledger, Tencent, Kubernetes, or Sub2API directly.

`services/control-plane` owns Console auth, account mappings, organizations,
Workspaces, monthly entitlements, billing-operation recovery, support mappings,
and product projections. Its public routes express product commands rather than
generic downstream APIs.

`services/fabric` owns compute pools, dedicated CVM allocations, CBS volumes,
attachments, Workspace runtimes, provider operations, and all Tencent/Kubernetes
SDK calls. Provider callbacks may update resource facts but cannot overwrite
Control Plane entitlement state.

`services/ledger` owns EvidenceReceipt, ReviewPolicy, ReconciliationReport,
Artifact, Continuation, retention, audit, and idempotency records. It never
changes Sub2API balance.

`packages/contracts` is machine-readable current truth, not a runtime service.
Speculative route and object entries remain outside the active contracts.

## Persistence

Control Plane, Fabric, and Ledger each own their PostgreSQL schema. Cross-service
writes go through typed HTTP clients; no service writes another service's tables.
Sub2API data remains in Sub2API.

All three services serialize startup migrations with one database-wide PostgreSQL
advisory lock. A migration is journaled in `opl_schema_migrations` by service and
version only after it succeeds. Completed hard cuts, backfills, Ent schema changes,
and embedded SQL are skipped on every later start; a failed migration has no success
record and is retried on the next start.

This deployment starts from a fresh database. There is no compatibility layer,
dual write, historical billing schema, or old-state importer.

## Resource And Billing State

The approved launch path must reserve the exact monthly amount in Sub2API before
Fabric mutates provider resources, then claim every prepaid resource and capture
the reservation before activating the entitlement. Partial or ambiguous provider
results keep the reservation frozen for operator review. Ledger receipt failure
is retryable and never repeats a financial or provider side effect.

The current implementation still prepares Fabric before a direct Sub2API balance
adjustment. That path is an explicit delivery gap, not an approved settlement
protocol; `docs/invariants.md` and the launch freeze contract define its replacement.

All attachment and Workspace runtime operations require an active entitlement.
Compute expiration destroys compute; storage expiration retains data but blocks
use until a new entitlement is purchased.

## Workspace Access Path

The current Workspace data path is:

```text
Browser
  -> workspace.medopl.cn shared CLB / TKE Ingress
  -> Control Plane reverse proxy
  -> Fabric-created per-Workspace ClusterIP Service :3000
  -> Workspace runtime
```

`/w/<workspaceId>/` selects a Workspace from the URL. Root `/api/`, `/ws`, and
other Workspace-host requests select it from the `opl_ws_active` cookie or a
Workspace referrer. The proxy writes `opl_ws_active` as routing context when a
clean Workspace URL is opened; the cookie is not an authentication credential.
It forwards traffic only after Fabric reports the Runtime ready and the
persisted Workspace state becomes `running`.

Fabric runs the Workspace image in `cloud` deployment mode with `password`
authentication. Fabric derives the runtime password and session secret from a
stable per-Workspace credential seed and stores them in a Kubernetes Secret.
The authorized runtime-status command returns the password transiently; Control
Plane never persists it, and Console retains it only in Workspace detail
component memory. Production currently selects the Workspace image through a
mutable `latest` tag, so the exact deployed digest-to-source revision remains
unverified.

This is a real exception to the Control Plane product-command boundary: it
carries Workspace HTML, API, and WebSocket data-plane traffic. The available
evidence does not prove an unauthenticated data disclosure; the inspected
runtime source retains password authentication. The mutable production image
tag prevents extending the source finding to an exact deployed revision.
Control Plane availability is coupled to every Workspace connection, and a
2xx/non-empty-page check can pass on the login page without proving an
authenticated Workspace session.

Keeping the shared proxy avoids per-Workspace CLB rules and is the smallest
topology for the current ten-customer launch. Control Plane selects the Runtime
Service; the Runtime owns password validation, its authenticated session, and
WebSocket access. Routing every Workspace Service directly with native TKE
Ingress removes Control Plane from the data path, but does not replace Runtime
authentication and adds per-Workspace rule quota, creation, deletion, retry,
and orphan reconciliation responsibilities. Do not add those routes until live
CLB limits justify the extra ownership.

The current decision is to retain the single shared entry and explicitly accept
Control Plane availability coupling for the ten-customer stage. A dedicated
Workspace Router remains a later ownership and scaling decision; no router or
security-model change is authorized by this document.

## Production

Production runs Control Plane, Fabric, and Ledger as separate Kubernetes
Deployments. Secrets are Kubernetes Secret references, configuration is a shared
ConfigMap, and the deploy workflow waits for all three rollouts. The legacy paid
production verifier is blocked by `docs/invariants.md` and is not a release gate;
its approved replacement reuses a fixed prepaid Verification Slot.

Control Plane remains one Pod. Its opt-in PostgreSQL capacity gate covers 1,000
accounts/resources, 100 concurrent Console requests, 20 concurrent resource
commands with same-key replay, and a 1,000-resource renewal scan. The current
local baseline passes its five-second request gate with no duplicate charge,
claim, or receipt. Additional replicas remain out of scope unless production
measurements breach the gate after query-level fixes; process-local resource
locks must be replaced with database coordination before any replica increase.

Infrastructure alarms remain in Tencent Cloud Monitor. Business alarms are a
projection of current Control Plane compute and storage rows in Operator Summary;
there is no alert table. `manual_review`, `past_due`, `ledger_receipt_pending`, and
`cleanup_failed` transitions emit stable, redacted log codes for CLS alerting.
