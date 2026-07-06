# Architecture

## Product Layers

OPL Cloud has five product layers:

- OPL Gateway: AI provider routing, token policy, and request usage policy.
- OPL Workspace: URL-delivered working environment running `one-person-lab-app`.
- OPL Console: account, workspace, billing, access, support, and admin control surface.
- OPL Fabric: compute, storage, image, route, and infrastructure handoff.
- OPL Ledger: billing ledger, wallet transactions, audit events, receipts, verifier evidence, and reconciliation.

## Current Implementation Slice

This repository is moving from one Node Console control-plane process to four explicit service boundaries:

- `apps/console-ui`: React + TypeScript browser UI. It owns no persistence and calls only the Control Plane API.
- `services/control-plane`: Go UI-facing API and product orchestrator. It owns auth, organizations, users, workspaces, support, operation requests, and UI projections. It calls Ledger and Fabric through typed HTTP clients.
- `services/ledger`: Go service backed by PostgreSQL. It owns wallets, holds, manual top-ups, ledger entries, wallet transactions, receipts, audit events, evidence references, idempotency keys, and reconciliation.
- `services/fabric`: Go service boundary for Tencent Cloud and Kubernetes operations. It owns resource catalog, compute, storage, attachments, runtimes, provider request ids, and retryable resource operations.
- `packages/contracts`: machine-readable product, route, lifecycle, billing, management, storage, evidence, and service-boundary contracts.

## Boundaries

Console UI may call Fabric and Ledger only through Control Plane routes. It must not import PostgreSQL drivers, Tencent Cloud SDKs, Kubernetes clients, or service internals.

Control Plane may call Fabric and Ledger only through published service APIs. It must not write ledger tables directly and must not call Tencent Cloud or Kubernetes clients directly.

Ledger owns money and evidence persistence. All Ledger write APIs require idempotency keys and use append-first writes.

Fabric owns cloud resource operations. Tencent Cloud SDK and Kubernetes client-go imports live under `services/fabric` only.

Fabric details such as TKE, Docker, Ingress, PVC/CBS, node-pool allocation, and runtime operation evidence are admin/operator surfaces. Lab Owner UI should expose product status and allowed actions, not raw infrastructure evidence.

Ledger details such as dedup rows, request fingerprints, and raw event payloads are admin/operator surfaces. Lab Owner UI should expose wallet balance, holds, recent charges, usage, top-ups, and human-readable receipts.

## Persistence

PostgreSQL is the production persistence target. JSON state remains a local development store.

Commercial identity, wallet, Workspace, billing, support, audit, and receipt data must persist across rollouts when `DATABASE_URL` is configured.

## No Compatibility Layer

Legacy Node API and store paths are retired directly after active callers move. The repository must not keep a Node proxy, BFF, or compatibility route layer in front of the Go services. The only acceptable bridge is a one-time state migration that reads the old data shape and writes the new service-owned tables.
