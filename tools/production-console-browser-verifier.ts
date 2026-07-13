import { access, copyFile, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { pathToFileURL } from "node:url";

import {
  cleanupVerificationResources,
  assertPublicHttpsUrl,
  productionVerificationMutationKey,
  verificationOwnerFromSeed,
  verifyWorkspaceBrowserUi,
  writeVerificationManifest
} from "./production-verifier.ts";

const DEFAULT_SLOT = "01";
const DEFAULT_ATTEMPTS = 90;
const DEFAULT_RETRY_DELAY_MS = 10_000;

function sleep(ms) {
  return ms > 0 ? new Promise((resolve) => setTimeout(resolve, ms)) : Promise.resolve();
}

function safeErrorText(value) {
  return String(value || "error")
    .replace(/https?:\/\/[^\s"'<>]+/g, (raw) => {
      try {
        const parsed = new URL(raw);
        parsed.search = "";
        parsed.hash = "";
        return parsed.toString();
      } catch {
        return "redacted_url";
      }
    })
    .replace(/((?:token|code|key|password|secret|session|auth)=)[^&\s]+/gi, "$1[redacted]");
}

function safeErrorValue(value) {
  if (Array.isArray(value)) return value.map(safeErrorValue);
  if (value && typeof value === "object") {
    return Object.fromEntries(Object.entries(value).map(([key, nested]) => [key, safeErrorValue(nested)]));
  }
  return typeof value === "string" ? safeErrorText(value) : value;
}

function cookieHeader(cookies = []) {
  return cookies.map(({ name, value }) => `${name}=${value}`).join("; ");
}

export async function runProductionConsoleBrowserVerifierCli({
  env = process.env,
  stdout = process.stdout,
  stderr = process.stderr,
  verify = verifyProductionConsoleLifecycle
} = {}) {
  const requestedAccountId = env.OPL_VERIFY_ACCOUNT_ID || "";
  const screenshotDir = env.OPL_VERIFY_SCREENSHOT_DIR || "";
  try {
    const owner = verificationOwnerFromSeed(env.OPL_VERIFY_AUTH_USERS_JSON, requestedAccountId);
    const result = await verify({
      origin: env.OPL_CONSOLE_ORIGIN,
      accountId: owner.accountId || requestedAccountId,
      ownerEmail: owner.email,
      ownerPassword: owner.password,
      runId: env.OPL_VERIFY_RUN_ID,
      slot: env.OPL_VERIFY_SLOT || DEFAULT_SLOT,
      packageId: env.OPL_VERIFY_PACKAGE_ID || "basic",
      workspaceName: env.OPL_VERIFY_WORKSPACE_NAME || "Production Verification Lab",
      attempts: Number(env.OPL_VERIFY_URL_ATTEMPTS || DEFAULT_ATTEMPTS),
      retryDelayMs: Number(env.OPL_VERIFY_RETRY_DELAY_MS || DEFAULT_RETRY_DELAY_MS),
      manifestPath: env.OPL_VERIFY_MANIFEST_PATH || (screenshotDir ? join(screenshotDir, "manifest.json") : ""),
      screenshotDir,
      modelAccessKey: env.OPL_VERIFY_MODEL_ACCESS_KEY || env.OPL_CODEX_API_KEY || ""
    });
    stdout.write(`${JSON.stringify(result)}\n`);
    return result;
  } catch (error) {
    stderr.write(`${JSON.stringify(safeErrorValue({
      ok: false,
      error: error.message,
      cleanupErrors: error.cleanupErrors || []
    }))}\n`);
    throw error;
  }
}

if (import.meta.url === pathToFileURL(process.argv[1] || "").href) {
  runProductionConsoleBrowserVerifierCli().catch(() => { process.exitCode = 1; });
}

function normalizedOrigin(origin) {
  const parsed = assertPublicHttpsUrl(origin, "public_origin_required", { hostname: "cloud.medopl.cn" });
  parsed.pathname = "/";
  parsed.search = "";
  parsed.hash = "";
  return parsed.toString().replace(/\/$/, "");
}

function publicWorkspaceUrl(value, workspaceId) {
  const parsed = assertPublicHttpsUrl(value, "workspace_url_required", { hostname: "workspace.medopl.cn" });
  if (parsed.pathname !== `/w/${workspaceId}/`) {
    throw new Error("workspace_url_required");
  }
  return parsed.toString();
}

function scrubWorkspaceUrl(value) {
  const parsed = new URL(value);
  parsed.search = "";
  parsed.hash = "";
  return parsed.toString();
}

async function defaultBrowserFactory() {
  const { chromium } = await import("playwright");
  return chromium.launch({ headless: true });
}

async function firstVisible(locator) {
  const candidate = locator.filter({ visible: true }).first();
  await candidate.waitFor({ state: "visible" });
  return candidate;
}

async function clickVisible(locator) {
  await (await firstVisible(locator)).click();
}

async function waitVisible(locator) {
  await firstVisible(locator);
}

async function readState(page, accountId) {
  return readPageJson(page, `/api/state?accountId=${encodeURIComponent(accountId)}`);
}

async function readPageJson(page, path) {
  return page.evaluate(async (requestPath) => {
    const response = await fetch(requestPath, { method: "GET" });
    const payload = await response.json();
    if (!response.ok || payload?.ok === false) throw new Error(payload?.error || `state_failed:${response.status}`);
    return payload;
  }, path);
}

function resourceAccountId(resource) {
  return resource?.accountId || resource?.ownerAccountId || "";
}

function uniqueNamed(rows, name, accountId) {
  const matches = (rows || []).filter((row) => row?.name === name && resourceAccountId(row) === accountId);
  if (matches.length > 1) throw new Error("verification_resource_ownership_mismatch");
  return matches[0] || null;
}

function exactId(rows, id, accountId) {
  const matches = (rows || []).filter((row) => row?.id === id && resourceAccountId(row) === accountId);
  if (matches.length !== 1) throw new Error("verification_resource_ownership_mismatch");
  return matches[0];
}

async function waitForState({ page, accountId, attempts, retryDelayMs, predicate, onState }) {
  let lastState = null;
  for (let attempt = 1; attempt <= attempts; attempt += 1) {
    lastState = await readState(page, accountId);
    await onState?.(lastState);
    const result = predicate(lastState);
    if (result) return { state: lastState, result };
    if (attempt < attempts) await sleep(retryDelayMs);
  }
  throw new Error("console_state_transition_timeout");
}

function mutationStage(method, pathname) {
  if (method !== "POST") return "";
  if (pathname === "/api/compute-allocations") return "create-compute";
  if (pathname === "/api/storage-volumes") return "create-storage";
  if (pathname === "/api/storage-attachments") return "create-attachment";
  if (pathname === "/api/workspaces") return "create-workspace";
  if (pathname === "/api/storage-attachments/detach") return "detach-storage";
  if (/^\/api\/compute-allocations\/[^/]+\/destroy$/.test(pathname)) return "destroy-compute";
  if (pathname === "/api/storage-volumes/destroy") return "destroy-storage";
  return "";
}

async function clickConfirmed(page, label) {
  const action = await firstVisible(page.getByRole("button", { name: label, exact: true }));
  if (!await action.isEnabled()) throw new Error(`console_action_disabled:${label}`);
  await action.click();
  await clickVisible(page.getByRole("button", { name: "确认", exact: true }));
}

async function selectVisibleOption(page, label, optionName) {
  await clickVisible(page.getByLabel(label));
  await clickVisible(page.getByRole("option", { name: optionName, exact: false }));
}

function activationDebit(rows, resource, resourceType) {
  if (!resource?.id || !resource?.holdId || !resource?.ledgerEntryId) throw new Error("first_hour_debit_required");
  const matches = (rows || []).filter((row) =>
    row?.id === resource.ledgerEntryId && row?.type === `${resourceType}_activation` &&
    row?.reason === resource.id && Number(row?.amountCents || 0) < 0
  );
  if (matches.length !== 1) throw new Error("first_hour_debit_required");
  return matches[0];
}

export async function verifyProductionConsoleLifecycle({
  origin,
  accountId,
  ownerEmail,
  ownerPassword,
  runId,
  slot = DEFAULT_SLOT,
  packageId = "basic",
  workspaceName = "Production Verification Lab",
  attempts = DEFAULT_ATTEMPTS,
  retryDelayMs = DEFAULT_RETRY_DELAY_MS,
  manifestPath = "",
  screenshotDir = "",
  modelAccessKey = "",
  browserFactory = defaultBrowserFactory,
  workspaceVerifier = verifyWorkspaceBrowserUi,
  cleanup = cleanupVerificationResources,
  manifestWriter = writeVerificationManifest,
  fetchImpl = globalThis.fetch
} = {}) {
  if (!accountId || !ownerEmail || !ownerPassword) throw new Error("verification_owner_credentials_required");
  if (!Number.isInteger(attempts) || attempts < 1) throw new Error("verification_attempts_invalid");
  const consoleOrigin = normalizedOrigin(origin);
  productionVerificationMutationKey(runId, slot, "validate");
  if (packageId !== "basic") throw new Error("production_console_basic_package_required");

  const effectiveWorkspaceName = workspaceName.includes(runId) ? workspaceName : `${workspaceName} ${runId}`;
  const computeName = `${workspaceName} compute ${runId}`;
  const storageName = `${workspaceName} storage ${runId}`;
  const mutationKeys = Object.fromEntries([
    "create-compute", "create-storage", "create-attachment", "create-workspace",
    "detach-storage", "destroy-compute", "destroy-storage"
  ].map((stage) => [stage, productionVerificationMutationKey(runId, slot, stage)]));
  const manifest = {
    runId,
    slot,
    accountId,
    resourceNames: { compute: computeName, storage: storageName, workspace: effectiveWorkspaceName },
    ids: {},
    holdIds: {},
    machineIdentities: {},
    mutationKeys
  };
  const persist = () => manifestWriter(manifestPath, manifest);
  const stages = [];
  let browser = null;
  let context = null;
  let page = null;
  let primaryError = null;
  let csrf = "";
  let compute = null;
  let storage = null;
  let attachment = null;
  let workspace = null;
  let ownerCookie = "";

  const screenshot = async (stage, fileName) => {
    if (!screenshotDir) throw new Error("production_console_screenshot_dir_required");
    await mkdir(screenshotDir, { recursive: true });
    await page.screenshot({ path: join(screenshotDir, fileName), fullPage: true });
    if (stage) {
      stages.push(stage);
    }
  };
  const rememberCompute = async (value) => {
    if (!value?.id) throw new Error("compute_identity_incomplete");
    manifest.ids.computeAllocationId = value.id;
    if (value.holdId) manifest.holdIds.compute = value.holdId;
    if (value.machineName && (value.instanceId || value.cvmInstanceId) && value.nodeName) {
      manifest.machineIdentities[value.id] = {
        machineId: value.machineName,
        instanceId: value.instanceId || value.cvmInstanceId,
        nodeName: value.nodeName
      };
    }
    await persist();
  };

  try {
    browser = await browserFactory();
    context = await browser.newContext();
    page = await context.newPage();
    await page.route("**/api/**", async (route, request) => {
      const method = request.method();
      const stage = mutationStage(method, new URL(request.url()).pathname);
      if (!stage) return route.continue();
      const headers = { ...request.headers(), "idempotency-key": mutationKeys[stage] };
      csrf = headers["x-opl-csrf"] || csrf;
      await route.continue({ headers });
    });
    await page.goto(`${consoleOrigin}/login`, { waitUntil: "networkidle" });
    await page.getByLabel("邮箱").fill(ownerEmail);
    await page.getByLabel("密码").fill(ownerPassword);
    await clickVisible(page.getByRole("button", { name: "登录", exact: true }));
    await page.waitForURL("**/console*", { timeout: 30_000 });
    await waitVisible(page.getByText("OPL Console", { exact: true }));
    ownerCookie = cookieHeader(await context.cookies(consoleOrigin));
    if (!ownerCookie) throw new Error("verification_owner_session_required");
    await screenshot("login", "console-login.png");
    for (const [path, name] of [
      ["/api/production/readiness", "production_readiness"],
      ["/api/runtime/readiness", "runtime_readiness"]
    ]) {
      const readiness = await readPageJson(page, path);
      if (readiness?.ready !== true) throw new Error(`${name}_not_ready`);
    }

    await page.goto(`${consoleOrigin}/console/compute/new`, { waitUntil: "networkidle" });
    await page.getByLabel("名称").fill(computeName);
    await selectVisibleOption(page, "规格", "Basic");
    for (const label of ["每小时价格", "预冻结", "冻结后可用"]) await waitVisible(page.getByText(label, { exact: true }));
    await waitVisible(page.getByText("7 天", { exact: true }));
    await screenshot("compute_hold_preview", "compute-hold.png");
    await clickConfirmed(page, "开通计算");
    ({ result: compute } = await waitForState({
      page, accountId, attempts, retryDelayMs,
      onState: async (state) => {
        const seen = (state.computeAllocations || []).find((row) => row.name === computeName && resourceAccountId(row) === accountId);
        if (seen) {
          compute = seen;
          await rememberCompute(seen);
        }
      },
      predicate: (state) => {
        const row = uniqueNamed(state.computeAllocations, computeName, accountId);
        if (!row) return null;
        const instanceId = row.instanceId || row.cvmInstanceId;
        const identityMatches = (state.computeAllocations || []).filter((candidate) =>
          resourceAccountId(candidate) === accountId && candidate.machineName === row.machineName &&
          (candidate.instanceId || candidate.cvmInstanceId) === instanceId && candidate.nodeName === row.nodeName
        ).length;
        if (row.machineName && instanceId && row.nodeName && identityMatches !== 1) {
          throw new Error("verification_resource_ownership_mismatch");
        }
        return row.status === "running" && row.billingStatus === "active" && row.packageId === "basic" &&
          row.holdId && row.machineName && instanceId && row.nodeName && identityMatches === 1 ? row : null;
      }
    }));
    await rememberCompute(compute);
    await page.goto(`${consoleOrigin}/console/compute`, { waitUntil: "networkidle" });
    await waitVisible(page.getByText(computeName, { exact: true }));
    await waitVisible(page.getByText("running", { exact: true }));
    await screenshot("compute_running", "compute-running.png");

    await page.goto(`${consoleOrigin}/console/storage/new`, { waitUntil: "networkidle" });
    await page.getByLabel("名称").fill(storageName);
    await selectVisibleOption(page, "计费规格", "Basic");
    await page.getByLabel("容量 GB").fill("10");
    for (const label of ["预冻结", "冻结后可用"]) await waitVisible(page.getByText(label, { exact: true }));
    await waitVisible(page.getByText("7 天", { exact: true }));
    await screenshot("storage_hold_preview", "storage-hold.png");
    await clickConfirmed(page, "开通存储");
    ({ result: storage } = await waitForState({
      page, accountId, attempts, retryDelayMs,
      onState: async (state) => {
        const seen = (state.storageVolumes || []).find((row) => row.name === storageName && resourceAccountId(row) === accountId);
        if (seen?.id) {
          storage = seen;
          manifest.ids.storageId = seen.id;
          if (seen.holdId) manifest.holdIds.storage = seen.holdId;
          await persist();
        }
      },
      predicate: (state) => {
        const row = uniqueNamed(state.storageVolumes, storageName, accountId);
        if (!row) return null;
        return row.status === "available" && row.billingStatus === "active" && row.holdId ? row : null;
      }
    }));
    await page.goto(`${consoleOrigin}/console/storage`, { waitUntil: "networkidle" });
    await waitVisible(page.getByText(storageName, { exact: true }));
    await waitVisible(page.getByText("available", { exact: true }));
    await screenshot("storage_available", "storage-available.png");

    await page.goto(`${consoleOrigin}/console/attachments/new`, { waitUntil: "networkidle" });
    await selectVisibleOption(page, "计算资源", computeName);
    await selectVisibleOption(page, "存储资源", storageName);
    await clickConfirmed(page, "挂载存储");
    ({ result: attachment } = await waitForState({
      page, accountId, attempts, retryDelayMs,
      onState: async (state) => {
        const seen = (state.storageAttachments || []).find((row) =>
          resourceAccountId(row) === accountId && row.computeAllocationId === compute.id && row.storageId === storage.id
        );
        if (seen?.id) {
          attachment = seen;
          manifest.ids.attachmentId = seen.id;
          await persist();
        }
      },
      predicate: (state) => {
        const matches = (state.storageAttachments || []).filter((row) =>
          resourceAccountId(row) === accountId && row.computeAllocationId === compute.id && row.storageId === storage.id
        );
        if (matches.length > 1) throw new Error("verification_resource_ownership_mismatch");
        if (matches.length === 0) return null;
        return matches[0].status === "attached" && matches[0].mountPath === "/data" ? matches[0] : null;
      }
    }));
    manifest.ids.attachmentId = attachment.id;
    await persist();
    await page.goto(`${consoleOrigin}/console/attachments`, { waitUntil: "networkidle" });
    await waitVisible(page.getByText(attachment.id, { exact: true }));
    await waitVisible(page.getByText("attached", { exact: true }));
    await screenshot("attached", "attached.png");

    await page.goto(`${consoleOrigin}/console/workspaces/new`, { waitUntil: "networkidle" });
    await page.getByLabel("名称").fill(effectiveWorkspaceName);
    await selectVisibleOption(page, "挂载关系", `${computeName} + ${storageName}`);
    await clickVisible(page.getByRole("button", { name: "创建工作区入口", exact: true }));
    ({ result: workspace } = await waitForState({
      page, accountId, attempts, retryDelayMs,
      onState: async (state) => {
        const seen = (state.workspaces || []).find((row) => row.name === effectiveWorkspaceName && resourceAccountId(row) === accountId);
        if (seen?.id) {
          workspace = seen;
          manifest.ids.workspaceId = seen.id;
          manifest.workspaceId = seen.id;
          if (seen.url) manifest.workspaceUrl = scrubWorkspaceUrl(publicWorkspaceUrl(seen.url, seen.id));
          await persist();
        }
      },
      predicate: (state) => {
        const row = uniqueNamed(state.workspaces, effectiveWorkspaceName, accountId);
        if (!row) return null;
        return row.state === "running" && row.openable === true && row.accessState === "available" &&
          row.computeAllocationId === compute.id && row.storageId === storage.id &&
          row.attachmentId === attachment.id && row.url ? row : null;
      }
    }));
    manifest.ids.workspaceId = workspace.id;
    manifest.workspaceId = workspace.id;
    manifest.workspaceUrl = scrubWorkspaceUrl(publicWorkspaceUrl(workspace.url, workspace.id));
    await persist();
    await page.goto(`${consoleOrigin}/console/workspaces`, { waitUntil: "networkidle" });
    await waitVisible(page.getByText(effectiveWorkspaceName, { exact: true }));
    await waitVisible(page.getByText("运行中", { exact: true }));
    await screenshot("", "workspace-ready.png");

    await page.goto(`${consoleOrigin}/console/workspaces`, { waitUntil: "networkidle" });
    const desktopRow = page.locator(".desktopWorkspaceTable").getByRole("row").filter({ hasText: effectiveWorkspaceName });
    const popupPromise = page.waitForEvent("popup");
    await clickVisible(desktopRow.getByRole("button", { name: "打开", exact: true }));
    const popup = await popupPromise;
    await popup.waitForURL?.(`**/w/${workspace.id}/**`, { timeout: 120_000 });
    const openedUrl = publicWorkspaceUrl(popup.url(), workspace.id);
    const workspaceCookies = await popup.context?.().cookies(openedUrl) || [];
    await popup.close?.();
    stages.push("workspace_url_opened");

    await workspaceVerifier({
      workspaceUrl: openedUrl,
      workspaceAuth: {
        url: openedUrl,
        cookie: cookieHeader(workspaceCookies),
        webuiUsername: workspace.access?.account || workspace.access?.username,
        webuiPassword: workspace.access?.password
      },
      runId,
      checks: [],
      modelAccessKey,
      screenshotDir
    });
    const replyScreenshot = join(screenshotDir, "workspace-reply.png");
    const verifierScreenshot = join(screenshotDir, `workspace-browser-e2e-${runId}-success.png`);
    if (workspaceVerifier === verifyWorkspaceBrowserUi) {
      await access(verifierScreenshot);
      await copyFile(verifierScreenshot, replyScreenshot);
      stages.push("workspace_reply");
    } else {
      await screenshot("workspace_reply", "workspace-reply.png");
    }

    await page.goto(`${consoleOrigin}/console/billing`, { waitUntil: "networkidle" });
    for (const label of ["冻结", "余额", "资源费用", "预计每小时"]) await waitVisible(page.getByText(label, { exact: true }));
    await waitVisible(page.getByText(compute.id, { exact: true }));
    await waitVisible(page.getByText(storage.id, { exact: true }));
    const billingState = await readState(page, accountId);
    compute = exactId(billingState.computeAllocations, compute.id, accountId);
    storage = exactId(billingState.storageVolumes, storage.id, accountId);
    if (!compute.holdId || !storage.holdId) throw new Error("billing_hold_required");
    const computeDebit = activationDebit(billingState.billingLedger, compute, "compute");
    const storageDebit = activationDebit(billingState.billingLedger, storage, "storage");
    const balanceBeforeRelease = Number(billingState.wallet?.balance);
    const balanceCentsBeforeRelease = Number(billingState.wallet?.balanceCents);
    const frozenCentsBeforeRelease = Number(billingState.wallet?.frozenCents);
    const computeRemainingCents = Number(compute.holdAmountCents);
    const storageRemainingCents = Number(storage.holdAmountCents);
    if (![balanceBeforeRelease, balanceCentsBeforeRelease, frozenCentsBeforeRelease, computeRemainingCents, storageRemainingCents].every(Number.isFinite)) {
      throw new Error("wallet_balance_required");
    }
    await screenshot("billing_visible", "billing.png");

    await page.goto(`${consoleOrigin}/console/attachments/${encodeURIComponent(attachment.id)}`, { waitUntil: "networkidle" });
    await clickConfirmed(page, "解除挂载");
    await waitForState({
      page, accountId, attempts, retryDelayMs,
      predicate: (state) => exactId(state.storageAttachments, attachment.id, accountId).status === "detached"
    });
    await waitVisible(page.getByText("detached", { exact: true }));
    await screenshot("detached", "detached.png");

    await page.goto(`${consoleOrigin}/console/compute/${encodeURIComponent(compute.id)}`, { waitUntil: "networkidle" });
    await clickConfirmed(page, "销毁计算资源");
    const computeDestroyed = await waitForState({
      page, accountId, attempts, retryDelayMs,
      predicate: (state) => {
        const row = exactId(state.computeAllocations, compute.id, accountId);
        return row.status === "destroyed" && row.billingStatus === "stopped" && row.holdId === compute.holdId && row.holdReleaseId ? row : null;
      }
    });
    const balanceAfterCompute = Number(computeDestroyed.state.wallet?.balance);
    const balanceCentsAfterCompute = Number(computeDestroyed.state.wallet?.balanceCents);
    const frozenCentsAfterCompute = Number(computeDestroyed.state.wallet?.frozenCents);
    if (
      balanceAfterCompute !== balanceBeforeRelease || balanceCentsAfterCompute !== balanceCentsBeforeRelease ||
      frozenCentsAfterCompute !== frozenCentsBeforeRelease - computeRemainingCents
    ) throw new Error("hold_release_amount_mismatch");
    await waitVisible(page.getByText("destroyed", { exact: true }));
    await waitVisible(page.getByText("已停止", { exact: true }));
    await screenshot("compute_destroyed", "compute-destroyed.png");

    await page.goto(`${consoleOrigin}/console/storage/${encodeURIComponent(storage.id)}`, { waitUntil: "networkidle" });
    await clickConfirmed(page, "销毁存储资源");
    const storageDestroyed = await waitForState({
      page, accountId, attempts, retryDelayMs,
      predicate: (state) => {
        const row = exactId(state.storageVolumes, storage.id, accountId);
        return row.status === "destroyed" && row.billingStatus === "stopped" && row.holdId === storage.holdId && row.holdReleaseId ? row : null;
      }
    });
    const balanceAfterStorage = Number(storageDestroyed.state.wallet?.balance);
    const balanceCentsAfterStorage = Number(storageDestroyed.state.wallet?.balanceCents);
    const frozenCentsAfterStorage = Number(storageDestroyed.state.wallet?.frozenCents);
    if (
      balanceAfterStorage !== balanceBeforeRelease || balanceCentsAfterStorage !== balanceCentsBeforeRelease ||
      frozenCentsAfterStorage !== frozenCentsAfterCompute - storageRemainingCents
    ) throw new Error("hold_release_amount_mismatch");
    await waitVisible(page.getByText("destroyed", { exact: true }));
    await waitVisible(page.getByText("已停止", { exact: true }));
    await screenshot("storage_destroyed", "storage-destroyed.png");

    return {
      ok: true,
      accountId,
      runId,
      slot,
      workspaceId: workspace.id,
      workspaceUrl: scrubWorkspaceUrl(openedUrl),
      url: scrubWorkspaceUrl(openedUrl),
      stages,
      manifest,
      billing: {
        compute: { holdId: compute.holdId, firstHourDebitId: computeDebit.id },
        storage: { holdId: storage.holdId, firstHourDebitId: storageDebit.id }
      },
      releaseBalances: {
        before: balanceBeforeRelease,
        afterCompute: balanceAfterCompute,
        afterStorage: balanceAfterStorage,
        balanceCents: { before: balanceCentsBeforeRelease, afterCompute: balanceCentsAfterCompute, afterStorage: balanceCentsAfterStorage },
        frozenCents: { before: frozenCentsBeforeRelease, afterCompute: frozenCentsAfterCompute, afterStorage: frozenCentsAfterStorage },
        releasedCents: { compute: computeRemainingCents, storage: storageRemainingCents }
      }
    };
  } catch (error) {
    primaryError = error;
    const cleanupErrors = [];
    let cleanupCookie = ownerCookie;
    try {
      cleanupCookie = cookieHeader(await context?.cookies?.(consoleOrigin) || []) || cleanupCookie;
    } catch {
      // Cached immediately after login so cleanup does not depend on a healthy browser context.
    }
    const auth = {
      csrf,
      cookie: cleanupCookie
    };
    if (compute?.id || attachment?.id) {
      try {
        cleanupErrors.push(...(await cleanup({
          fetchImpl, origin: consoleOrigin, accountId, manifest,
          computeAllocationId: compute?.id,
          attachmentId: attachment?.id,
          expectedComputeHoldId: compute?.holdId || manifest.holdIds.compute,
          auth,
          attempts,
          retryDelayMs,
          cleanupStage: "first-cleanup"
        })).map(safeErrorText));
      } catch (cleanupError) {
        cleanupErrors.push(`first-cleanup:${safeErrorText(cleanupError.message)}`);
      }
    }
    if (storage?.id) {
      try {
        cleanupErrors.push(...(await cleanup({
          fetchImpl, origin: consoleOrigin, accountId, manifest,
          storageId: storage.id,
          expectedStorageHoldId: storage.holdId || manifest.holdIds.storage,
          auth,
          attempts,
          retryDelayMs,
          cleanupStage: "final-cleanup"
        })).map(safeErrorText));
      } catch (cleanupError) {
        cleanupErrors.push(`final-cleanup:${safeErrorText(cleanupError.message)}`);
      }
    }
    error.cleanupErrors = cleanupErrors;
    throw error;
  } finally {
    const closeErrors = [];
    for (const [name, resource] of [["context", context], ["browser", browser]]) {
      if (!resource?.close) continue;
      try {
        await resource.close();
      } catch (closeError) {
        closeErrors.push(`close_${name}:${safeErrorText(closeError.message)}`);
      }
    }
    if (primaryError) {
      primaryError.cleanupErrors = [...(primaryError.cleanupErrors || []), ...closeErrors];
    } else if (closeErrors.length > 0) {
      throw new Error(closeErrors.join(";"));
    }
  }
}
