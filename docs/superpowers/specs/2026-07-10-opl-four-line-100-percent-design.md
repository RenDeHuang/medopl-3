# OPL Four-Line 100 Percent Program Design

## Status

Draft for implementation review. The overall direction was approved on 2026-07-10:

- complete the full OPL Workspace, Console, Fabric, and Ledger vision;
- treat OPL Gateway as a required cross-cutting product track;
- establish one shared contract spine before parallel implementation;
- use short-lived worktrees and repeated integration trains, not four long-lived branches.

## Decision

The program will not attempt one four-branch merge containing every remaining capability.

It will use this delivery shape:

```text
shared contract spine
        |
        v
repeated integration train
  Workspace  Console  Fabric  Ledger
        \       |       |       /
         ordered merge and cross-service E2E
                    |
                    v
          staging, canary, cleanup
```

Each train starts from a clean, synchronized `main`, creates fresh short-lived worktrees, lands a bounded vertical capability, verifies the whole system, pushes `main`, and deletes the worktrees and branches.

"100 percent" means every capability and production gate in this document has objective evidence. It is not a subjective code-completeness percentage.

## Goal

Deliver One Person Lab Cloud as a system where one researcher can:

1. Start or resume the same project and task from the local App or a cloud Workspace.
2. Request resources without knowing cloud-provider or Kubernetes details.
3. Pass organization, policy, quota, budget, and human approval gates.
4. Execute through a versioned environment, connector, model, or agent.
5. Recover files and task state after disconnects, failures, upgrades, or infrastructure replacement.
6. Inspect artifacts, reviews, usage, cost, and an immutable human-readable receipt.
7. Continue work from the receipt on another allowed execution surface.

## Repository Boundary

This repository owns the OPL Cloud control-plane slice described in `docs/project.md`:

- Console UI and Control Plane;
- Workspace lifecycle and stable URL control plane;
- Fabric and Ledger service boundaries implemented here;
- contracts and integration evidence for external product components.

The full goal is multi-repository:

- local App project/task storage and sync behavior belongs in the App repository;
- Workspace runtime file/task behavior belongs in `one-person-lab-app`;
- Gateway routing internals belong in the Gateway repository;
- product-level contracts belong in `one-person-lab-cloud` and are mirrored here only when this repository has an executable boundary.

No missing external capability will be reimplemented inside Control Plane merely to make this repository appear complete.

## Current Baseline

The current repository already provides a controlled commercial pilot for CPU Workspaces:

- account and admin login;
- CPU compute allocations, storage volumes, attachments, and stable Workspace URLs;
- PostgreSQL-backed commercial records when `DATABASE_URL` is configured;
- wallet, holds, releases, settlements, audit, evidence, and reconciliation;
- explicit Control Plane, Fabric, Ledger, and Console UI service boundaries;
- production manifests and a real cloud E2E path.

Known foundation gaps include:

- organizations and memberships are held in process memory;
- Control Plane runtime-operation and reconciliation projections retain in-memory state;
- production Fabric can fall back to memory when PostgreSQL is not injected;
- provider reconciliation exists but is not enabled as a production worker;
- task receipts are narrower than the required research execution evidence chain;
- generic jobs, artifacts, connectors, environments, agents, and Gateway usage are not one end-to-end model.

## Shared Contract Spine

All product lines must use the same canonical identities:

```text
Organization
  -> Workspace
      -> Project
          -> Task
              -> ExecutionRequest
                  -> Approval
                      -> Job
                          -> Artifact
                              -> Review
                                  -> Receipt
                                      -> Continuation
```

### Identity Rules

- IDs are opaque, globally unique strings with a stable object prefix.
- The service that owns an object creates its canonical ID.
- A canonical ID never changes when a task moves between local and cloud execution surfaces.
- Local-only projects and tasks may have temporary client IDs before first sync. Promotion to a canonical ID is transactional and records an alias; no second cloud identity is created.
- Provider IDs, Kubernetes names, object keys, and database row IDs are references, never business identity.
- Every write carries an idempotency key and actor, organization, workspace, and caller context where applicable.

### Ownership

| Object | Authority | Durable record |
| --- | --- | --- |
| Organization, Team, Role, Membership | Control Plane | PostgreSQL |
| Workspace | Control Plane | PostgreSQL |
| Project, Task cloud identity and sync head | Control Plane | PostgreSQL |
| ExecutionRequest, Approval | Control Plane | PostgreSQL plus Ledger evidence |
| Job and resource operation | Fabric | PostgreSQL |
| Artifact bytes | Approved object or Workspace storage | Provider storage |
| Artifact manifest and provenance | Ledger | PostgreSQL, append-first |
| Review result | Ledger | PostgreSQL, append-first |
| Receipt and Continuation | Ledger | PostgreSQL, append-first |
| Gateway key policy and route decision | Gateway | Gateway store plus Ledger usage evidence |

Control Plane orchestrates but does not write Fabric or Ledger tables. Ledger records evidence but does not own artifact bytes. Fabric executes but does not decide organization policy or billing truth.

### Shared Execution States

The product-level flow is:

```text
request -> plan -> approve -> execute -> monitor -> collect
        -> review -> settle -> receipt -> continue
```

Task states:

```text
draft
planned
awaiting_approval
queued
running
review_required
review_blocked
completed
failed
cancelled
archived
```

Job states:

```text
queued
provisioning
running
collecting
succeeded
failed
cancelled
timed_out
```

The public state is derived from durable facts. Services must not depend on a process-memory projection as the only truth.

### Receipt Schema

The general receipt links the complete evidence chain:

```text
request
plan
approval
execution
environment
input refs
output refs
reviewer checks
cost
receipt
continuation
```

Required top-level fields:

- `receiptId`, `schemaVersion`, `status`, `surface`, and timestamps;
- actor, organization, workspace, project, task, request, approval, and job references;
- environment, connector, model, agent, code, command, and input references when used;
- output artifact manifests with content digest, media type, size, and storage reference;
- review results, policy result, owner, usage, and cost;
- `continuationRef` with the authorized task checkpoint and required environment/artifact references.

Receipt writes are append-first and idempotent. Corrections append a superseding record; they never silently mutate historical evidence. Secrets, tokens, raw credentials, signed URLs, object keys, and kubeconfig content are forbidden from receipts.

## Workspace 100 Percent

Workspace is complete when local and cloud are equivalent execution surfaces for the same durable project and task.

### Sync

Each synced mutation contains:

- canonical or temporary object ID;
- entity type and operation ID;
- client ID and base server version;
- payload or content digest;
- timestamp and idempotency key.

The server assigns a monotonically increasing entity version and a workspace sync cursor.

Conflict policy:

- append-only events with distinct operation IDs merge automatically;
- disjoint metadata fields may merge automatically when both base versions are known;
- concurrent edits to the same structured field return a conflict containing both versions;
- concurrent file edits preserve both file bodies and require an explicit user choice;
- conflict resolution itself is a new versioned mutation and receipt reference.

Offline clients use a durable outbox and cursor-based inbox. File transfer is content-addressed, chunked, resumable, checksum-verified, and independent from metadata sync.

### Continue Across Surfaces

A task may be created locally, approved and executed in cloud, then resumed locally. Continuation requires:

- the same `projectId` and `taskId`;
- the receipt and latest task version;
- required artifact digests;
- environment and model references;
- checkpoint or command context that contains no secret;
- an authorization check at resume time.

### Backup And Lifecycle

Workspace supports:

- scheduled volume snapshots and on-demand backups;
- backup manifests covering project/task metadata, artifact manifests, and storage snapshot identity;
- one-click restore to the same Workspace;
- clone to a new Workspace with new access credentials and preserved provenance;
- export and provider-neutral migration manifests;
- sleep, cold start, runtime upgrade, storage expansion, and compute migration;
- restore verification that reads known files and validates task and receipt references.

Default GA recovery targets:

- control metadata RPO: at most 15 minutes;
- control metadata RTO: at most 60 minutes;
- default Workspace volume RPO: at most 24 hours plus on-demand snapshots;
- Workspace restore RTO: at most 4 hours for the launch size limit.

The targets are release gates and must be proven by a real disaster exercise, not inferred from provider documentation.

### User Surface

Workspace and Console expose one user-visible path for:

- project and task status;
- execution and approval state;
- artifacts and downloads;
- review results and blocked actions;
- receipts and continuation;
- backup, restore, clone, export, and migration history.

## Console 100 Percent

Console is the durable governance and commercial control surface.

### Identity And Access

- registration, invitation, email verification, password reset;
- MFA, SSO, session listing, targeted revocation, and global sign-out;
- durable Organization, Team, Role, Membership, and invitation records;
- organization-scoped RBAC enforced server-side on every protected object;
- account freeze and deletion workflows that preserve required financial and audit evidence.

### Approval And Policy

One approval engine covers Workspace, Gateway, Connector, Environment, Agent, and high-cost resource requests. Approval records contain approver, scope, permitted resources, budget, expiry, and decision reason.

Approvals are checked again immediately before execution. Expired, revoked, or scope-mismatched approvals fail closed.

### Quota And Budget

Quota and budget may be scoped to user, team, organization, Workspace, model, connector, environment, and resource class. Reservation happens before execution; usage settlement and release are idempotent.

### Payments

- payment order, channel callback, top-up settlement, refund, and invoice records;
- callback signature verification and replay protection;
- explicit reconciliation between payment orders, wallets, usage, and Ledger entries;
- operator recovery paths that append audit evidence.

### Notifications And Compliance

- email, in-product, and Webhook delivery;
- user preferences, retries, dead-letter visibility, and delivery audit;
- complete admin audit, data export, retention, privacy deletion, and account closure;
- compliance policy is explicit about immutable financial/evidence facts versus deletable personal content.

## Fabric 100 Percent

Fabric presents provider-neutral execution and resource contracts to App, Workspace, MAS, and Control Plane.

### Durable Operations

- production requires PostgreSQL and fails readiness if it is absent;
- every provider request, retry, result, and external resource reference is durable;
- a production reconcile worker compares desired and observed state;
- drift, external deletion, partial creation, and orphaned resources produce explicit operations and alerts.

### Registries

OPL Connect provides connector registration, credential references, approval requirements, rate limits, retries, health, and audit. The first production connector is PubMed read-only with normalized citation output and deterministic source references.

Environment Catalog provides immutable, versioned definitions for Python, R, CUDA, Quarto, and LaTeX. A job records the resolved digest, not a mutable label such as `latest`.

Agent Registry provides package identity, publisher, version, review status, resource requirements, permissions, environment, and instantiation contract.

### Adapters And Scheduler

Fabric supports adapter contracts for:

- CPU and GPU jobs;
- Docker jobs;
- Workspace runtime jobs;
- SSH and HPC batch execution;
- object storage and database access.

The scheduler provides queueing, placement, retry policy, cancellation, timeout, output collection, lease expiry, cleanup, and orphan reconciliation. An adapter may expose provider-specific diagnostics to operators, but consumers use only Fabric objects and states.

## Ledger 100 Percent

Ledger is both the financial ledger and the evidence ledger.

### Evidence Records

- general receipt schema and idempotent write/query APIs;
- artifact manifests and references to data, code, commands, environments, connectors, models, and agents;
- App, Workspace, Gateway, Fabric, and Agent Run receipts using the same envelope;
- immutable review results and policy decisions;
- signed human-readable receipt pages and export packages;
- continuation references that can restart an authorized task from recorded evidence.

### Reviewer Gate

Reviewer Gate is a generic policy boundary. MAS, MAG, RCA, BookForge, and future reviewers plug into it by publishing:

- reviewer identity and version;
- input artifact digests;
- checks and structured findings;
- pass, fail, blocked, or human-review-required decision;
- evidence and recommended continuation.

Ledger records the decision. It does not embed domain-specific reviewer logic.

### Retention And Privacy

- evidence is append-first and tamper-evident;
- retention policy is explicit by receipt and artifact class;
- privacy deletion removes or tombstones permitted personal content while retaining legally required accounting facts;
- reconciliation links Gateway usage, Fabric jobs, payment settlement, wallet movement, and receipts;
- unexplained money, usage, jobs, or provider resources block GA promotion.

## Gateway Cross-Cutting Track

Gateway is required for the One Person Lab Cloud vision even though its internals are outside this repository.

It must provide:

- organization-scoped keys and revocation;
- provider and model routing policy;
- model allowlists, fallback policy, and data-handling policy;
- reservation, quota, rate limit, and budget enforcement;
- normalized token and request usage;
- cost attribution to organization, Workspace, project, task, job, and receipt;
- route-decision and usage evidence delivered to Ledger;
- Console read models for keys, usage, quota, policy, and cost.

Gateway contract work lands before its Console and Ledger consumers. Gateway implementation is delivered in its owning repository and verified through cross-repository contract and production tests.

## Delivery Phases

### Phase 0: Contract Spine

Deliver:

- canonical IDs and ownership;
- shared execution states and mutation envelopes;
- receipt schema and continuation contract;
- service APIs and error semantics;
- contract fixtures consumed by App, Control Plane, Fabric, Ledger, and Gateway tests.

Exit gate: the same fixture can be parsed and validated by every participating repository. No product worktree may introduce a competing object or state definition.

### Phase 1: Persistence And Identity

Deliver:

- organization, membership, operation projection, and reconciliation persistence;
- mandatory production PostgreSQL for Fabric;
- project/task cloud identity and sync head;
- durable Gateway key and organization binding contract.

Exit gate: restart and rollout tests preserve all business state; production readiness rejects memory fallback.

### Phase 2: Execution Loop

Deliver one complete path:

```text
local task -> cloud sync -> approval -> Fabric job -> artifact
           -> Ledger receipt -> local continuation
```

Include Gateway usage and cost evidence when a model is used.

Exit gate: a real task crosses surfaces without changing IDs or losing task state, files, artifacts, approval, cost, or evidence.

### Phase 3: Product Breadth

Deliver:

- robust bidirectional sync and resumable transfer;
- PubMed Connect;
- versioned environment catalog;
- GPU, Docker, SSH, HPC, object storage, and database adapters;
- Agent Registry and instantiation;
- Workspace backup, restore, clone, export, upgrade, and migration.

Exit gate: each adapter and registry passes the same execution, cancellation, collection, receipt, and cleanup contract suite.

### Phase 4: Governance And Evidence

Deliver:

- full account security and organization governance;
- approval policy, quota, budget, payment, refund, invoice, and notification flows;
- Reviewer Gate integrations;
- signed receipts, exports, retention, privacy deletion, and reconciliation.

Exit gate: policy bypass, duplicate payment callback, duplicate usage event, duplicate job completion, and receipt replay tests all fail safely or replay idempotently.

### Phase 5: Reliability

Deliver:

- sync conflict recovery and offline recovery;
- provider drift reconciliation and orphan cleanup;
- retry, cancellation, timeout, and interrupted collection recovery;
- scheduled backups and real disaster restore drills;
- migration and upgrade rollback procedures.

Exit gate: fault injection demonstrates no silent data loss, double charge, duplicate resource, or orphaned execution.

### Phase 6: Production Readiness

Deliver:

- capacity, load, security, and dependency-failure tests;
- dashboards, alerts, SLOs, runbooks, on-call ownership, and rollback;
- staged rollout, canary verification, and controlled GA promotion;
- final capability matrix and production evidence package.

Exit gate: all production gates below pass with no unresolved critical finding.

## Worktree And Merge Protocol

Phase 0 uses one dedicated short-lived contract worktree and lands before parallel work begins.

Each later integration train creates at most four repository-local worktrees:

```text
feat/train-<n>-workspace
feat/train-<n>-console
feat/train-<n>-fabric
feat/train-<n>-ledger
```

Rules:

1. All branches start from the same clean `main` commit.
2. Each worktree owns its service paths and tests. Shared contract changes go through a contract-spine patch first.
3. A train has one vertical acceptance scenario. It does not collect unrelated backlog work.
4. Database migrations are additive until all consumers in the train have moved. A compatibility service or duplicate long-term API is forbidden.
5. The normal merge order is Ledger, Fabric, Console/Control Plane, then Workspace-facing integration. A train may change the order only when its dependency graph proves another order.
6. Every branch passes focused tests before merge.
7. After all train branches land, `main` passes the complete repository suite and cross-service E2E.
8. Push `main`, delete merged remote branches, remove physical worktrees, delete local branches, prune worktree metadata, and verify a clean workspace.
9. External App and Gateway repositories use their own short-lived worktrees and land contract-compatible releases before the consuming train exits.

Long-lived product branches and a final large-bang merge are explicitly rejected.

## API And Error Semantics

All mutating service APIs use:

- authenticated actor and organization context;
- idempotency key;
- request/correlation ID;
- canonical object references;
- structured error code and retryability;
- optimistic version when updating mutable state.

Common outcomes:

- `400`: invalid contract or unsupported transition;
- `401/403`: missing authentication or policy denial;
- `404`: canonical object is absent or not visible;
- `409`: idempotency fingerprint mismatch, version conflict, or invalid concurrent transition;
- `422`: valid request cannot run because a required approval, environment, connector, or artifact is unavailable;
- `429`: quota or rate limit;
- `503`: retryable provider or dependency failure.

Retries reuse the same idempotency key. A timeout is not proof of failure; clients query the durable operation before retrying with a new operation.

## Verification Strategy

### Smallest Runnable Checks

Every non-trivial contract or state machine leaves one focused runnable check. Tests consume shared fixtures where possible rather than copying state definitions.

### Per-Branch

- focused Go or TypeScript tests for changed ownership boundaries;
- migration up/down or forward/restore validation as applicable;
- `git diff --check`;
- contract fixture validation.

### Per-Train

- `npm test`;
- `npm run typecheck`;
- `npm run build`;
- `go test ./...` in Control Plane, Ledger, and Fabric;
- structural and contract checks;
- cross-service integration tests with PostgreSQL;
- browser verification for user-visible flows;
- provider emulator tests plus selected real staging cloud E2E.

### Production Evidence

- real App-to-cloud-to-App continuation;
- backup restore after deliberate compute and control-plane replacement;
- payment and usage reconciliation;
- provider drift and orphan cleanup exercise;
- permission and approval bypass tests;
- signed receipt export and independent verification;
- post-test proof of no leaked resources, active holds, or unexplained charges.

## Stable GA Gates

Public GA requires all of the following:

- no production business state has an in-memory-only authority or silent fallback;
- schema migration, backup, restore, and rollback are exercised in staging;
- monthly control API and Workspace entry availability target is at least 99.9%;
- launch load test sustains at least twice the initial planned peak with defined saturation behavior;
- no open severity-0 or severity-1 defect and every severity-2 defect has explicit risk acceptance;
- secrets are referenced, rotated, redacted, and absent from receipts, logs, and exports;
- MFA is available and required for privileged operators;
- payment, usage, wallet, receipt, job, and provider-resource reconciliation has no unexplained delta;
- disaster exercises meet the declared RPO and RTO;
- canary and rollback are automated and verified;
- runbooks name an owner and a concrete recovery action for every paging alert;
- a controlled pilot completes before unrestricted GA.

## Completion Matrix

A product line reaches 100 percent only when:

1. Every capability in its section is implemented in the owning repository.
2. Its durable state survives restart, rollout, and restore.
3. Its user-facing path exists and enforces permissions.
4. Its API and evidence contracts are covered by automated tests.
5. Its failure, retry, cancellation, and recovery paths are exercised.
6. Its cross-product scenario passes in staging and selected production canaries.
7. Its operational dashboard, alert, runbook, and owner exist.
8. No critical truth is represented only by a backlog contract, mock, process-memory map, or manual operator assumption.

The One Person Lab Cloud vision reaches 100 percent only when all four lines and the Gateway cross-cutting track pass this matrix together.

## Rejected Alternatives

### Four Long-Lived Worktrees

Rejected because shared identities, migrations, and state machines would drift, while late integration would hide incompatible domain models until the most expensive point.

### One Giant Platform Rewrite

Rejected because current CPU Workspace, billing, Fabric, and Ledger boundaries already provide reusable foundations. Replacing them would increase risk without proving new product value.

### Complete One Product Before Starting The Next

Rejected because none of the four products reaches user value alone. Vertical integration trains produce earlier evidence and expose contract errors while they are still cheap to fix.

## Next Step

After this design is approved, create an implementation roadmap with:

- one execution plan for Phase 0;
- one plan per integration train, not one plan per permanent product branch;
- explicit cross-repository dependencies for App and Gateway;
- task-level file ownership, tests, merge order, and cleanup commands.
