import test from "node:test";
import assert from "node:assert/strict";

import { basename, join } from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";

import {
  runProductionConsoleBrowserVerifierCli,
  verifyProductionConsoleLifecycle
} from "../../tools/production-console-browser-verifier.ts";

const runId = "console-run-7";
const slot = "03";
const accountId = "account-console-7";
const computeName = `Production Verification Lab compute ${runId}`;
const storageName = `Production Verification Lab storage ${runId}`;
const workspaceName = `Production Verification Lab ${runId}`;
const workspaceUrl = "https://workspace.medopl.cn/w/ws-console-7/";
const workspaceOpenUrl = `${workspaceUrl}?token=share-console-secret`;

async function testTempDir(t) {
  const path = await mkdtemp(join(tmpdir(), "opl-console-browser-test-"));
  t.after(() => rm(path, { recursive: true, force: true }));
  return path;
}

function stateFixture() {
  return {
    account: { accountId },
    wallet: { accountId, balance: 1000, balanceCents: 100000, frozen: 5, frozenCents: 500, available: 995, availableCents: 99500 },
    computeAllocations: [],
    storageVolumes: [],
    storageAttachments: [],
    workspaces: [],
    billingLedger: [],
    billingSummary: { activeHourlyEstimate: 0, recentResourceDebitTotal: 0 }
  };
}

function cleanupOwnershipState() {
  return {
    account: { accountId },
    wallet: { accountId },
    computeAllocations: [{
      id: "compute-7", accountId, name: computeName, holdId: "hold-compute-7",
      machineName: "machine-7", instanceId: "ins-7", nodeName: "node-7"
    }],
    storageVolumes: [{ id: "storage-7", accountId, name: storageName, holdId: "hold-storage-7" }],
    storageAttachments: [{
      id: "attachment-7", accountId, computeAllocationId: "compute-7", storageId: "storage-7"
    }],
    workspaces: [{
      id: "ws-console-7", accountId, name: workspaceName, computeAllocationId: "compute-7",
      storageId: "storage-7", attachmentId: "attachment-7"
    }]
  };
}

function jsonResponse(payload) {
  return {
    ok: true,
    status: 200,
    headers: { get: (name) => name.toLowerCase() === "content-type" ? "application/json" : "" },
    async json() { return structuredClone(payload); },
    async text() { return JSON.stringify(payload); }
  };
}

function fakeConsoleBrowserFactory({
  stages, screenshots, mutations, visibleTexts = [], readiness = true,
  delayCreateProjection = false, partialIdentityProjection = false, replayStages = [], duplicateMachineIdentity = false,
  failCookieRefresh = false, cookieReads = [], closeEvents = [], contextCloseError = "", browserCloseError = "", failAt = "",
  nestedConsoleRoute = false
}) {
  const state = stateFixture();
  let routeHandler = null;
  let path = "/login";
  let pendingAction = "";
  let popupResolve = null;
  let delayNextState = false;
  let delayedStage = "";

  const mutationFor = {
    "开通计算": ["POST", "/api/compute-allocations", "create-compute", () => {
      state.computeAllocations = [{
        id: "compute-7", accountId, ownerAccountId: accountId, name: computeName, packageId: "basic",
        provider: "tencent-tke", status: "running", billingStatus: "active", holdId: "hold-compute-7", holdAmountCents: 16800,
        ledgerEntryId: "bill-compute", machineName: "machine-7", instanceId: "ins-7", nodeName: "node-7", hourlyPrice: 1
      }];
      if (duplicateMachineIdentity) state.computeAllocations.push({
        ...state.computeAllocations[0], id: "compute-duplicate", name: "unrelated compute"
      });
      state.wallet.balance = 999;
      state.wallet.balanceCents = 99900;
      state.wallet.frozen = 173;
      state.wallet.frozenCents = 17300;
      state.wallet.available = 826;
      state.wallet.availableCents = 82600;
      state.billingSummary.activeHourlyEstimate = 1;
    }],
    "开通存储": ["POST", "/api/storage-volumes", "create-storage", () => {
      state.storageVolumes = [{
        id: "storage-7", accountId, ownerAccountId: accountId, name: storageName, packageId: "basic",
        provider: "tencent-tke", providerResourceId: "pvc/storage-7", sizeGb: 10,
        status: "available", billingStatus: "active", holdId: "hold-storage-7", holdAmountCents: 1680,
        ledgerEntryId: "bill-storage", hourlyEstimate: 0.1
      }];
      state.wallet.balance = 998.9;
      state.wallet.balanceCents = 99890;
      state.wallet.frozen = 189.8;
      state.wallet.frozenCents = 18980;
      state.wallet.available = 809.1;
      state.wallet.availableCents = 80910;
      state.billingSummary.activeHourlyEstimate = 1.1;
    }],
    "挂载存储": ["POST", "/api/storage-attachments", "create-attachment", () => {
      state.storageAttachments = [{
        id: "attachment-7", accountId, ownerAccountId: accountId, computeAllocationId: "compute-7",
        storageId: "storage-7", mountPath: "/data", provider: "tencent-tke", status: "attached"
      }];
      state.storageVolumes[0].status = "attached";
    }],
    "创建工作区入口": ["POST", "/api/workspaces", "create-workspace", () => {
      state.workspaces = [{
        id: "ws-console-7", accountId, ownerAccountId: accountId, name: workspaceName,
        computeAllocationId: "compute-7", currentComputeAllocationId: "compute-7", storageId: "storage-7",
        attachmentId: "attachment-7", currentAttachmentId: "attachment-7", provider: "tencent-tke",
        state: "running", openable: true, accessState: "available", url: workspaceOpenUrl,
        access: { tokenStatus: "active", password: "hidden" },
        billing: { currentChargeTotal: 1.1, activeHourlyEstimate: 1.1 }
      }];
      state.billingLedger = [
        { id: "bill-compute", type: "compute_activation", source: "compute_activation", reason: "compute-7", amountCents: -100 },
        { id: "bill-storage", type: "storage_activation", source: "storage_activation", reason: "storage-7", amountCents: -10 }
      ];
      state.billingSummary.recentResourceDebitTotal = 1.1;
    }],
    "解除挂载": ["POST", "/api/storage-attachments/detach", "detach-storage", () => {
      state.storageAttachments[0].status = "detached";
      state.storageVolumes[0].status = "available";
    }],
    "销毁计算资源": ["POST", "/api/compute-allocations/compute-7/destroy", "destroy-compute", () => {
      Object.assign(state.computeAllocations[0], { status: "destroyed", billingStatus: "stopped", holdReleaseId: "release-compute-7" });
      state.wallet.frozen = 16.8;
      state.wallet.frozenCents = 2180;
      state.wallet.available = 977.1;
      state.wallet.availableCents = 97710;
      state.billingSummary.activeHourlyEstimate = 0.1;
    }],
    "销毁存储资源": ["POST", "/api/storage-volumes/destroy", "destroy-storage", () => {
      Object.assign(state.storageVolumes[0], { status: "destroyed", billingStatus: "stopped", holdReleaseId: "release-storage-7" });
      state.wallet.frozen = 5;
      state.wallet.frozenCents = 500;
      state.wallet.available = 993.9;
      state.wallet.availableCents = 99390;
      state.billingSummary.activeHourlyEstimate = 0;
    }]
  };

  async function dispatchMutation(label) {
    const [method, requestPath, stage, apply] = mutationFor[label];
    const headers = { "content-type": "application/json", "x-opl-csrf": "csrf-console-7" };
    const request = {
      method: () => method,
      url: () => `https://cloud.medopl.cn${requestPath}`,
      headers: () => ({ ...headers })
    };
    const route = () => ({
      async continue(options = {}) {
        mutations.push({ origin: "ui", method, path: requestPath, headers: options.headers || headers });
      }
    });
    await routeHandler(route(), request);
    if (replayStages.includes(stage)) await routeHandler(route(), request);
    apply();
    if ((delayCreateProjection || partialIdentityProjection) && stage.startsWith("create-")) {
      delayNextState = true;
      delayedStage = stage;
    }
  }

  class FakeLocator {
    constructor(name, scope = "") {
      this.name = name;
      this.scope = scope;
    }
    filter() { return this; }
    first() { return this; }
    nth() { return this; }
    async count() { return 1; }
    async isVisible() { return true; }
    async isEnabled() { return true; }
    async waitFor() {}
    async fill() {}
    async selectOption() {}
    getByRole(_role, options = {}) { return new FakeLocator(options.name, this.scope); }
    async click() {
      const label = String(this.name || "");
      if (label === "登录") {
        path = nestedConsoleRoute ? "/console/overview" : "/console";
        return;
      }
      if (label === "确认") {
        const action = pendingAction;
        pendingAction = "";
        await dispatchMutation(action);
        return;
      }
      if (label === "打开" && this.scope.includes("desktopWorkspaceTable")) {
        popupResolve?.({
          url: () => workspaceOpenUrl,
          async waitForURL() {},
          context: () => ({ async cookies() { return [{ name: "opl_ws_active", value: "ws-console-7" }]; } })
        });
        popupResolve = null;
        return;
      }
      if (mutationFor[label]) {
        if (label === "创建工作区入口") await dispatchMutation(label);
        else pendingAction = label;
      }
    }
  }

  const page = {
    async route(_pattern, handler) { routeHandler = handler; },
    async goto(url) { path = new URL(url).pathname; },
    async waitForURL(matcher) {
      const url = new URL(`https://cloud.medopl.cn${path}`);
      if (typeof matcher === "function") {
        if (!matcher(url)) throw new Error("wait_for_url_timeout");
        return;
      }
      if (matcher === "**/console*" && path.startsWith("/console/")) throw new Error("wait_for_url_timeout");
    },
    url() { return `https://cloud.medopl.cn${path}`; },
    getByLabel(name) { return new FakeLocator(name); },
    getByRole(_role, options = {}) { return new FakeLocator(options.name); },
    getByText(name) { visibleTexts.push({ name, path }); return new FakeLocator(name); },
    locator(selector) { return new FakeLocator(selector, selector); },
    async evaluate(_fn, requestPath) {
      if (["/api/production/readiness", "/api/runtime/readiness"].includes(requestPath)) return { ready: readiness };
      assert.match(requestPath, /^\/api\/state\?accountId=/);
      if (delayNextState) {
        delayNextState = false;
        if (partialIdentityProjection && ["create-compute", "create-storage"].includes(delayedStage)) {
          const partial = structuredClone(state);
          const row = delayedStage === "create-compute" ? partial.computeAllocations[0] : partial.storageVolumes[0];
          delete row.holdId;
          delete row.ledgerEntryId;
          if (delayedStage === "create-compute") {
            delete row.machineName;
            delete row.instanceId;
            delete row.nodeName;
            row.status = "provisioning";
            row.billingStatus = "pending";
          } else {
            row.status = "creating";
            row.billingStatus = "pending";
          }
          return partial;
        }
        return stateFixture();
      }
      return structuredClone(state);
    },
    async waitForEvent(event) {
      assert.equal(event, "popup");
      return new Promise((resolve) => { popupResolve = resolve; });
    },
    async screenshot({ path: screenshotPath }) {
      screenshots.push({ file: basename(screenshotPath), path });
      if (failAt && basename(screenshotPath).includes(failAt)) throw new Error(`screenshot_failed:${failAt}`);
    }
  };
  const context = {
    async newPage() { return page; },
    async cookies() {
      context.cookieReads = (context.cookieReads || 0) + 1;
      cookieReads.push(context.cookieReads);
      if (context.failCookieRefresh && context.cookieReads > 1) throw new Error("context_cookie_read_failed");
      return [{ name: "opl_session", value: "session-console-7" }];
    },
    failCookieRefresh,
    async close() {
      closeEvents.push("context");
      if (contextCloseError) throw new Error(contextCloseError);
    }
  };
  return async () => ({
    async newContext() { return context; },
    async close() {
      closeEvents.push("browser");
      if (browserCloseError) throw new Error(browserCloseError);
    }
  });
}

test("production Console drives the paid lifecycle and returns the opened Workspace URL", async (t) => {
  const artifactDir = await testTempDir(t);
  const stages = [];
  const screenshots = [];
  const mutations = [];
  const visibleTexts = [];
  const apiWrites = [];
  let workspaceVerified = "";
  let workspaceCookie = "";
  const result = await verifyProductionConsoleLifecycle({
    origin: "https://cloud.medopl.cn",
    accountId,
    ownerEmail: "owner@example.com",
    ownerPassword: "owner-password",
    runId,
    slot,
    packageId: "basic",
    attempts: 2,
    retryDelayMs: 0,
    manifestPath: join(artifactDir, "manifest.json"),
    screenshotDir: join(artifactDir, "screenshots"),
    browserFactory: fakeConsoleBrowserFactory({
      stages, screenshots, mutations, visibleTexts, delayCreateProjection: true, replayStages: ["create-compute"],
      nestedConsoleRoute: true
    }),
    workspaceVerifier: async ({ workspaceUrl: url, workspaceAuth }) => {
      workspaceVerified = url;
      workspaceCookie = workspaceAuth.cookie;
    },
    fetchImpl: async (_url, options = {}) => {
      if ((options.method || "GET") !== "GET") apiWrites.push(options);
      throw new Error("unexpected_api_request");
    }
  });

  assert.deepEqual(result.stages, [
    "login", "compute_hold_preview", "compute_running", "storage_hold_preview", "storage_available",
    "attached", "workspace_url_opened", "workspace_reply", "billing_visible", "detached",
    "compute_destroyed", "storage_destroyed"
  ]);
  assert.equal(result.workspaceId, "ws-console-7");
  assert.equal(result.workspaceUrl, workspaceUrl);
  assert.equal(result.url, workspaceUrl);
  assert.equal(workspaceVerified, workspaceOpenUrl);
  assert.equal(workspaceCookie, "opl_ws_active=ws-console-7");
  assert.equal(result.manifest.workspaceUrl, workspaceUrl);
  assert.doesNotMatch(JSON.stringify(result.manifest), /share-console-secret|token|password|csrf|session/i);
  assert.equal(result.billing.compute.holdId, "hold-compute-7");
  assert.equal(result.billing.compute.firstHourDebitId, "bill-compute");
  assert.equal(result.billing.storage.holdId, "hold-storage-7");
  assert.equal(result.billing.storage.firstHourDebitId, "bill-storage");
  assert.equal(result.releaseBalances.before, 998.9);
  assert.equal(result.releaseBalances.afterCompute, 998.9);
  assert.equal(result.releaseBalances.afterStorage, 998.9);
  assert.deepEqual(result.releaseBalances.balanceCents, { before: 99890, afterCompute: 99890, afterStorage: 99890 });
  assert.deepEqual(result.releaseBalances.frozenCents, { before: 18980, afterCompute: 2180, afterStorage: 500 });
  assert.deepEqual(result.releaseBalances.releasedCents, { compute: 16800, storage: 1680 });
  assert.deepEqual(apiWrites, []);
  assert.deepEqual(mutations.map(({ path }) => path), [
    "/api/compute-allocations", "/api/compute-allocations", "/api/storage-volumes", "/api/storage-attachments", "/api/workspaces",
    "/api/storage-attachments/detach", "/api/compute-allocations/compute-7/destroy", "/api/storage-volumes/destroy"
  ]);
  assert.deepEqual(mutations.map(({ headers }) => headers["idempotency-key"]), [
    "production-verification:console-run-7:03:create-compute",
    "production-verification:console-run-7:03:create-compute",
    "production-verification:console-run-7:03:create-storage",
    "production-verification:console-run-7:03:create-attachment",
    "production-verification:console-run-7:03:create-workspace",
    "production-verification:console-run-7:03:detach-storage",
    "production-verification:console-run-7:03:destroy-compute",
    "production-verification:console-run-7:03:destroy-storage"
  ]);
  assert.deepEqual(screenshots.map(({ file }) => file), [
    "console-login.png", "compute-hold.png", "compute-running.png", "storage-hold.png",
    "storage-available.png", "attached.png", "workspace-ready.png", "workspace-reply.png",
    "billing.png", "detached.png", "compute-destroyed.png", "storage-destroyed.png"
  ]);
  assert.deepEqual(screenshots.map(({ path }) => path), [
    "/console/overview", "/console/compute/new", "/console/compute", "/console/storage/new",
    "/console/storage", "/console/attachments", "/console/workspaces", "/console/workspaces",
    "/console/billing", "/console/attachments/attachment-7", "/console/compute/compute-7", "/console/storage/storage-7"
  ]);
  for (const expected of [
    { name: "compute-7", path: "/console/billing" },
    { name: "storage-7", path: "/console/billing" },
    { name: "detached", path: "/console/attachments/attachment-7" },
    { name: "destroyed", path: "/console/compute/compute-7" },
    { name: "已停止", path: "/console/compute/compute-7" },
    { name: "destroyed", path: "/console/storage/storage-7" },
    { name: "已停止", path: "/console/storage/storage-7" }
  ]) assert.ok(visibleTexts.some((item) => item.name === expected.name && item.path === expected.path));
});

test("production Console persists resource IDs before Hold and Machine facts arrive", async (t) => {
  const artifactDir = await testTempDir(t);
  const snapshots = [];
  await verifyProductionConsoleLifecycle({
    origin: "https://cloud.medopl.cn",
    accountId,
    ownerEmail: "owner@example.com",
    ownerPassword: "owner-password",
    runId,
    slot,
    attempts: 2,
    retryDelayMs: 0,
    manifestPath: join(artifactDir, "manifest.json"),
    screenshotDir: join(artifactDir, "screenshots"),
    browserFactory: fakeConsoleBrowserFactory({
      stages: [], screenshots: [], mutations: [], partialIdentityProjection: true
    }),
    manifestWriter: async (_path, manifest) => { snapshots.push(structuredClone(manifest)); },
    workspaceVerifier: async () => {}
  });
  assert.ok(snapshots.some((manifest) => manifest.ids.computeAllocationId === "compute-7" && !manifest.holdIds.compute));
  assert.ok(snapshots.some((manifest) => manifest.ids.storageId === "storage-7" && !manifest.holdIds.storage));
  assert.ok(snapshots.some((manifest) => manifest.machineIdentities["compute-7"]?.instanceId === "ins-7"));
});

test("production Console rejects duplicate Machine ownership before attaching storage", async (t) => {
  const artifactDir = await testTempDir(t);
  const mutations = [];
  await assert.rejects(() => verifyProductionConsoleLifecycle({
    origin: "https://cloud.medopl.cn",
    accountId,
    ownerEmail: "owner@example.com",
    ownerPassword: "owner-password",
    runId,
    slot,
    attempts: 1,
    retryDelayMs: 0,
    manifestPath: join(artifactDir, "manifest.json"),
    screenshotDir: artifactDir,
    browserFactory: fakeConsoleBrowserFactory({
      stages: [], screenshots: [], mutations, duplicateMachineIdentity: true
    }),
    fetchImpl: async () => { throw new Error("ownership_cleanup_read_blocked"); }
  }), /verification_resource_ownership_mismatch/);
  assert.deepEqual(mutations.map(({ path }) => path), ["/api/compute-allocations"]);
});

test("production Console rejects private origins before opening a browser", async () => {
  let browserOpened = false;
  for (const origin of [
    "https://127.0.0.1",
    "https://attacker.example",
    "https://user:password@cloud.medopl.cn",
    "https://cloud.medopl.cn:444"
  ]) {
    await assert.rejects(() => verifyProductionConsoleLifecycle({
      origin,
      accountId,
      ownerEmail: "owner@example.com",
      ownerPassword: "owner-password",
      runId,
      slot,
      screenshotDir: "/tmp/private-origin-must-not-open",
      browserFactory: async () => { browserOpened = true; throw new Error("browser_opened"); }
    }), /public_origin_required/);
  }
  assert.equal(browserOpened, false);
});

test("production Console refuses all UI mutations while production readiness is blocked", async (t) => {
  const artifactDir = await testTempDir(t);
  const mutations = [];
  await assert.rejects(() => verifyProductionConsoleLifecycle({
    origin: "https://cloud.medopl.cn",
    accountId,
    ownerEmail: "owner@example.com",
    ownerPassword: "owner-password",
    runId,
    slot,
    attempts: 1,
    retryDelayMs: 0,
    screenshotDir: artifactDir,
    browserFactory: fakeConsoleBrowserFactory({ stages: [], screenshots: [], mutations, readiness: false })
  }), /production_readiness_not_ready/);
  assert.deepEqual(mutations, []);
});

test("production Console failure requests exact manifest cleanup and never broad cleanup", async (t) => {
  const artifactDir = await testTempDir(t);
  const cleanupWrites = [];
  const cookieReads = [];
  let caught = null;
  try {
    await verifyProductionConsoleLifecycle({
    origin: "https://cloud.medopl.cn",
    accountId,
    ownerEmail: "owner@example.com",
    ownerPassword: "owner-password",
    runId,
    slot,
    attempts: 1,
    retryDelayMs: 0,
    manifestPath: join(artifactDir, "manifest.json"),
    screenshotDir: artifactDir,
    browserFactory: fakeConsoleBrowserFactory({
      stages: [], screenshots: [], mutations: [], failCookieRefresh: true, cookieReads
    }),
    workspaceVerifier: async () => { throw new Error("workspace_reply_failed"); },
    fetchImpl: async (url, options = {}) => {
      const method = options.method || "GET";
      const path = new URL(url).pathname;
      if (method === "GET") return jsonResponse(cleanupOwnershipState());
      cleanupWrites.push({ method, path, headers: options.headers });
      if (path === "/api/storage-attachments/detach") return jsonResponse({ status: "detached" });
      if (path === "/api/compute-allocations/compute-7/destroy") return jsonResponse({
        status: "destroyed", billingStatus: "stopped", holdId: "hold-compute-7", holdReleaseId: "release-compute-7"
      });
      if (path === "/api/storage-volumes/destroy") return jsonResponse({
        status: "destroyed", billingStatus: "stopped", holdId: "hold-storage-7", holdReleaseId: "release-storage-7"
      });
      throw new Error(`unexpected_cleanup:${method}:${path}`);
    }
  });
  } catch (error) {
    caught = error;
  }
  assert.equal(caught?.message, "workspace_reply_failed");
  assert.deepEqual(caught?.cleanupErrors, []);
  assert.deepEqual(cookieReads, [1, 2]);
  assert.deepEqual(cleanupWrites.map(({ path }) => path), [
    "/api/storage-attachments/detach", "/api/compute-allocations/compute-7/destroy", "/api/storage-volumes/destroy"
  ]);
  for (const write of cleanupWrites) {
    assert.equal(write.headers.cookie, "opl_session=session-console-7");
    assert.equal(write.headers["x-opl-csrf"], "csrf-console-7");
    assert.match(write.headers["Idempotency-Key"], /^production-verification:console-run-7:03:/);
  }
});

test("production Console default cleanup sends no writes for mismatched Hold ownership", async (t) => {
  const artifactDir = await testTempDir(t);
  const cleanupWrites = [];
  let caught = null;
  try {
    await verifyProductionConsoleLifecycle({
      origin: "https://cloud.medopl.cn",
      accountId,
      ownerEmail: "owner@example.com",
      ownerPassword: "owner-password",
      runId,
      slot,
      attempts: 1,
      retryDelayMs: 0,
      manifestPath: join(artifactDir, "manifest.json"),
      screenshotDir: artifactDir,
      browserFactory: fakeConsoleBrowserFactory({ stages: [], screenshots: [], mutations: [] }),
      workspaceVerifier: async () => { throw new Error("workspace_reply_failed"); },
      fetchImpl: async (url, options = {}) => {
        if ((options.method || "GET") !== "GET") cleanupWrites.push({ url, options });
        const state = cleanupOwnershipState();
        state.computeAllocations[0].holdId = "hold-other";
        return jsonResponse(state);
      }
    });
  } catch (error) {
    caught = error;
  }
  assert.equal(caught?.message, "workspace_reply_failed");
  assert.deepEqual(caught?.cleanupErrors, ["verification_resource_ownership_mismatch", "verification_resource_ownership_mismatch"]);
  assert.deepEqual(cleanupWrites, []);
});

test("production Console preserves the primary error when cleanup and browser close fail", async (t) => {
  const artifactDir = await testTempDir(t);
  const cleanupStages = [];
  const closeEvents = [];
  let caught = null;
  try {
    await verifyProductionConsoleLifecycle({
      origin: "https://cloud.medopl.cn",
      accountId,
      ownerEmail: "owner@example.com",
      ownerPassword: "owner-password",
      runId,
      slot,
      attempts: 1,
      retryDelayMs: 0,
      manifestPath: join(artifactDir, "manifest.json"),
      screenshotDir: artifactDir,
      browserFactory: fakeConsoleBrowserFactory({
        stages: [], screenshots: [], mutations: [], closeEvents,
        contextCloseError: "context_close_failed", browserCloseError: "browser_close_failed"
      }),
      workspaceVerifier: async () => { throw new Error("workspace_reply_failed"); },
      cleanup: async ({ cleanupStage }) => {
        cleanupStages.push(cleanupStage);
        if (cleanupStage === "first-cleanup") {
          throw new Error(`cleanup_failed:${workspaceOpenUrl}&unknown=cleanup-secret`);
        }
        return [];
      }
    });
  } catch (error) {
    caught = error;
  }
  assert.equal(caught?.message, "workspace_reply_failed");
  assert.deepEqual(cleanupStages, ["first-cleanup", "final-cleanup"]);
  assert.deepEqual(closeEvents, ["context", "browser"]);
  assert.doesNotMatch(JSON.stringify(caught?.cleanupErrors), /share-console-secret|cleanup-secret|\?token/);
  assert.ok(caught?.cleanupErrors.some((item) => item.startsWith("first-cleanup:")));
  assert.ok(caught?.cleanupErrors.includes("close_context:context_close_failed"));
  assert.ok(caught?.cleanupErrors.includes("close_browser:browser_close_failed"));
});

test("production Console closes initialized browser resources when newPage fails", async (t) => {
  const artifactDir = await testTempDir(t);
  const closed = [];
  const context = {
    async newPage() { throw new Error("new_page_failed"); },
    async close() { closed.push("context"); }
  };
  await assert.rejects(() => verifyProductionConsoleLifecycle({
    origin: "https://cloud.medopl.cn",
    accountId,
    ownerEmail: "owner@example.com",
    ownerPassword: "owner-password",
    runId,
    slot,
    manifestPath: join(artifactDir, "manifest.json"),
    screenshotDir: artifactDir,
    browserFactory: async () => ({
      async newContext() { return context; },
      async close() { closed.push("browser"); }
    })
  }), /new_page_failed/);
  assert.deepEqual(closed, ["context", "browser"]);
});

test("production Console CLI maps owner env and prints one secret-free JSON result", () => {
  const moduleUrl = new URL("../../tools/production-console-browser-verifier.ts", import.meta.url).href;
  const script = `
    import { runProductionConsoleBrowserVerifierCli } from ${JSON.stringify(moduleUrl)};
    await runProductionConsoleBrowserVerifierCli({
      verify: async (options) => ({ ok: true, workspaceId: "ws-cli", url: "https://workspace.medopl.cn/w/ws-cli/", accountId: options.accountId, runId: options.runId, slot: options.slot })
    });
  `;
  const child = spawnSync(process.execPath, ["--input-type=module", "--eval", script], {
    encoding: "utf8",
    env: {
      ...process.env,
      OPL_CONSOLE_ORIGIN: "https://cloud.medopl.cn",
      OPL_VERIFY_ACCOUNT_ID: accountId,
      OPL_VERIFY_AUTH_USERS_JSON: JSON.stringify([{ accountId, email: "owner@example.com", password: "cli-secret-password", role: "owner" }]),
      OPL_VERIFY_RUN_ID: "cli-run",
      OPL_VERIFY_SLOT: "04",
      OPL_VERIFY_SCREENSHOT_DIR: "/tmp/cli-screenshots"
    }
  });
  assert.equal(child.status, 0, child.stderr);
  assert.deepEqual(JSON.parse(child.stdout), {
    ok: true,
    workspaceId: "ws-cli",
    url: "https://workspace.medopl.cn/w/ws-cli/",
    accountId,
    runId: "cli-run",
    slot: "04"
  });
  assert.doesNotMatch(`${child.stdout}${child.stderr}`, /cli-secret-password|owner@example\.com/);
});

test("production Console CLI scrubs Workspace query secrets from errors", async () => {
  let stdout = "";
  let stderr = "";
  const env = {
    OPL_CONSOLE_ORIGIN: "https://cloud.medopl.cn",
    OPL_VERIFY_ACCOUNT_ID: accountId,
    OPL_VERIFY_AUTH_USERS_JSON: JSON.stringify([{ accountId, email: "owner@example.com", password: "owner-password", role: "owner" }]),
    OPL_VERIFY_RUN_ID: "cli-secret-error",
    OPL_VERIFY_SCREENSHOT_DIR: "/tmp/unused-cli-secret-error"
  };
  await assert.rejects(() => runProductionConsoleBrowserVerifierCli({
    env,
    stdout: { write: (chunk) => { stdout += chunk; } },
    stderr: { write: (chunk) => { stderr += chunk; } },
    verify: async () => { throw new Error(`workspace_navigation_failed:${workspaceOpenUrl}&unknown=raw-secret`); }
  }), /raw-secret/);
  assert.equal(stdout, "");
  assert.doesNotMatch(stderr, /share-console-secret|raw-secret|\?token/);
  assert.equal(JSON.parse(stderr).error, "workspace_navigation_failed:https://workspace.medopl.cn/w/ws-console-7/");
});

test("production Console CLI main reports invalid owner seed as secret-free JSON", () => {
  const modulePath = fileURLToPath(new URL("../../tools/production-console-browser-verifier.ts", import.meta.url));
  const child = spawnSync(process.execPath, [modulePath], {
    encoding: "utf8",
    env: {
      ...process.env,
      OPL_CONSOLE_ORIGIN: "https://cloud.medopl.cn",
      OPL_VERIFY_ACCOUNT_ID: accountId,
      OPL_VERIFY_AUTH_USERS_JSON: "invalid-secret-seed",
      OPL_VERIFY_RUN_ID: "cli-invalid-seed",
      OPL_VERIFY_SCREENSHOT_DIR: "/tmp/cli-invalid-seed"
    }
  });
  assert.equal(child.status, 1);
  assert.equal(child.stdout, "");
  assert.equal(JSON.parse(child.stderr).error, "verification_owner_credentials_required");
  assert.doesNotMatch(child.stderr, /invalid-secret-seed/);
});

test("rendered Console controls expose the labels used by the browser verifier", async (t) => {
  const [{ createElement }, { renderToStaticMarkup }, { createServer }] = await Promise.all([
    import("react"),
    import("react-dom/server"),
    import("vite")
  ]);
  const vite = await createServer({ server: { middlewareMode: true }, appType: "custom", logLevel: "silent" });
  t.after(() => vite.close());
  const [{ default: LoginPage }, resources] = await Promise.all([
    vite.ssrLoadModule("/apps/console-ui/src/pages/LoginPage.tsx"),
    vite.ssrLoadModule("/apps/console-ui/src/pages/resources/ResourceProvisioningPages.tsx")
  ]);
  const loginHtml = renderToStaticMarkup(createElement(LoginPage, { route: { path: "/login" }, onLogin() {} }));
  assert.match(loginHtml, /邮箱/);
  assert.match(loginHtml, /密码/);
  assert.match(loginHtml, /登录/);

  const props = {
    state: {
      account: { accountId },
      wallet: { accountId, balance: 1000, frozen: 0, available: 1000 },
      packages: [{ id: "basic", name: "Basic", available: true, server: "2c4g", cpu: 2, memoryGb: 4, diskGb: 10 }]
    },
    session: { csrfToken: "ssr" },
    runAction: async (action) => action()
  };
  const computeHtml = renderToStaticMarkup(createElement(resources.CreateComputeAllocationPage, props));
  for (const text of ["名称", "规格", "每小时价格", "预冻结", "冻结后可用", "7 天", "开通计算"]) {
    assert.match(computeHtml, new RegExp(text));
  }
  const storageHtml = renderToStaticMarkup(createElement(resources.CreateStorageVolumePage, props));
  for (const text of ["名称", "计费规格", "容量 GB", "预冻结", "冻结后可用", "7 天", "开通存储"]) {
    assert.match(storageHtml, new RegExp(text));
  }
});
