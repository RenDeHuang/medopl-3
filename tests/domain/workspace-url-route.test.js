import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { createRequestHandler } from "../../packages/console/api/server.js";
import { createOplCloud } from "../../packages/console/src/opl-cloud.js";
import { LocalDockerProvider } from "../../packages/fabric/src/runtime-providers/local-docker.js";
import { MemoryStore } from "../../packages/console/src/store.js";

async function listen(handler) {
  const server = createServer(handler);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  return {
    origin: `http://127.0.0.1:${address.port}`,
    close: () => new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()))
  };
}

test("workspace URL route validates token and returns OPL Workspace entry page", async () => {
  const root = await mkdtemp(join(tmpdir(), "opl-cloud-route-"));
  const appService = createOplCloud({
    store: new MemoryStore(),
    runtimeProvider: new LocalDockerProvider({
      rootDir: root,
      baseUrl: "http://127.0.0.1:8787",
      execute: false
    }),
    pricing: {
      serverHourly: { basic: 1, pro: 4 },
      diskGbMonth: 0.2,
      markup: 0.2
    }
  });
  const { origin, close } = await listen(createRequestHandler({ appService }));
  try {
    await appService.manualTopUp({ accountId: "pi-route", amount: 250, reason: "route_test_credit" });
    const workspace = await appService.createWorkspace({
      accountId: "pi-route",
      workspaceName: "Route Lab",
      packageId: "basic"
    });

    const invalidResponse = await fetch(`${origin}/workspaces/${workspace.slug}?token=wrong`);
    assert.equal(invalidResponse.status, 403);

    const validResponse = await fetch(`${origin}/workspaces/${workspace.slug}?token=${workspace.access.token}`);
    const html = await validResponse.text();
    assert.equal(validResponse.status, 200);
    assert.match(html, /Route Lab/);
    assert.match(html, /OPL Workspace/);
    assert.match(html, /docker-compose\.yml|runtime target/);
  } finally {
    await close();
    await rm(root, { recursive: true, force: true });
  }
});

test("workspace gateway validates URL token, sets scoped cookie, and proxies WebUI assets", async () => {
  const upstreamRequests = [];
  const upstream = await listen((request, response) => {
    upstreamRequests.push(request.url);
    if (request.url === "/") {
      response.writeHead(200, { "content-type": "text/html; charset=utf-8" });
      response.end('<!doctype html><script type="module" src="./assets/app.js"></script>');
      return;
    }
    if (request.url === "/assets/app.js") {
      response.writeHead(200, { "content-type": "text/javascript; charset=utf-8" });
      response.end("window.__OPL_WORKSPACE_LOADED__ = true;");
      return;
    }
    if (request.url === "/api/chat?model=gpt") {
      response.writeHead(200, { "content-type": "application/json; charset=utf-8" });
      response.end(JSON.stringify({ ok: true }));
      return;
    }
    response.writeHead(404, { "content-type": "text/plain; charset=utf-8" });
    response.end("missing");
  });
  const appService = {
    async resolveWorkspaceAccess({ workspaceId, token }) {
      if (workspaceId !== "ws-gateway001") throw new Error("workspace_not_found");
      if (token !== "share_gateway") throw new Error("workspace_token_invalid");
      return {
        id: workspaceId,
        state: "running",
        server: { status: "running" },
        access: { tokenStatus: "active" },
        docker: { localUrl: upstream.origin }
      };
    }
  };
  const { origin, close } = await listen(createRequestHandler({ appService }));
  try {
    const blockedAsset = await fetch(`${origin}/w/ws-gateway001/assets/app.js`);
    assert.equal(blockedAsset.status, 403);

    const redirect = await fetch(`${origin}/w/ws-gateway001?token=share_gateway`, { redirect: "manual" });
    const cookie = redirect.headers.get("set-cookie").split(";")[0];
    assert.equal(redirect.status, 308);
    assert.equal(redirect.headers.get("location"), "/w/ws-gateway001/?token=share_gateway");

    const htmlResponse = await fetch(`${origin}/w/ws-gateway001/?token=share_gateway`);
    const html = await htmlResponse.text();
    assert.equal(htmlResponse.status, 200);
    assert.match(html, /assets\/app\.js/);

    const assetResponse = await fetch(`${origin}/w/ws-gateway001/assets/app.js`, {
      headers: { cookie }
    });
    assert.equal(assetResponse.status, 200);
    assert.equal(assetResponse.headers.get("content-type"), "text/javascript; charset=utf-8");
    assert.match(await assetResponse.text(), /OPL_WORKSPACE_LOADED/);

    const apiResponse = await fetch(`${origin}/w/ws-gateway001/api/chat?model=gpt`, {
      headers: { cookie }
    });
    assert.equal(apiResponse.status, 200);
    assert.deepEqual(await apiResponse.json(), { ok: true });
    assert.deepEqual(upstreamRequests, ["/", "/assets/app.js", "/api/chat?model=gpt"]);
  } finally {
    await close();
    await upstream.close();
  }
});

test("runtime readiness route reports provider execution gaps without creating resources", async () => {
  const appService = {
    runtimeReadiness: async () => ({
      provider: "tencent-tke",
      ready: false,
      missingEnv: ["OPL_WORKSPACE_STORAGE_CLASS"],
      missingTools: ["kubectl"]
    })
  };
  const { origin, close } = await listen(createRequestHandler({ appService }));
  try {
    const response = await fetch(`${origin}/api/runtime/readiness`);
    const payload = await response.json();

    assert.equal(response.status, 200);
    assert.deepEqual(payload, {
      provider: "tencent-tke",
      ready: false,
      missingEnv: ["OPL_WORKSPACE_STORAGE_CLASS"],
      missingTools: ["kubectl"]
    });
  } finally {
    await close();
  }
});

test("production readiness route reports launch blockers without creating resources", async () => {
  const appService = {
    productionReadiness: async () => ({
      ready: false,
      missingEnv: ["DATABASE_URL"],
      missingTools: ["kubectl"],
      failedChecks: ["database_url", "tools"],
      checks: []
    })
  };
  const { origin, close } = await listen(createRequestHandler({ appService }));
  try {
    const response = await fetch(`${origin}/api/production/readiness`);
    const payload = await response.json();

    assert.equal(response.status, 200);
    assert.deepEqual(payload, {
      ready: false,
      missingEnv: ["DATABASE_URL"],
      missingTools: ["kubectl"],
      failedChecks: ["database_url", "tools"],
      checks: []
    });
  } finally {
    await close();
  }
});

test("runtime status route returns structured Workspace resource evidence without mutating resources", async () => {
  const requests = [];
  const appService = {
    runtimeStatus: async (input) => {
      requests.push(input);
      return {
        provider: "tencent-tke",
        workspaceId: input.workspaceId,
        ready: true,
        checks: [
          { name: "deployment_ready", ok: true },
          { name: "pvc_bound", ok: true },
          { name: "ingress_routes_workspace_gateway", ok: true }
        ]
      };
    }
  };
  const { origin, close } = await listen(createRequestHandler({ appService }));
  try {
    const response = await fetch(`${origin}/api/workspaces/runtime-status`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ accountId: "pi-route", workspaceId: "ws-route001" })
    });
    const payload = await response.json();

    assert.equal(response.status, 200);
    assert.deepEqual(requests, [{ accountId: "pi-route", workspaceId: "ws-route001" }]);
    assert.deepEqual(payload, {
      provider: "tencent-tke",
      workspaceId: "ws-route001",
      ready: true,
      checks: [
        { name: "deployment_ready", ok: true },
        { name: "pvc_bound", ok: true },
        { name: "ingress_routes_workspace_gateway", ok: true }
      ]
    });
  } finally {
    await close();
  }
});

test("operator summary route returns notification and failed operation aggregates without tokens", async () => {
  const appService = {
    operatorSummary: async (input) => ({
      product: "OPL Console",
      accountScope: input.accountId,
      workspaces: { total: 1, running: 0, needsAttention: 1 },
      notifications: {
        total: 1,
        error: 1,
        warning: 0,
        recent: [
          {
            id: "notification-1",
            accountId: "pi-route",
            workspaceId: "ws-route001",
            type: "workspace.create_failed",
            severity: "error",
            message: "image_pull_failed",
            createdAt: "2026-07-01T00:00:00.000Z"
          }
        ]
      },
      runtimeOperations: {
        total: 1,
        failed: 1,
        recentFailed: [
          {
            id: "op-1",
            accountId: "pi-route",
            workspaceId: "ws-route001",
            operationType: "create_workspace",
            error: "image_pull_failed",
            updatedAt: "2026-07-01T00:00:00.000Z"
          }
        ]
      }
    })
  };
  const { origin, close } = await listen(createRequestHandler({ appService, operatorSummaryToken: "operator-test-token" }));
  try {
    const blockedResponse = await fetch(`${origin}/api/operator/summary?accountId=pi-route`);
    assert.equal(blockedResponse.status, 403);

    const response = await fetch(`${origin}/api/operator/summary?accountId=pi-route`, {
      headers: { "x-opl-operator-token": "operator-test-token" }
    });
    const payload = await response.json();

    assert.equal(response.status, 200);
    assert.equal(payload.accountScope, "pi-route");
    assert.equal(payload.notifications.error, 1);
    assert.equal(payload.runtimeOperations.failed, 1);
    assert.equal(JSON.stringify(payload).includes("share_"), false);
  } finally {
    await close();
  }
});
