# Status

## Current Boundary

Current status is a controlled CPU Workspace pilot, not public GA.

Implemented:

- owner/admin login and server-side tenant isolation;
- live-verified positive `sub2apiUserId` account mapping before user creation;
- live Sub2API USD balance in Console;
- Basic monthly compute purchase;
- storage purchase in 10 GB blocks;
- stable purchase/renewal redeem codes and recovery states;
- renewal, auto-renew control, expiration, retained storage, and entitlement gates;
- dedicated Tencent CVM, CBS, attachment, runtime, and Workspace URL flows;
- Ledger billing receipts and product-scoped receipt lookup;
- separate PostgreSQL-backed Control Plane, Fabric, and Ledger services;
- TKE deployment workflow and a legacy production-verifier implementation.

Not ready:

- Pro purchase and provider evidence, although its `8c16g` product definition is approved;
- Sub2API reserve/capture/release integration for monthly resource settlement;
- prepaid Tencent CVM/CBS procurement and renewal;
- the reusable `SA5.MEDIUM2` plus 10GB CBS Verification Slot;
- safe release verification; the legacy paid verifier is blocked and is not a release gate;
- GPU packages;
- public self-registration or a reusable unified identity system;
- production backup/restore, because the current TKE snapshot installation does
  not expose the required GA `snapshot.storage.k8s.io/v1` API;
- public GA operational evidence.

Sub2API source, API keys, routing, request usage, and balance remain outside this
repository. Console links to the external Gateway and never mirrors its key or
usage database.

## Completion Gate

```bash
npm test
npm run typecheck
npm run build
(cd services/control-plane && go test ./...)
(cd services/fabric && go test ./...)
(cd services/ledger && go test ./...)
git diff --check
```

Production delivery additionally requires CI, immutable image publication,
bounded rollout status for all three services, recovery evidence, and the
reusable verification receipt defined by `docs/invariants.md`.
