# Project Scope

This repository implements the OPL Cloud backend and display-only Console UI for OPL Workspace, OPL Console, OPL Fabric, and OPL Ledger. OPL Gateway remains an external service boundary.

## External Sources

- `one-person-lab` owns the development framework: goal, attempt, readiness, receipt, blocker, next step, human gate, recovery, evidence, docs lifecycle, and contract-light rules.
- `one-person-lab-cloud` owns the product definition: OPL Gateway, OPL Workspace, OPL Console, OPL Fabric, and OPL Ledger.

## Repository Ownership

This repository owns:

- OPL Console account, billing, access, resource allocation, support, and admin surfaces.
- OPL Workspace identity, URL delivery, metadata sync, resumable content transfer, and recovery orchestration.
- OPL Fabric as the standalone PostgreSQL-backed owner of Tencent Cloud and Kubernetes resource operations, runtime access facts, content transfer, and storage snapshots.
- OPL Ledger as the standalone PostgreSQL-backed owner of wallets, holds, settlements, reconciliation, general receipts, Artifact Manifests, Review Results, and continuation references.
- Tencent TKE runtime handoff and production diagnostics.
- ComputePool, ComputeAllocation, StorageVolume, and StorageAttachment contracts.
- Runtime readiness, production readiness, production manifests, and TKE deployment handoff.

## Out Of Scope

This repository does not own:

- OPL Gateway internals.
- One Person Lab framework internals.
- `one-person-lab-app` WebUI behavior.
- Domain reviewer implementation logic and artifact byte storage.
- Connector, environment, and agent marketplaces beyond approved catalog boundaries.

## Development Rule

Before adding a check or feature, classify it as:

1. One Person Lab framework rule.
2. OPL Cloud product rule.
3. This repository's Console, Workspace, Fabric, Ledger, or external Gateway integration rule.

Implement it here only when this repository owns the rule or the integration boundary.
