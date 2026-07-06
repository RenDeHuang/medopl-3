# OPL Ledger Pricing Hardening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make OPL Cloud billing a resource-only ledger with an OPL user price catalog, provider cost snapshots as internal evidence, owner/resource traceability, Admin/Console evidence display, and bounded resource usage log growth.

**Architecture:** OPL Cloud keeps resource billing in Console/Ledger and leaves request-level billing to one-person-lab-cloud. Fabric remains responsible for provisioning CVM/Node, storage, attachments, and URL evidence; Ledger records holds, debits, releases, destruction, and resource usage snapshots. Pricing uses final user prices for billing; provider/Tencent prices are stored only as internal cost evidence and reconciliation metadata.

**Tech Stack:** Node.js ESM, `node:test`, React/Ant Design Console UI, JSON contracts in `packages/contracts`, in-repo Memory/Postgres store abstractions.

---

## File Map

- Modify `packages/contracts/opl-cloud-pricing-contract.json`: rename commercial meaning from markup-based defaults to final OPL user price catalog; add provider cost estimate fields.
- Modify `packages/contracts/opl-cloud-billing-ledger-contract.json`: remove "Tencent cost + markup" as billing rule; require price snapshots and resource owner refs on ledger entries.
- Modify `packages/contracts/opl-cloud-route-api-contract.json`: remove request-debit wording from OPL Cloud billing routes; require Admin resource/ledger evidence fields.
- Modify `packages/console/src/services/pricing-service.js`: bill from final OPL user prices; expose provider cost snapshot helpers separately.
- Modify `packages/console/src/services/resource-provisioning-service.js`: write final user price snapshots and provider cost snapshots into compute/storage holds and resource records.
- Modify `packages/console/src/services/billing-service.js`: resource-only billing metadata; remove request debit from OPL Cloud billing policy and hot ledger index expectations.
- Modify `packages/console/src/services/console-read-model-service.js`: expose owner/resource/price evidence and resource usage rollups for Admin/Console.
- Modify `packages/console/src/store.js`: add optional resource usage aggregation/archive collections; keep ledger and wallet transactions append-only.
- Modify `packages/console/ui/pages/billing/BillingPage.jsx`: show only resource billing, wallet holds, resource usage, and reconciliation evidence.
- Modify `packages/console/ui/pages/resources/ResourceProvisioningPages.jsx`: show final user prices, provider cost evidence as internal/admin detail, and owner/resource links.
- Modify `packages/console/ui/pages/admin/AdminPages.jsx` or current admin management page files: expose account-owned compute/storage/workspace URL ledger evidence.
- Modify `.github/workflows/deploy-tke-production.yml` and `.env` examples if they still imply markup billing or wrong Pro instance type.
- Modify tests under `tests/contracts`, `tests/billing`, `tests/domain`, `tests/ui`, `tests/production`, and `tests/persistence`.

---

### Task 1: Create Isolated Worktree And Protect Current Main

**Files:**
- No code files changed in this task.

- [ ] **Step 1: Inspect current workspace**

Run:

```bash
git status --short --branch
git worktree list
```

Expected:

```text
## main...origin/main
```

If local changes exist in the main worktree, do not modify them. Continue in a new worktree.

- [ ] **Step 2: Create feature worktree**

Run:

```bash
mkdir -p /home/dev/.config/superpowers/worktrees/medopl-3
git worktree add -b ledger-pricing-hardening /home/dev/.config/superpowers/worktrees/medopl-3/ledger-pricing-hardening main
cd /home/dev/.config/superpowers/worktrees/medopl-3/ledger-pricing-hardening
```

Expected:

```text
Preparing worktree
HEAD is now at <main-sha>
```

- [ ] **Step 3: Confirm feature worktree is clean**

Run:

```bash
git status --short --branch
```

Expected:

```text
## ledger-pricing-hardening
```

---

### Task 2: Lock Product Pricing Contract Semantics With RED Tests

**Files:**
- Modify: `tests/production/production-env-contract.test.js`
- Modify: `tests/production/tke-deploy-workflow.test.js`
- Modify: `tests/contracts/business-object-contract.test.js`
- Modify: `tests/billing/prepaid-ledger-billing.test.js`

- [ ] **Step 1: Add pricing contract assertions**

In `tests/production/production-env-contract.test.js`, add this test after the existing pricing contract test:

```js
test("pricing contract is an OPL user price catalog, not a Tencent markup formula", async () => {
  const contract = JSON.parse(await readFile(pricingContractPath, "utf8"));

  assert.equal(contract.priceBasis, "opl_user_price_catalog");
  assert.equal(contract.currency, "CNY");
  assert.equal(contract.computeHourly.basic, 0.468);
  assert.equal(contract.computeHourly.pro, 1.38);
  assert.equal(contract.storageGbMonth, 0.432);
  assert.equal(contract.providerCostBasis, "internal_estimate_only");
  assert.equal(contract.providerCostEstimate.computeHourly.basic.instanceType, "SA5.MEDIUM4");
  assert.equal(contract.providerCostEstimate.computeHourly.pro.instanceType, "SA5.2XLARGE16");
  assert.equal(contract.providerCostEstimate.storageGbMonth.storageClass, "premium_cbs");
  assert.equal(contract.markup, undefined);
});
```

Pricing choices are intentionally final user prices:

```text
basic 2C4G: 0.468 CNY/hour
pro 8C16G: 1.38 CNY/hour
storage premium CBS: 0.432 CNY/GB/month
```

The `1.38` pro price is a user catalog price above approximate Tencent `SA5.2XLARGE16` cost, not a dynamic Tencent formula.

- [ ] **Step 2: Add Pro instance contract assertion**

In `tests/production/tke-deploy-workflow.test.js`, replace assertions that expect `SA5.LARGE16` with `SA5.2XLARGE16`. Add this focused test:

```js
test("production Pro package maps 8c16g to the real 8C16G Tencent instance type", async () => {
  const workflow = await readFile(deployWorkflowPath, "utf8");
  assert.match(workflow, /OPL_PRO_COMPUTE_INSTANCE_TYPE:[^\n]*SA5\.2XLARGE16/);
  assert.doesNotMatch(workflow, /OPL_PRO_COMPUTE_INSTANCE_TYPE:[^\n]*SA5\.LARGE16/);
});
```

- [ ] **Step 3: Add ledger metadata assertions**

In `tests/billing/prepaid-ledger-billing.test.js`, add:

```js
test("resource ledger entries snapshot final user price and provider cost separately", async () => {
  const service = createService();
  await service.manualTopUp({ accountId: "pi-alpha", amount: 300, reason: "owner_credit" });

  const compute = await service.createComputeAllocation({
    accountId: "pi-alpha",
    userId: "usr-alpha",
    packageId: "basic",
    name: "billing node"
  });
  await service.processPendingResourceProvisioning({ limit: 1 });

  await service.settleResourceBilling({ accountId: "pi-alpha", hours: 1, sourceEventId: "tick-user-price" });
  const state = await service.getState("pi-alpha");
  const debit = state.billingLedger.find((entry) => entry.type === "compute_debit" && entry.computeAllocationId === compute.id);

  assert.equal(debit.metadata.priceBasis, "opl_user_price_catalog");
  assert.equal(debit.metadata.userPrice.computeHourly, 0.468);
  assert.equal(debit.metadata.providerCostEstimate.instanceType, "SA5.MEDIUM4");
  assert.equal(debit.metadata.providerCostEstimate.billingUse, "internal_reconciliation_only");
  assert.equal(debit.metadata.markup, undefined);
});
```

- [ ] **Step 4: Run RED tests**

Run:

```bash
npm test -- tests/production/production-env-contract.test.js tests/production/tke-deploy-workflow.test.js tests/billing/prepaid-ledger-billing.test.js
```

Expected: FAIL on missing `priceBasis`, `providerCostEstimate`, incorrect Pro instance type, and old `markup` metadata.

---

### Task 3: Convert Pricing Service To Final User Price Catalog

**Files:**
- Modify: `packages/contracts/opl-cloud-pricing-contract.json`
- Modify: `packages/console/src/services/pricing-service.js`
- Modify: `.github/workflows/deploy-tke-production.yml`
- Modify: `.env.production.example` or current env template files found by `rg "OPL_PRO_COMPUTE_INSTANCE_TYPE|OPL_BILLING_MARKUP|OPL_STORAGE_GB_MONTH_CNY"`.

- [ ] **Step 1: Update pricing contract JSON**

Replace `packages/contracts/opl-cloud-pricing-contract.json` with this structure:

```json
{
  "schemaVersion": 2,
  "owner": "OPL Console",
  "purpose": "Versioned OPL user price catalog consumed by local and Tencent TKE production environments.",
  "state": "current",
  "priceBasis": "opl_user_price_catalog",
  "providerCostBasis": "internal_estimate_only",
  "machineBoundary": "Final user-facing compute hourly price and storage GB-month price. Provider costs are internal evidence only and never part of the billing formula.",
  "lifecycle": {
    "type": "long_term_contract",
    "removalCondition": "Replace only with a newer current pricing contract and matching migration notes."
  },
  "catalogVersion": "2026-07-06-opl-user-resource-v1",
  "currency": "CNY",
  "computeHourly": {
    "basic": 0.468,
    "pro": 1.38
  },
  "storageGbMonth": 0.432,
  "providerCostEstimate": {
    "currency": "CNY",
    "source": "tencent_public_price_snapshot",
    "sourceRegion": "na-siliconvalley",
    "billingUse": "internal_reconciliation_only",
    "computeHourly": {
      "basic": {
        "instanceType": "SA5.MEDIUM4",
        "vcpu": 2,
        "memoryGb": 4,
        "estimatedHourly": 0.27
      },
      "pro": {
        "instanceType": "SA5.2XLARGE16",
        "vcpu": 8,
        "memoryGb": 16,
        "estimatedHourly": 1.15
      }
    },
    "storageGbMonth": {
      "storageClass": "premium_cbs",
      "estimatedGbMonth": 0.34
    }
  },
  "env": {
    "basicComputeHourly": "OPL_BASIC_COMPUTE_HOURLY_CNY",
    "proComputeHourly": "OPL_PRO_COMPUTE_HOURLY_CNY",
    "storageGbMonth": "OPL_STORAGE_GB_MONTH_CNY"
  },
  "rules": [
    "OPL user prices are final billing prices, not dynamic Tencent cost formulas.",
    "Provider cost estimates are internal reconciliation evidence and must not change customer debits.",
    "GPU pricing is not part of the current production catalog.",
    "Environment templates and runtime defaults must use this catalog version until a newer contract replaces it."
  ]
}
```

- [ ] **Step 2: Refactor pricing service helpers**

In `packages/console/src/services/pricing-service.js`, replace `pricingMarkup`, `computeHourlyBase`, and `storageGbMonthBase` usage with final user price helpers:

```js
export function userComputeHourly({ packagePlan, pricing }) {
  return money(pricing.computeHourly?.[packagePlan.id] ?? pricing.serverHourly?.[packagePlan.id] ?? 0);
}

export function userStorageGbMonth(pricing) {
  return money(pricing.storageGbMonth ?? pricing.diskGbMonth ?? 0);
}

export function providerCostEstimate({ packagePlan, pricing }) {
  const planEstimate = pricing.providerCostEstimate?.computeHourly?.[packagePlan.id] || {};
  return {
    billingUse: pricing.providerCostEstimate?.billingUse || "internal_reconciliation_only",
    source: pricing.providerCostEstimate?.source || "",
    sourceRegion: pricing.providerCostEstimate?.sourceRegion || "",
    instanceType: planEstimate.instanceType || packagePlan.instanceType || "",
    estimatedHourly: money(Number(planEstimate.estimatedHourly || 0))
  };
}

export function providerStorageCostEstimate(pricing) {
  const estimate = pricing.providerCostEstimate?.storageGbMonth || {};
  return {
    billingUse: pricing.providerCostEstimate?.billingUse || "internal_reconciliation_only",
    source: pricing.providerCostEstimate?.source || "",
    sourceRegion: pricing.providerCostEstimate?.sourceRegion || "",
    storageClass: estimate.storageClass || "",
    estimatedGbMonth: money(Number(estimate.estimatedGbMonth || 0))
  };
}

export function pricedComputeHourly({ packagePlan, pricing }) {
  return userComputeHourly({ packagePlan, pricing });
}

export function pricedStorageGbMonth(pricing) {
  return userStorageGbMonth(pricing);
}

export function hourlyStorageAmount({ packagePlan, pricing, hours }) {
  return money((packagePlan.diskGb * userStorageGbMonth(pricing) / 30 / 24) * hours);
}

export function storageGbHourPrice(pricing) {
  return money(userStorageGbMonth(pricing) / 30 / 24);
}

export function hourlyComputeAmount({ packagePlan, pricing, hours }) {
  return money(userComputeHourly({ packagePlan, pricing }) * hours);
}
```

Keep compatibility with tests that pass `serverHourly` and `diskGbMonth`, but treat them as final test user prices.

- [ ] **Step 3: Update billing policy text**

In `pricing-service.js`, change `billingPolicy` to:

```js
export function billingPolicy(pricing) {
  return {
    currency: "CNY",
    priceBasis: pricing.priceBasis || "opl_user_price_catalog",
    providerCostBasis: pricing.providerCostBasis || "internal_estimate_only",
    prepaidHoldDays: 7,
    minimumBillableHours: 1,
    billingCadence: "hourly",
    fundingOrder: ["available_balance", "frozen_hold"],
    computeHoldExhaustion: "mark_compute_hold_exhausted",
    storageHoldExhaustion: "freeze_workspace_until_top_up_or_storage_destroy",
    storageDestroyConfirmation: "required"
  };
}
```

- [ ] **Step 4: Fix production Pro instance type**

In `.github/workflows/deploy-tke-production.yml`, change the default/fallback for `OPL_PRO_COMPUTE_INSTANCE_TYPE` to:

```yaml
OPL_PRO_COMPUTE_INSTANCE_TYPE: ${{ vars.OPL_PRO_COMPUTE_INSTANCE_TYPE || 'SA5.2XLARGE16' }}
```

Also update any env example that has:

```text
OPL_PRO_COMPUTE_INSTANCE_TYPE=SA5.LARGE16
```

to:

```text
OPL_PRO_COMPUTE_INSTANCE_TYPE=SA5.2XLARGE16
```

- [ ] **Step 5: Run pricing tests**

Run:

```bash
npm test -- tests/production/production-env-contract.test.js tests/production/tke-deploy-workflow.test.js
```

Expected: PASS.

- [ ] **Step 6: Commit pricing contract**

Run:

```bash
git add packages/contracts/opl-cloud-pricing-contract.json packages/console/src/services/pricing-service.js .github/workflows/deploy-tke-production.yml tests/production/production-env-contract.test.js tests/production/tke-deploy-workflow.test.js
git commit -m "feat: make OPL resource pricing a user catalog"
```

---

### Task 4: Harden Resource Ledger Price And Owner Evidence

**Files:**
- Modify: `packages/console/src/services/resource-provisioning-service.js`
- Modify: `packages/console/src/services/billing-service.js`
- Modify: `packages/console/src/services/ledger-evidence-service.js`
- Modify: `tests/domain/resource-provisioning.test.js`
- Modify: `tests/billing/prepaid-ledger-billing.test.js`
- Modify: `tests/e2e/local-business-chain.test.js`

- [ ] **Step 1: Import new pricing helpers**

Update imports in `resource-provisioning-service.js` and `billing-service.js`:

```js
import {
  billableHours,
  hourlyComputeAmount,
  hourlyStorageAmount,
  packageHoldAmount,
  pricedComputeHourly,
  pricedStorageGbMonth,
  providerCostEstimate,
  providerStorageCostEstimate,
  storageGbHourPrice,
  userComputeHourly,
  userStorageGbMonth
} from "./pricing-service.js";
```

- [ ] **Step 2: Write price snapshot helper**

Add near the top of `resource-provisioning-service.js` and mirror or import in `billing-service.js` if local style prefers no shared file:

```js
function computePriceSnapshot({ packagePlan, pricing }) {
  return {
    priceBasis: pricing.priceBasis || "opl_user_price_catalog",
    userPrice: {
      computeHourly: userComputeHourly({ packagePlan, pricing }),
      currency: pricing.currency || "CNY"
    },
    providerCostEstimate: providerCostEstimate({ packagePlan, pricing })
  };
}

function storagePriceSnapshot({ pricing, sizeGb }) {
  return {
    priceBasis: pricing.priceBasis || "opl_user_price_catalog",
    userPrice: {
      storageGbMonth: userStorageGbMonth(pricing),
      storageGbHour: storageGbHourPrice(pricing),
      sizeGb,
      currency: pricing.currency || "CNY"
    },
    providerCostEstimate: providerStorageCostEstimate(pricing)
  };
}
```

- [ ] **Step 3: Update hold ledger metadata**

In compute hold metadata, replace `baseHourly` and `markup` with:

```js
metadata: {
  computeAllocationId: allocationId,
  ownerAccountId: accountId,
  ownerUserId: account.id,
  packageId,
  holdDays: 7,
  ...computePriceSnapshot({ packagePlan, pricing: this.pricing })
}
```

In storage hold metadata, replace `baseGbMonth` and `markup` with:

```js
metadata: {
  storageId,
  ownerAccountId: accountId,
  ownerUserId: account.id,
  packageId,
  sizeGb: normalizedSizeGb,
  holdDays: 7,
  ...storagePriceSnapshot({ pricing: this.pricing, sizeGb: normalizedSizeGb })
}
```

- [ ] **Step 4: Update debit ledger metadata**

In `debitComputeAllocationUsage`, replace metadata with:

```js
const metadata = {
  computeAllocationId: compute.id,
  ownerAccountId: compute.ownerAccountId,
  ownerUserId: compute.ownerUserId || account.id,
  workspaceIds: compute.workspaceIds || [],
  nodePoolId: compute.nodePoolId || "",
  cvmInstanceId: compute.cvmInstanceId || compute.instanceId || "",
  machineName: compute.machineName || "",
  nodeName: compute.nodeName || "",
  privateIp: compute.privateIp || "",
  publicIp: compute.publicIp || "",
  requestedHours: hours,
  balanceBefore,
  frozenBefore,
  ...computePriceSnapshot({ packagePlan, pricing: this.pricing })
};
```

In `debitStorageResourceUsage`, replace metadata with:

```js
const metadata = {
  storageId: storage.id,
  ownerAccountId: storage.ownerAccountId,
  ownerUserId: storage.ownerUserId || account.id,
  workspaceIds: storage.workspaceIds || [],
  providerResourceId: storage.providerResourceId || "",
  storageClassId: storage.storageClassId || "",
  requestedHours: hours,
  sizeGb: storage.sizeGb,
  balanceBefore,
  frozenBefore,
  ...storagePriceSnapshot({ pricing: this.pricing, sizeGb: storage.sizeGb })
};
```

- [ ] **Step 5: Add tests for traceability**

In `tests/domain/resource-provisioning.test.js`, add assertions to the first full provisioning test:

```js
const computeHold = state.billingLedger.find((entry) => entry.type === "compute_hold" && entry.computeAllocationId === readyCompute.id);
assert.equal(computeHold.metadata.ownerAccountId, "pi-alpha");
assert.equal(computeHold.metadata.priceBasis, "opl_user_price_catalog");
assert.equal(computeHold.metadata.userPrice.computeHourly, 1);
assert.equal(computeHold.metadata.providerCostEstimate.billingUse, "internal_reconciliation_only");

const storageHold = state.billingLedger.find((entry) => entry.type === "storage_hold" && entry.storageId === storage.id);
assert.equal(storageHold.metadata.ownerAccountId, "pi-alpha");
assert.equal(storageHold.metadata.userPrice.storageGbMonth, 0.2);
assert.equal(storageHold.metadata.userPrice.sizeGb, 20);
```

- [ ] **Step 6: Run resource billing tests**

Run:

```bash
npm test -- tests/domain/resource-provisioning.test.js tests/billing/prepaid-ledger-billing.test.js tests/e2e/local-business-chain.test.js
```

Expected: PASS.

- [ ] **Step 7: Commit ledger evidence**

Run:

```bash
git add packages/console/src/services/resource-provisioning-service.js packages/console/src/services/billing-service.js packages/console/src/services/ledger-evidence-service.js tests/domain/resource-provisioning.test.js tests/billing/prepaid-ledger-billing.test.js tests/e2e/local-business-chain.test.js
git commit -m "feat: snapshot resource price and owner evidence"
```

---

### Task 5: Remove OPL Cloud Request-Billing Narrative

**Files:**
- Modify: `packages/contracts/opl-cloud-route-api-contract.json`
- Modify: `packages/contracts/opl-cloud-billing-ledger-contract.json`
- Modify: `packages/console/ui/pages/billing/BillingPage.jsx`
- Modify: `packages/console/src/services/billing-service.js`
- Modify: `packages/console/src/store.js`
- Modify: `tests/ui/commercial-console-surface.test.js`
- Modify: `tests/persistence/postgres-store.test.js`
- Modify: `tests/README.md`

- [ ] **Step 1: Change UI contract tests from request billing to resource billing**

In `tests/ui/commercial-console-surface.test.js`, replace:

```js
assert.match(billingSource, /request_debit|请求扣费/, "billing page must expose request debit evidence");
```

with:

```js
assert.doesNotMatch(billingSource, /request_debit|请求扣费|token|tokens|model pricing/i, "OPL Cloud billing must not expose request-level charging");
assert.match(billingSource, /compute_debit|storage_debit|资源扣费|存储扣费/, "billing page must expose resource debit evidence");
```

- [ ] **Step 2: Remove request debit from hot ledger index expectation**

In `tests/persistence/postgres-store.test.js`, replace:

```js
assert.match(indexStatement, /state->>'type'\) IN \('compute_debit', 'storage_debit', 'compute_hold_exhausted', 'request_debit'\)/);
```

with:

```js
assert.match(indexStatement, /state->>'type'\) IN \('compute_debit', 'storage_debit', 'compute_hold_exhausted'\)/);
assert.doesNotMatch(indexStatement, /request_debit/);
```

- [ ] **Step 3: Remove OPL Cloud request usage API from read model**

If `recordRequestUsage` is still exported from `packages/console/src/opl-cloud.js`, remove that public method. Keep low-level stored historical arrays only if tests require migration compatibility; do not expose them in current Admin/Billing UI.

Remove this method:

```js
async recordRequestUsage(...args) {
  return this.billing.recordRequestUsage(...args);
}
```

Do not remove one-person-lab-cloud request accounting from other repositories.

- [ ] **Step 4: Update billing page copy and data sources**

In `BillingPage.jsx`, remove visible strings and table filters related to:

```text
request_debit
请求扣费
token
model pricing
```

Keep visible resource evidence:

```text
compute_hold
storage_hold
compute_debit
storage_debit
compute_destroyed
storage_destroyed
hold_release
```

- [ ] **Step 5: Run UI/persistence tests**

Run:

```bash
npm test -- tests/ui/commercial-console-surface.test.js tests/persistence/postgres-store.test.js tests/contracts/route-api-contract.test.js
```

Expected: PASS.

- [ ] **Step 6: Commit narrative cleanup**

Run:

```bash
git add packages/contracts/opl-cloud-route-api-contract.json packages/contracts/opl-cloud-billing-ledger-contract.json packages/console/ui/pages/billing/BillingPage.jsx packages/console/src/services/billing-service.js packages/console/src/store.js tests/ui/commercial-console-surface.test.js tests/persistence/postgres-store.test.js tests/README.md
git commit -m "refactor: keep OPL billing resource-only"
```

---

### Task 6: Admin And Console Resource Ledger Evidence

**Files:**
- Modify: `packages/console/src/services/console-read-model-service.js`
- Modify: `packages/console/api/server.js` or current route file that serves Admin summaries
- Modify: `packages/console/ui/pages/resources/ResourceProvisioningPages.jsx`
- Modify: `packages/console/ui/pages/billing/BillingPage.jsx`
- Modify: `packages/console/ui/pages/admin/AdminPages.jsx` or current admin user/resource page
- Modify: `tests/management/management-model.test.js`
- Modify: `tests/ui/commercial-console-surface.test.js`
- Modify: `tests/contracts/route-api-contract.test.js`

- [ ] **Step 1: Add read-model resource ledger summary**

In `console-read-model-service.js`, add a helper:

```js
function resourceLedgerEvidence(state, accountId) {
  const ledger = state.billingLedger || [];
  const walletTransactions = state.walletTransactions || [];
  return {
    computeAllocations: (state.computeAllocations || [])
      .filter((item) => !accountId || item.ownerAccountId === accountId)
      .map((compute) => ({
        id: compute.id,
        ownerAccountId: compute.ownerAccountId,
        ownerUserId: compute.ownerUserId || "",
        packageId: compute.packageId,
        spec: compute.spec,
        nodePoolId: compute.nodePoolId || "",
        cvmInstanceId: compute.cvmInstanceId || compute.instanceId || "",
        machineName: compute.machineName || "",
        nodeName: compute.nodeName || "",
        privateIp: compute.privateIp || "",
        publicIp: compute.publicIp || "",
        billingStatus: compute.billingStatus,
        hourlyPrice: compute.hourlyPrice,
        holdAmount: compute.holdAmount,
        workspaceIds: compute.workspaceIds || [],
        ledgerEntryIds: ledger
          .filter((entry) => entry.computeAllocationId === compute.id)
          .map((entry) => entry.id),
        walletTransactionIds: walletTransactions
          .filter((entry) => entry.metadata?.computeAllocationId === compute.id)
          .map((entry) => entry.id)
      })),
    storageVolumes: (state.storageVolumes || [])
      .filter((item) => !accountId || item.ownerAccountId === accountId)
      .map((storage) => ({
        id: storage.id,
        ownerAccountId: storage.ownerAccountId,
        ownerUserId: storage.ownerUserId || "",
        packageId: storage.packageId,
        providerResourceId: storage.providerResourceId || "",
        storageClassId: storage.storageClassId || "",
        sizeGb: storage.sizeGb,
        billingStatus: storage.billingStatus,
        gbMonthPrice: storage.gbMonthPrice,
        hourlyEstimate: storage.hourlyEstimate,
        holdAmount: storage.holdAmount,
        workspaceIds: storage.workspaceIds || [],
        ledgerEntryIds: ledger
          .filter((entry) => entry.storageId === storage.id)
          .map((entry) => entry.id),
        walletTransactionIds: walletTransactions
          .filter((entry) => entry.metadata?.storageId === storage.id)
          .map((entry) => entry.id)
      }))
  };
}
```

Include this object in account Admin summaries as `resourceLedgerEvidence`.

- [ ] **Step 2: Add Admin test**

In `tests/management/management-model.test.js`, add:

```js
test("admin summary exposes owner-bound resource ledger evidence", async () => {
  const service = createManagementService();
  await service.manualTopUp({ accountId: "pi-alpha", amount: 300, reason: "owner_credit" });
  const compute = await service.createComputeAllocation({ accountId: "pi-alpha", packageId: "basic" });
  await service.processPendingResourceProvisioning({ limit: 1 });
  const storage = await service.createStorageVolume({ accountId: "pi-alpha", packageId: "basic", sizeGb: 10 });

  const summary = await service.adminAccountSummary({ accountId: "pi-alpha" });
  assert.equal(summary.resourceLedgerEvidence.computeAllocations[0].id, compute.id);
  assert.equal(summary.resourceLedgerEvidence.computeAllocations[0].ownerAccountId, "pi-alpha");
  assert.ok(summary.resourceLedgerEvidence.computeAllocations[0].ledgerEntryIds.length > 0);
  assert.equal(summary.resourceLedgerEvidence.storageVolumes[0].id, storage.id);
  assert.equal(summary.resourceLedgerEvidence.storageVolumes[0].ownerAccountId, "pi-alpha");
});
```

Use the existing management service factory names in that file; if the local helper is named differently, use the existing helper but keep the assertions unchanged.

- [ ] **Step 3: Update UI tests**

In `tests/ui/commercial-console-surface.test.js`, assert Admin and resource pages expose these strings or field names:

```js
for (const signal of [
  "ownerAccountId",
  "cvmInstanceId",
  "nodeName",
  "storageId",
  "workspaceIds",
  "ledgerEntryIds",
  "walletTransactionIds"
]) {
  assert.match(adminSource + resourceSource + billingSource, new RegExp(signal), `Console/Admin must expose ${signal}`);
}
```

- [ ] **Step 4: Implement UI surfaces**

Add columns/descriptions in Resource and Admin pages:

```js
{ title: "Owner", dataIndex: "ownerAccountId", ellipsis: true }
{ title: "CVM ID", dataIndex: "cvmInstanceId", ellipsis: true }
{ title: "Node", dataIndex: "nodeName", ellipsis: true }
{ title: "Workspaces", dataIndex: "workspaceIds", render: (items = []) => items.join(", ") || "-" }
{ title: "Ledger", dataIndex: "ledgerEntryIds", render: (items = []) => String(items.length || 0) }
```

Do not show provider cost estimates in normal user-facing copy unless the page is Admin/diagnostic.

- [ ] **Step 5: Run Admin/UI tests**

Run:

```bash
npm test -- tests/management/management-model.test.js tests/ui/commercial-console-surface.test.js tests/contracts/route-api-contract.test.js
```

Expected: PASS.

- [ ] **Step 6: Commit Admin evidence**

Run:

```bash
git add packages/console/src/services/console-read-model-service.js packages/console/api/server.js packages/console/ui/pages/resources/ResourceProvisioningPages.jsx packages/console/ui/pages/billing/BillingPage.jsx packages/console/ui/pages/admin/AdminPages.jsx tests/management/management-model.test.js tests/ui/commercial-console-surface.test.js tests/contracts/route-api-contract.test.js
git commit -m "feat: expose resource ledger evidence to admin"
```

If the Admin page path is different, substitute the actual file returned by:

```bash
rg -n "Admin|adminAccountSummary|resourceLedgerEvidence" packages/console -S
```

---

### Task 7: Resource Usage Aggregation And Archive

**Files:**
- Modify: `packages/console/src/store.js`
- Modify: `packages/console/src/services/billing-service.js`
- Modify: `packages/console/src/services/console-read-model-service.js`
- Modify: `tests/persistence/postgres-store.test.js`
- Modify: `tests/billing/prepaid-ledger-billing.test.js`

- [ ] **Step 1: Add collections to store defaults**

In `packages/console/src/store.js`, add default arrays:

```js
resourceUsageHourly: [],
resourceUsageDaily: [],
resourceUsageArchive: [],
resourceUsageCleanupTasks: []
```

Persist them in the same JSONB table pattern used by `resourceUsageLogs`.

- [ ] **Step 2: Add hourly aggregation method**

In `billing-service.js`, add:

```js
async aggregateResourceUsage({ olderThan = now(), sourceEventId = "" } = {}) {
  return this.store.update((state) => {
    ensureBillingCollections(state);
    state.resourceUsageHourly ??= [];
    const cutoff = new Date(olderThan).getTime();
    const candidates = (state.resourceUsageLogs || []).filter((log) => new Date(log.createdAt).getTime() <= cutoff);
    const buckets = new Map();

    for (const log of candidates) {
      const created = new Date(log.createdAt);
      created.setMinutes(0, 0, 0);
      const key = [
        created.toISOString(),
        log.accountId,
        log.resourceType,
        log.computeAllocationId || "",
        log.storageId || ""
      ].join("|");
      const current = buckets.get(key) || {
        bucketStart: created.toISOString(),
        accountId: log.accountId,
        resourceType: log.resourceType,
        computeAllocationId: log.computeAllocationId || "",
        storageId: log.storageId || "",
        quantity: 0,
        amount: 0,
        requestedAmount: 0,
        currency: "CNY",
        sourceEventIds: []
      };
      current.quantity = money(current.quantity + Number(log.quantity || 0));
      current.amount = money(current.amount + Number(log.amount || 0));
      current.requestedAmount = money(current.requestedAmount + Number(log.requestedAmount || 0));
      current.sourceEventIds.push(log.sourceEventId);
      buckets.set(key, current);
    }

    for (const aggregate of buckets.values()) {
      const exists = state.resourceUsageHourly.some((row) =>
        row.bucketStart === aggregate.bucketStart &&
        row.accountId === aggregate.accountId &&
        row.resourceType === aggregate.resourceType &&
        row.computeAllocationId === aggregate.computeAllocationId &&
        row.storageId === aggregate.storageId
      );
      if (!exists) {
        state.resourceUsageHourly.push({
          id: makeId("resource-usage-hourly", aggregate.accountId, aggregate.resourceType, aggregate.bucketStart, aggregate.computeAllocationId || aggregate.storageId || "resource"),
          ...aggregate,
          createdAt: now()
        });
      }
    }

    state.audit.push(this.auditEvent({
      accountId: "all",
      workspaceId: "resource",
      type: "billing.resource_usage_aggregated",
      sourceEventId: sourceEventId || `resource_usage_aggregate:${now()}`
    }));

    return { aggregated: buckets.size };
  });
}
```

- [ ] **Step 3: Add archive method**

In `billing-service.js`, add:

```js
async archiveResourceUsageLogs({ olderThan, limit = 1000, sourceEventId = "" }) {
  if (!olderThan) throw new Error("older_than_required");
  return this.store.update((state) => {
    ensureBillingCollections(state);
    state.resourceUsageArchive ??= [];
    state.resourceUsageCleanupTasks ??= [];
    const cutoff = new Date(olderThan).getTime();
    const archive = [];
    const keep = [];

    for (const log of state.resourceUsageLogs || []) {
      if (archive.length < limit && new Date(log.createdAt).getTime() < cutoff) archive.push(log);
      else keep.push(log);
    }

    state.resourceUsageArchive.push(...archive.map((log) => ({
      ...clone(log),
      archivedAt: now()
    })));
    state.resourceUsageLogs = keep;
    state.resourceUsageCleanupTasks.push({
      id: makeId("resource-usage-cleanup", sourceEventId || String(Date.now()), String(state.resourceUsageCleanupTasks.length)),
      status: "succeeded",
      olderThan,
      archivedRows: archive.length,
      createdAt: now(),
      finishedAt: now()
    });

    return { archivedRows: archive.length, remainingRows: keep.length };
  });
}
```

- [ ] **Step 4: Add aggregation/archive tests**

In `tests/billing/prepaid-ledger-billing.test.js`, add:

```js
test("resource usage logs can be aggregated and archived without deleting ledger evidence", async () => {
  const service = createService();
  await service.manualTopUp({ accountId: "pi-alpha", amount: 300, reason: "owner_credit" });
  const compute = await service.createComputeAllocation({ accountId: "pi-alpha", packageId: "basic" });
  await service.processPendingResourceProvisioning({ limit: 1 });
  await service.settleResourceBilling({ accountId: "pi-alpha", hours: 1, sourceEventId: "tick-archive" });

  const aggregate = await service.aggregateResourceUsage({ olderThan: new Date(Date.now() + 1000).toISOString(), sourceEventId: "aggregate-test" });
  assert.ok(aggregate.aggregated >= 1);

  const archive = await service.archiveResourceUsageLogs({ olderThan: new Date(Date.now() + 1000).toISOString(), limit: 100, sourceEventId: "archive-test" });
  assert.ok(archive.archivedRows >= 1);

  const state = await service.getState("pi-alpha");
  assert.equal(state.billingLedger.some((entry) => entry.computeAllocationId === compute.id), true);
  assert.equal(state.resourceUsageHourly.some((entry) => entry.computeAllocationId === compute.id), true);
  assert.equal(state.resourceUsageArchive.some((entry) => entry.computeAllocationId === compute.id), true);
});
```

- [ ] **Step 5: Expose service methods from `opl-cloud.js`**

Add:

```js
async aggregateResourceUsage(...args) {
  return this.billing.aggregateResourceUsage(...args);
}

async archiveResourceUsageLogs(...args) {
  return this.billing.archiveResourceUsageLogs(...args);
}
```

- [ ] **Step 6: Run persistence and billing tests**

Run:

```bash
npm test -- tests/billing/prepaid-ledger-billing.test.js tests/persistence/postgres-store.test.js
```

Expected: PASS.

- [ ] **Step 7: Commit aggregation/archive**

Run:

```bash
git add packages/console/src/store.js packages/console/src/services/billing-service.js packages/console/src/services/console-read-model-service.js packages/console/src/opl-cloud.js tests/billing/prepaid-ledger-billing.test.js tests/persistence/postgres-store.test.js
git commit -m "feat: aggregate and archive resource usage logs"
```

---

### Task 8: Full Verification, Merge, Push, Rollout

**Files:**
- No direct code edits unless tests expose a bug from previous tasks.

- [ ] **Step 1: Run full local validation**

Run:

```bash
npm test
npm run build
go test ./cmd/opl-tencent-provisioner
```

Expected:

```text
# npm test: all tests pass
# npm run build: Vite build completes
# go test: ok ./cmd/opl-tencent-provisioner
```

- [ ] **Step 2: Inspect diff**

Run:

```bash
git status --short
git log --oneline --decorate -5
git diff --stat main...HEAD
```

Expected: only pricing contracts, ledger/pricing services, Console/Admin billing UI, store persistence, tests, env/workflow defaults.

- [ ] **Step 3: Merge back to main**

Run from the main worktree:

```bash
cd /home/dev/medopl-3
git status --short --branch
git merge --no-ff ledger-pricing-hardening -m "merge: harden OPL resource ledger pricing"
```

If main has user-owned dirty files, do not stash or overwrite them. Stop and report the exact conflicting paths.

- [ ] **Step 4: Re-run validation on main**

Run:

```bash
npm test
npm run build
go test ./cmd/opl-tencent-provisioner
```

Expected: all pass.

- [ ] **Step 5: Push and rollout**

Run:

```bash
git push origin main
gh workflow run "Release OPL Cloud Image" --ref main
```

Wait for release success and capture the image tag from the run logs. Then run:

```bash
gh workflow run "Deploy TKE Production" --ref main -f cloud_image="uswccr.ccs.tencentyun.com/oplcloud/opl-cloud:<tag>" -f workspace_image="uswccr.ccs.tencentyun.com/oplcloud/one-person-lab-app:latest"
```

Then run:

```bash
gh workflow run "Verify Production Chain" --ref main
```

Expected: Release, Deploy, and Verify Production Chain succeed. Do not rerun verifier blindly if it fails; read logs and stop after repeated failure.

- [ ] **Step 6: Cleanup feature worktree**

Run:

```bash
git worktree remove /home/dev/.config/superpowers/worktrees/medopl-3/ledger-pricing-hardening
git branch -d ledger-pricing-hardening
```

Do not remove `/home/dev/.config/superpowers/worktrees/medopl-3/workspace-nav-simplification`.

---

## Self-Review

**Spec coverage:** The plan covers final OPL user pricing, provider cost as internal evidence, traceable holds/debits/releases by owner/resource/workspace, Admin/Console evidence, and resource usage aggregation/archive. It intentionally excludes request-level billing from OPL Cloud.

**Placeholder scan:** No implementation step depends on "TBD", "TODO", or unspecified error handling. Where local Admin file paths may vary, the plan gives the exact `rg` command and required assertions.

**Type consistency:** Price snapshot fields are consistently `priceBasis`, `userPrice`, and `providerCostEstimate`. Resource identity fields are consistently `ownerAccountId`, `ownerUserId`, `computeAllocationId`, `storageId`, `workspaceIds`, `cvmInstanceId`, `nodeName`, and `ledgerEntryIds`.
