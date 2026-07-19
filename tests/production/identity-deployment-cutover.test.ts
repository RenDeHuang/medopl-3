import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

import { productionManifestRequiredEnv } from "../../services/control-plane/ops/production-manifest.ts";

const retiredInput = "OPL_CONSOLE_USERS_JSON";

test("production deployment no longer injects the retired local Console user seed", async () => {
  const paths = [
    ".github/workflows/deploy-tke-production.yml",
    ".github/workflows/verify-production-chain.yml",
    "deploy/tke/opl-cloud.k8s.json",
    "deploy/production-manifest.example.json",
    "deploy/tke/opl-cloud-production.env.example",
    "deploy/tke/opl-cloud-staging.local.env.example",
    ".env.example",
    "services/control-plane/ops/production-manifest.ts",
    "services/fabric/ops/production-readiness.ts"
  ];
  for (const path of paths) {
    assert.doesNotMatch(await readFile(path, "utf8"), new RegExp(retiredInput), path);
  }
  assert.equal(productionManifestRequiredEnv().includes(retiredInput), false);
});
