import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const repoRoot = new URL("../../", import.meta.url);

async function source(relativePath) {
  return readFile(new URL(relativePath, repoRoot), "utf8");
}

test("package scripts expose separate demo, staging-local, and cloud verifier entrypoints", async () => {
  const packageSource = JSON.parse(await source("package.json"));

  assert.equal(packageSource.scripts["demo:api"], "node tools/start-uiux-demo-api.js");
  assert.equal(packageSource.scripts["demo:ui"], "node tools/start-uiux-demo-ui.js");
  assert.equal(packageSource.scripts["staging:local"], "node tools/start-staging-local-api.js");
  assert.equal(packageSource.scripts["staging:ui"], "node tools/start-staging-local-ui.js");
  assert.equal(packageSource.scripts["staging:readiness"], "node tools/staging-readiness.js");
  assert.equal(packageSource.scripts["staging:e2e"], "node tools/staging-local-verifier.js");
  assert.equal(packageSource.scripts["verify:production"], "node tools/production-verifier.js");
});

test("UIUX demo API refuses Tencent TKE so demo cannot mutate staging resources", async () => {
  const demoApiSource = await source("tools/start-uiux-demo-api.js");

  assert.match(demoApiSource, /uiux_demo_refuses_real_tke/, "demo API must fail closed when OPL_RUNTIME_PROVIDER=tencent-tke");
  assert.doesNotMatch(demoApiSource, /assertRuntimeReadyForProvisioning/, "real TKE readiness belongs to staging scripts, not demo API");
  assert.doesNotMatch(demoApiSource, /runtimeReadiness/, "demo API must not preflight or seed real TKE resources");
});

test("staging-local scripts load ignored staging env files and require explicit paid E2E confirmation", async () => {
  const localApiSource = await source("tools/start-staging-local-api.js");
  const readinessSource = await source("tools/staging-readiness.js");
  const verifierSource = await source("tools/staging-local-verifier.js");
  const envExample = await source("deploy/tke/opl-cloud-staging.local.env.example");

  assert.match(localApiSource, /loadEnvFile/, "local staging API must load .env.staging.local");
  assert.match(localApiSource, /validateStagingLocalEnv/, "local staging API must validate cloud env before serving");
  assert.match(readinessSource, /validateStagingLocalEnv/, "readiness must use the same env contract");
  assert.match(verifierSource, /OPL_CONFIRM_REAL_CLOUD_E2E/, "real cloud E2E must require explicit confirmation");
  assert.match(verifierSource, /allowPrivateConsoleOrigin: true/, "local-to-staging verifier may use a private local Console origin");
  assert.match(envExample, /OPL_RUNTIME_PROVIDER=tencent-tke/);
  assert.match(envExample, /OPL_CONFIRM_REAL_CLOUD_E2E=0/);
});
