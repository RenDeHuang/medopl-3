# Invariants

## Workspace Resource Mapping

```text
1 ComputePool = package-level Tencent TKE node pool for one fixed compute specification
1 ComputeAllocation = account-owned dedicated CVM node inside one ComputePool
1 StorageVolume = account-owned persistent storage
1 StorageAttachment = one storage volume mounted to one ComputeAllocation runtime
1 OPL Workspace = stable URL token entry backed by one StorageVolume and the current ComputeAllocation/StorageAttachment runtime pointer
1 RuntimeTemplate = deployable application image for the Workspace runtime; the default image is one-person-lab-app
```

One Lab Owner can own multiple compute allocations, storage volumes, attachments, and OPL Workspace URL entries.

Fabric-managed ComputePools are explicit placement pools, not user resource identities. TKE autoscaling and node-pool auto repair must stay disabled; every billable CVM is created, owned, billed, and destroyed as a Console ComputeAllocation.

RuntimeTemplate/ImageRef is not a billing object. Replacing the default app image must not change resource ownership, storage ownership, Workspace URL identity, or Ledger semantics.

## Access

Workspace URLs are long-lived token URLs:

```text
https://workspace.medopl.cn/w/<workspaceId>/?token=<share-token>
```

Opening a Workspace URL does not require member login. The token remains valid until the owner resets or deletes it.

Workspace summaries, readiness reports, and public API responses must not leak the share token unless the route is explicitly authenticated and scoped to the owner/admin.

## Compute And Storage

Compute pools, compute allocations, and persistent storage have separate lifecycles.

Destroying a compute allocation must not destroy persistent storage.

Storage destruction requires explicit confirmation and is the only action that stops storage billing.

## Billing

Billing is hourly.

Before opening a compute allocation, OPL Cloud freezes enough balance for 7 days of compute. Before opening storage, it freezes enough balance for 7 days of storage.

Debits charge available balance first, then the relevant frozen hold.

If compute or storage holds are exhausted, OPL Console records account and runtime notifications and blocks new resource actions until top-up or admin recovery.

If storage hold is exhausted, storage is preserved until top-up or explicit storage destruction.

Manual top-ups must create both a wallet transaction and a manual top-up audit record.

Usage debits must be idempotent by source event or request fingerprint.

## Permissions

Lab Owner sees Workspace distribution, URL actions, package, state, balance, bill explanation, usage, support, alerts, and human-readable receipts.

Admin sees users, roles, manual recharge, audit, runtime readiness, production readiness, raw Ledger evidence, Fabric internals, and external support ticket mappings.

Lab Owner must not see request fingerprints, dedup rows, runtime evidence, production readiness, manual settlement, or raw Ledger event internals.

## Runtime

Production readiness fails closed until runtime provider, registry image, workspace domain, PostgreSQL, Tencent configuration, Go provisioner boundary, required host tools, and auth seed/persistence requirements are satisfied.

Production manifests must not inline secrets. Sensitive values must use secret references or mounted secret files.
