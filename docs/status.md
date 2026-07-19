# Status

## Current Boundary

Current status is a Fast Invite-Only Paid Pilot candidate for 2-5 customer
accounts. It is locally code-complete through Task 13A, but not production-proven and not
yet saleable.

Code-complete and locally tested:

- one Console User to one Account to one Sub2API User/Wallet identity hard cut,
  normalized-email verification, Sub2API password authority, and tenant Sessions;
- granular source DTOs for Auth, Wallet, Key list/status, Usage, Usage Stats,
  balance history, Workspace, Runtime readiness, and Ledger receipts;
- fixed Basic `52_580_000` and Pro `240_080_000` USD-micros monthly Workspace prices;
- one-submit durable Workspace launch with debit-before-provider recovery;
- one Workspace-level renewal operation, while enabling `autoRenew` remains blocked;
- PREPAID Tencent CVM/CBS request/readback, retained CBS expiry behavior, Runtime,
  owner-only credential commands, and account-scoped Gateway Secret handling;
- Ledger validation, receipts, reconciliation, and replay safety;
- dual Basic/Pro Provider Acceptance code and one-request Basic release live-QA;
- immutable Ready-Pod imageID checks, security boundaries, and grouped rollback;
- the four-surface customer Console, five-surface operations Console, retired
  local-user deployment seed, Sentrux structural gate, and desktop/mobile
  source-truth browser QA with independent unavailable states.

Remaining blockers:

- Basic and Pro Provider Acceptance has not run and no real Tencent resource
  evidence exists for this candidate;
- no approved real renewal, production rollout, browser login/WebSocket, model
  request, exact-one Usage/wallet delta, or rollback evidence exists;
- public registration, payment/order UI, Key mutation, backup/recovery/sync/
  transfer, HA, GPU, and multiple Workspaces are outside the Pilot.

Workspace file bodies remain only on CBS. Platform PostgreSQL contains identity,
operation, reference, and audit facts only; PostgreSQL recovery does not back up
or restore Workspace files.

## Completion Gate

```bash
npm test
npm run typecheck
npm run lint
npm run build
(cd services/control-plane && go test ./... -count=1)
(cd services/fabric && go test ./... -count=1)
(cd services/ledger && go test ./... -count=1)
(cd services/internal/postgresmigrate && go test ./... -count=1)
sentrux check .
git diff --check
```

The four PostgreSQL suites must use local or CI isolated databases and complete
with zero SKIP; production PostgreSQL is forbidden for this gate.

Production delivery additionally requires immutable image publication, both
retained Acceptance slots, one approved Basic live-QA request, bounded rollout
for all three services, source-truth readback, and the evidence defined by
`docs/invariants.md`.
