import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../../", import.meta.url);

async function text(path) {
  return readFile(new URL(path, root), "utf8");
}

async function json(path) {
  return JSON.parse(await text(path));
}

test("root agent instructions require the launch invariants", async () => {
  const [agents, gitignore] = await Promise.all([text("AGENTS.md"), text(".gitignore")]);

  assert.match(agents, /docs\/invariants\.md/);
  assert.match(agents, /Before changing billing, Fabric, Workspace, Gateway, Ledger, deployment, or E2E/i);
  assert.match(gitignore, /^\.codegraph\/$/m);
});

test("launch freeze fixes the four owners, products, settlement, and verification slot", async () => {
  const freeze = await json("packages/contracts/opl-cloud-launch-freeze-contract.json");

  assert.equal(freeze.architectureAuthority.repository, "https://github.com/gaofeng21cn/one-person-lab-cloud");
  assert.equal(freeze.architectureAuthority.reviewedRevision, "fdeb0e4df3e4905fb1c3551337b9dfda65bb2119");
  assert.deepEqual(Object.keys(freeze.layers), ["console", "fabric", "ledger", "gateway"]);
  assert.equal(freeze.layers.gateway.backend, "Sub2API");
  assert.equal(freeze.layers.gateway.spendableBalanceOwner, true);
  assert.equal(freeze.layers.console.localWalletForbidden, true);

  assert.deepEqual(freeze.customerProducts.basic.resources, { cpu: 2, memoryGb: 4 });
  assert.deepEqual(freeze.customerProducts.pro.resources, { cpu: 8, memoryGb: 16 });
  assert.equal(freeze.customerProducts.basic.currentAvailability, "available");
  assert.equal(freeze.customerProducts.pro.currentAvailability, "implementation_required");

  assert.deepEqual(freeze.monthlySettlement.protocol, ["reserve", "provision", "claim", "capture"]);
  assert.equal(freeze.monthlySettlement.failureBeforeProviderCost, "release");
  assert.equal(freeze.monthlySettlement.ambiguousProviderResult, "keep_reserved_and_manual_review");
  assert.equal(freeze.providerProcurement.chargeType, "PREPAID");
  assert.equal(freeze.providerProcurement.periodMonths, 1);
  assert.equal(freeze.providerProcurement.renewFlag, "NOTIFY_AND_MANUAL_RENEW");
  assert.deepEqual(freeze.providerProcurement.forbiddenChargeTypes, ["POSTPAID_BY_HOUR"]);

  assert.equal(freeze.verification.slot.computeInstanceType, "SA5.MEDIUM2");
  assert.equal(freeze.verification.slot.customerProduct, false);
  assert.equal(freeze.verification.slot.reuseForBillingPeriod, true);
  assert.equal(freeze.verification.perRunTencentPurchase, false);
  assert.equal(freeze.verification.monthlyBillingBackend, "fake");
  assert.equal(freeze.verification.gatewayRequest, "real_dedicated_test_key");
  assert.deepEqual(freeze.verification.providerResourcesDeletedPerRun, []);
});

test("every launch slide declares business, current state, deliverables, and evidence", async () => {
  const freeze = await json("packages/contracts/opl-cloud-launch-freeze-contract.json");
  const expected = [
    "offer_identity",
    "wallet_quote",
    "balance_reservation",
    "prepaid_fulfillment",
    "claim_and_capture",
    "workspace_access",
    "gateway_usage",
    "renewal_expiry_recovery",
    "reusable_verification",
    "production_release"
  ];

  assert.deepEqual(freeze.slides.map((slide) => slide.id), expected);
  for (const slide of freeze.slides) {
    assert.ok(slide.business, `${slide.id} business`);
    assert.ok(Array.isArray(slide.owners) && slide.owners.length > 0, `${slide.id} owners`);
    assert.ok(slide.currentState, `${slide.id} currentState`);
    assert.ok(Array.isArray(slide.requiredDeliverables) && slide.requiredDeliverables.length > 0, `${slide.id} requiredDeliverables`);
    assert.ok(Array.isArray(slide.completionEvidence) && slide.completionEvidence.length > 0, `${slide.id} completionEvidence`);
  }
});

test("human invariants reject paid per-run resource verification", async () => {
  const invariants = await text("docs/invariants.md");

  for (const heading of ["Console", "Fabric", "Ledger", "Gateway", "Launch Slides", "Verification Slot"]) {
    assert.match(invariants, new RegExp(`## ${heading}`));
  }
  assert.match(invariants, /SA5\.MEDIUM2/);
  assert.match(invariants, /reserve.*capture.*release/is);
  assert.match(invariants, /POSTPAID_BY_HOUR.*forbidden/is);
  assert.doesNotMatch(invariants, /Production E2E requires explicit confirmation that it spends real balance/);
  assert.doesNotMatch(invariants, /Fabric prepares before charge/);
});

test("legacy paid verifier is blocked and removed from the release gate", async () => {
  const deployment = await json("packages/contracts/opl-cloud-deployment-contract.json");
  const [architecture, decisions, project, readme, runbook, status] = await Promise.all([
    text("docs/architecture.md"),
    text("docs/decisions.md"),
    text("docs/project.md"),
    text("README.md"),
    text("docs/runtime/production-runbook.md"),
    text("docs/status.md")
  ]);

  assert.equal(deployment.productionVerificationWorkflow.launchStatus, "blocked");
  assert.equal(deployment.productionVerificationWorkflow.releaseGate, false);
  assert.equal(deployment.productionVerificationWorkflow.replacement, "reusable_prepaid_verification_slot");
  assert.match(runbook, /Do not run the legacy paid verifier/);
  assert.match(architecture, /reserve.*before\s+Fabric.*capture/is);
  assert.doesNotMatch(architecture, /Fabric preparation happens before the external charge/);
  for (const document of [architecture, decisions, project, readme, status]) {
    assert.doesNotMatch(document, /single paid verifier|one paid production verifier|explicitly confirmed paid E2E/i);
  }
});
