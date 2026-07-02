import assert from "node:assert/strict";
import { once } from "node:events";
import { createServer } from "node:http";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";

import { createAuthController, createRequestHandler } from "../../packages/console/api/server.js";

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

async function postJson(origin, path, body, headers = {}) {
  const response = await fetch(`${origin}${path}`, {
    method: "POST",
    headers: { "content-type": "application/json", ...headers },
    body: JSON.stringify(body)
  });
  return { response, payload: await response.json() };
}

function cookieFrom(response) {
  return response.headers.get("set-cookie")?.split(";")[0] || "";
}

test("commercial Console login creates a session and logout invalidates it", async () => {
  const root = await mkdtemp(join(tmpdir(), "opl-auth-"));
  const appService = {
    async getState(accountId) {
      return { account: { id: accountId }, workspaces: [], packages: [], billingLedger: [], audit: [], notifications: [] };
    }
  };
  const auth = createAuthController({
    usersPath: join(root, "users.json"),
    seedUsers: [
      {
        id: "usr-pi",
        email: "pi@example.com",
        password: "secret-pi",
        name: "Pilot PI",
        role: "pi",
        accountId: "pi-alpha"
      }
    ]
  });
  const { origin, close } = await listen(createRequestHandler({ appService, auth }));
  try {
    const login = await postJson(origin, "/api/auth/login", {
      email: "pi@example.com",
      password: "secret-pi"
    });

    assert.equal(login.response.status, 200);
    assert.equal(login.payload.user.accountId, "pi-alpha");
    assert.equal(login.payload.user.email, "pi@example.com");
    assert.ok(login.payload.csrfToken);
    assert.match(login.response.headers.get("set-cookie"), /HttpOnly/);

    const cookie = cookieFrom(login.response);
    const meResponse = await fetch(`${origin}/api/auth/me`, { headers: { cookie } });
    const me = await meResponse.json();
    assert.equal(meResponse.status, 200);
    assert.equal(me.user.id, "usr-pi");

    const logout = await postJson(origin, "/api/auth/logout", {}, {
      cookie,
      "x-opl-csrf": login.payload.csrfToken
    });
    assert.equal(logout.response.status, 200);
    assert.match(logout.response.headers.get("set-cookie"), /Max-Age=0/);

    const afterLogout = await fetch(`${origin}/api/auth/me`, { headers: { cookie } });
    assert.equal(afterLogout.status, 401);
  } finally {
    await close();
    await rm(root, { recursive: true, force: true });
  }
});

test("PI sessions are account scoped and cannot top up balances", async () => {
  const root = await mkdtemp(join(tmpdir(), "opl-auth-scope-"));
  const calls = [];
  const appService = {
    async getState(accountId) {
      calls.push(["getState", accountId]);
      return { account: { id: accountId }, workspaces: [], packages: [], billingLedger: [], audit: [], notifications: [] };
    },
    async creditAccount(input) {
      calls.push(["creditAccount", input]);
      return { id: input.accountId, balance: input.amount, frozen: 0 };
    }
  };
  const auth = createAuthController({
    usersPath: join(root, "users.json"),
    seedUsers: [
      {
        id: "usr-pi",
        email: "pi@example.com",
        password: "secret-pi",
        name: "Pilot PI",
        role: "pi",
        accountId: "pi-alpha"
      },
      {
        id: "usr-admin",
        email: "admin@example.com",
        password: "secret-admin",
        name: "Admin",
        role: "admin",
        accountId: "admin"
      }
    ]
  });
  const { origin, close } = await listen(createRequestHandler({ appService, auth }));
  try {
    const piLogin = await postJson(origin, "/api/auth/login", {
      email: "pi@example.com",
      password: "secret-pi"
    });
    const piCookie = cookieFrom(piLogin.response);

    const stateResponse = await fetch(`${origin}/api/state?accountId=pi-beta`, {
      headers: { cookie: piCookie }
    });
    const state = await stateResponse.json();
    assert.equal(stateResponse.status, 200);
    assert.equal(state.account.id, "pi-alpha");

    const piCredit = await postJson(origin, "/api/accounts/credit", {
      accountId: "pi-alpha",
      amount: 200,
      reason: "manual_top_up"
    }, {
      cookie: piCookie,
      "x-opl-csrf": piLogin.payload.csrfToken
    });
    assert.equal(piCredit.response.status, 403);
    assert.equal(piCredit.payload.error, "admin_role_required");

    const adminLogin = await postJson(origin, "/api/auth/login", {
      email: "admin@example.com",
      password: "secret-admin"
    });
    const adminCredit = await postJson(origin, "/api/accounts/credit", {
      accountId: "pi-alpha",
      amount: 200,
      reason: "manual_top_up"
    }, {
      cookie: cookieFrom(adminLogin.response),
      "x-opl-csrf": adminLogin.payload.csrfToken
    });
    assert.equal(adminCredit.response.status, 200);
    assert.deepEqual(calls, [
      ["getState", "pi-alpha"],
      ["creditAccount", { accountId: "pi-alpha", amount: 200, reason: "manual_top_up" }]
    ]);
  } finally {
    await close();
    await rm(root, { recursive: true, force: true });
  }
});
