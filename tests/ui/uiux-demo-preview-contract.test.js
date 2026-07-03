import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import {
  uiuxDemoAccounts,
  uiuxDemoAuthSeedJson
} from "../../tools/uiux-demo-fixture.js";

const repoRoot = new URL("../../", import.meta.url);

async function source(relativePath) {
  return readFile(new URL(relativePath, repoRoot), "utf8");
}

test("UIUX demo fixture provides stable owner and admin accounts", () => {
  assert.deepEqual(uiuxDemoAccounts.map((account) => `${account.label}:${account.email}:${account.password}`), [
    "Lab Owner:owner@opl.local:OplOwnerPass2026!",
    "Admin:admin@opl.local:OplAdminPass2026!"
  ]);

  const authSeed = JSON.parse(uiuxDemoAuthSeedJson());
  assert.deepEqual(authSeed.map((account) => account.role), ["pi", "admin"]);
  assert.equal(authSeed[0].accountId, "acct-owner-uiux");
  assert.equal(authSeed[1].accountId, "admin");
});

test("login page keeps one credential form without role shortcuts", async () => {
  const loginSource = await source("packages/console/ui/pages/LoginPage.jsx");
  const demoUiSource = await source("tools/start-uiux-demo-ui.js");
  const packageSource = JSON.parse(await source("package.json"));

  assert.doesNotMatch(loginSource, /demoAccounts|demoLoginPanel|UserRound|runtimeConfig/, "login must not expose role/demo account shortcuts");
  assert.doesNotMatch(demoUiSource, /VITE_OPL_DEMO_ACCOUNTS_JSON|VITE_OPL_DEMO_MODE/, "demo UI must not inject account choices into the browser");
  assert.equal(packageSource.scripts["demo:api"], "node tools/start-uiux-demo-api.js");
  assert.equal(packageSource.scripts["demo:ui"], "node tools/start-uiux-demo-ui.js");
});
