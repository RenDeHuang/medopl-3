import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import * as authApi from "../../apps/console-ui/src/api/auth-api.ts";
import { maskGatewaySummary } from "../../apps/console-ui/src/console-model.ts";

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
});

test("logout clears the local session before the remote request settles", async () => {
  assert.equal(typeof authApi.logoutLocalFirst, "function");

  let settle: (response: Response) => void = () => {};
  const remote = new Promise<Response>((resolve) => { settle = resolve; });
  globalThis.fetch = async () => remote;
  const events: string[] = [];

  const pending = authApi.logoutLocalFirst(
    "csrf-alpha",
    () => events.push("local-cleared"),
    () => events.push("navigated")
  );
  assert.deepEqual(events, ["local-cleared", "navigated"]);

  settle(new Response(JSON.stringify({ ok: true }), {
    status: 200,
    headers: { "content-type": "application/json" }
  }));
  await pending;
});

test("Gateway cleanup removes raw keys", () => {
  const revealed = { apiKey: { id: "key-alpha", revealed: true, value: "sk-raw", maskedValue: "sk-****" } };
  assert.deepEqual(maskGatewaySummary(revealed), {
    apiKey: { id: "key-alpha", revealed: false, maskedValue: "sk-****" }
  });
});
