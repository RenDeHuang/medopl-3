# OPL Cloud Implementation Goal Ledger

This repository is the OPL Cloud implementation workspace for the OPL Console and OPL Workspace control-plane slice.

## Product Truth

[`one-person-lab-cloud`](https://github.com/gaofeng21cn/one-person-lab-cloud) owns the Cloud product definition and fixed product layers:

- OPL Gateway
- OPL Workspace
- OPL Console
- OPL Fabric
- OPL Ledger

This repository currently implements the OPL Console / OPL Workspace control plane, with early OPL Fabric and OPL Ledger boundaries.

## Development Truth

[`one-person-lab`](https://github.com/gaofeng21cn/one-person-lab) owns the development framework concepts. Work here is organized by:

- goal
- attempt
- readiness
- receipt
- blocker
- next step
- human gate
- recovery
- evidence

The repository should not accumulate phase-only smoke files, temporary reports, or broad staged narratives that outlive the actual implementation evidence.

## Goal

Support this business chain:

```text
PI signs in to OPL Console
-> creates an OPL Workspace
-> OPL Cloud creates one server, one cloud disk, one one-person-lab-app Docker runtime, and one URL
-> PI shares the URL
-> members enter the OPL Workspace without login
-> OPL Console manages lifecycle, billing, audit, readiness, recovery, and evidence
```

Resource invariant:

```text
1 OPL Workspace
= 1 server
= 1 one-person-lab-app Docker container
= 1 cloud disk
= 1 URL
```

Server and cloud disk lifecycles stay separate. Server stop or destroy must not destroy the cloud disk. Disk destruction is explicit and is the only action that stops storage billing.

## Current Attempts And Receipts

### Console And Workspace Control Plane

Attempt:

- Implement the PI-facing OPL Console for workspace distribution.
- Keep Workspace URLs token-gated and usable without member login.
- Preserve one PI account to many Workspaces.

Receipts:

- `src/main.jsx`
- `services/api/src/opl-cloud.js`
- `tests/domain/workspace-lifecycle.test.js`
- `tests/domain/workspace-url-route.test.js`
- `contracts/opl-cloud-product-contract.json`
- `contracts/opl-cloud-workspace-lifecycle-contract.json`

### OPL Fabric Runtime Providers

Attempt:

- Keep Local Docker as the local runtime loop.
- Keep Tencent CVM as the cloud runtime provider.
- Hand off cloud provisioning through OpenTofu, Ansible, Caddy, Harbor image contracts, and Tencent CLI boundaries.

Receipts:

- `services/api/src/runtime-provider-factory.js`
- `services/api/src/runtime-providers/local-docker.js`
- `services/api/src/runtime-providers/tencent-cvm.js`
- `infra/tencent-cvm/`
- `tests/providers/local-docker-provider.test.js`
- `tests/providers/tencent-cvm-provider.test.js`
- `tests/providers/tencent-cvm-ansible.test.js`
- `tests/providers/server-provider-config.test.js`

### OPL Ledger And Evidence

Attempt:

- Keep OPL Console as the v1 billing truth.
- Emit OpenMeter usage events when configured.
- Preserve operation attempts, billing ledger entries, audit events, verifier output, and Tencent bill reconciliation evidence.

Receipts:

- `services/api/src/openmeter.js`
- `services/api/src/billing-reconciliation.js`
- `services/api/src/store.js`
- `tools/reconcile-tencent-bills.js`
- `tools/production-verifier.js`
- `tests/billing/`
- `tests/persistence/postgres-store.test.js`
- `tests/production/production-verifier.test.js`

### Production Readiness And Handoff

Attempt:

- Fail closed until production runtime provider, Harbor image, workspace domain, PostgreSQL, OpenMeter, Tencent environment, and required host tools are ready.
- Validate the production manifest without leaking secrets.
- Keep real cloud verification behind an operator-controlled human gate.

Receipts:

- `services/api/src/production-readiness.js`
- `services/api/src/production-manifest.js`
- `deploy/production-manifest.example.json`
- `docs/PRODUCTION_RUNBOOK.md`
- `tools/validate-production-manifest.js`
- `tests/production/production-readiness.test.js`
- `tests/production/production-manifest.test.js`
- `tests/production/production-manifest-cli.test.js`

## Readiness Gates

Local readiness:

```text
GET /api/runtime/readiness
```

Production readiness:

```text
GET /api/production/readiness
```

Manifest readiness:

```bash
npm run validate:production-manifest -- --manifest deploy/production-manifest.example.json
```

Structural readiness:

```bash
sentrux check .
```

Development verification:

```bash
npm test
npm run build
```

## Human Gates

The following actions require explicit human approval before execution:

- Renaming the GitHub repository.
- Renaming the local folder.
- Running `npm run verify:production`.
- Creating real Tencent CVM, CBS, DNS, or OpenMeter events.
- Injecting or confirming production secrets.

## Active Blockers

Preproduction launch remains blocked until the operator provides or confirms:

Required missing inputs:

- `DATABASE_URL`
- `OPENMETER_ENDPOINT`
- `OPENMETER_API_KEY`
- `OPL_IMAGE_ID`
- `OPL_SSH_KEY_ID`

Needs human confirmation:

- `OPL_HARBOR_REGISTRY`
- `OPL_WORKSPACE_IMAGE`
- `OPL_WORKSPACE_DOMAIN`
- `TENCENTCLOUD_SECRET_ID`
- `TENCENTCLOUD_SECRET_KEY`
- `OPL_VPC_ID`
- `OPL_SUBNET_ID`
- `OPL_SECURITY_GROUP_ID`
- `OPL_AVAILABILITY_ZONE`

Do not print secret values. Do not commit `.env.preproduction*`.

## Next Step

Close the implementation repository around the current product truth, fix production readiness correctness, rerun safe verification, and leave real cloud verification gated for the operator.
