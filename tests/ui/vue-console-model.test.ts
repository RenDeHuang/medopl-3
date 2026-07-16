import assert from "node:assert/strict";
import test from "node:test";

import {
  customerMenu,
  fixedMonthlySpend,
  formatUsdMicros,
  gatewayCanCall,
  needsSession,
  resourceOrderStage,
  resourceNeedsAttention,
  resourceStatusLabel,
  renewalSummary,
  storageMonthlyPrice,
  workspaceMonthlyPrice,
  workspaceProgress
} from "../../apps/console-ui/src/console-model.ts";

test("customer navigation is the five product views", () => {
  assert.deepEqual(customerMenu.map(({ label, path }) => ({ label, path })), [
    { label: "概览", path: "/console/overview" },
    { label: "计算", path: "/console/compute" },
    { label: "存储", path: "/console/storage" },
    { label: "Gateway", path: "/console/gateway" },
    { label: "账单", path: "/console/billing" }
  ]);
});

test("public and login routes render without waiting for session recovery", () => {
  assert.equal(needsSession("/"), false);
  assert.equal(needsSession("/login"), false);
  assert.equal(needsSession("/console/overview"), true);
  assert.equal(needsSession("/admin"), true);
});

test("Workspace progress is derived from state resources only", () => {
  const state = {
    workspaces: [{
      id: "ws-1",
      currentComputeAllocationId: "compute-1",
      currentAttachmentId: "attachment-1",
      runtimeId: "runtime-1",
      openable: true
    }],
    computeAllocations: [{ id: "compute-1", status: "running" }],
    storageVolumes: [{ id: "storage-1", status: "available" }],
    storageAttachments: [{
      id: "attachment-1",
      computeAllocationId: "compute-1",
      storageId: "storage-1",
      status: "attached"
    }]
  };

  assert.deepEqual(workspaceProgress(state, state.workspaces[0]), [
    { label: "计算可用", complete: true },
    { label: "存储可用", complete: true },
    { label: "挂载完成", complete: true },
    { label: "Workspace 启动", complete: true },
    { label: "可打开", complete: true }
  ]);
  assert.equal(workspaceProgress({ ...state, storageVolumes: [] }, state.workspaces[0])[1].complete, false);
});

test("four-stage order progress never claims a stage without matching API state", () => {
  assert.equal(resourceOrderStage({ id: "compute-1", status: "provisioning", billingStatus: "preparing" }), 1);
  assert.equal(resourceOrderStage({ id: "compute-1", status: "provisioning", billingStatus: "provider_pending" }), 3);
  assert.equal(resourceOrderStage({ id: "compute-1", status: "running", billingStatus: "active" }), 4);
  assert.equal(resourceOrderStage({ id: "storage-1", status: "available", billingStatus: "active" }), 4);
});

test("billing review is not presented as ordinary provisioning", () => {
  assert.equal(resourceNeedsAttention({ status: "provisioning", billingStatus: "manual_review" }), true);
  assert.equal(resourceNeedsAttention({ status: "provisioning", billingStatus: "provider_pending" }), false);
  assert.equal(resourceStatusLabel({ status: "running", billingStatus: "manual_review" }), "需要处理");
});

test("USD values are formatted from integer micros", () => {
  assert.equal(formatUsdMicros(83_200_000), "$83.20");
  assert.equal(formatUsdMicros(undefined), "-");
  assert.equal(formatUsdMicros(null), "-");
  assert.equal(formatUsdMicros("83200000"), "-");
});

test("Gateway usability requires a live Key and positive spendable balance", () => {
  const activeKey = { apiKey: { status: "active" }, balance: { available: true, usdMicros: 1 } };
  assert.equal(gatewayCanCall(activeKey), true);
  assert.equal(gatewayCanCall({ ...activeKey, balance: { available: true, usdMicros: 0 } }), false);
  assert.equal(gatewayCanCall({ ...activeKey, apiKey: { status: "disabled" } }), false);
});

test("storage price uses the exact server preview without block rounding", () => {
  const quotes = {
    basic: { chargeUsdMicros: 2_571_429 },
    pro: { chargeUsdMicros: 25_714_286 }
  };
  assert.equal(storageMonthlyPrice(quotes, "basic"), 2_571_429);
  assert.equal(storageMonthlyPrice(quotes, "pro"), 25_714_286);
  assert.equal(storageMonthlyPrice({ basic: { chargeUsdMicros: null } }, "basic"), undefined);
});

test("missing monthly price facts stay unavailable instead of becoming zero", () => {
  assert.equal(fixedMonthlySpend([]), 0);
  assert.equal(fixedMonthlySpend([{ billingStatus: "active" }]), undefined);
  assert.equal(fixedMonthlySpend([{ billingStatus: "active", chargeUsdMicros: "50000000" }]), undefined);
  assert.equal(fixedMonthlySpend([
    { billingStatus: "active", chargeUsdMicros: 50_000_000 },
    { billingStatus: "active", chargeUsdMicros: 2_571_429 }
  ]), 52_571_429);

  const basic = { id: "basic", price: { chargeUsdMicros: 50_000_000 } };
  assert.equal(workspaceMonthlyPrice(basic, {}), undefined);
  assert.equal(workspaceMonthlyPrice(basic, { basic: { chargeUsdMicros: 2_571_429 } }), 52_571_429);
});

test("renewal summary does not invent a policy when no resources exist", () => {
  assert.equal(renewalSummary([]), "-");
  assert.equal(renewalSummary([{}]), "-");
  assert.equal(renewalSummary([{ autoRenew: false }]), "手动续费");
  assert.equal(renewalSummary([{ autoRenew: true }]), "自动续费");
});
