import { createServer } from "node:http";
import { join } from "node:path";
import { fileURLToPath } from "node:url";

import { buildTencentProvisioner, defaultProvisionerBin } from "./build-tencent-provisioner.js";
import {
  applyEnv,
  defaultStagingEnvPath,
  loadEnvFile,
  validateStagingLocalEnv
} from "./staging-env.js";

const root = fileURLToPath(new URL("..", import.meta.url));
const envFile = process.env.OPL_STAGING_ENV_FILE || defaultStagingEnvPath;
let loadedEnv = null;
try {
  loadedEnv = loadEnvFile({ filePath: envFile, baseEnv: process.env });
} catch (error) {
  console.error(JSON.stringify({
    ok: false,
    error: error.message,
    envFile
  }, null, 2));
  process.exit(1);
}

loadedEnv.OPL_TENCENT_PROVISIONER_BIN ||= defaultProvisionerBin;
loadedEnv.PORT ||= "8787";
loadedEnv.OPL_PUBLIC_URL ||= `http://127.0.0.1:${loadedEnv.PORT}`;
applyEnv(loadedEnv);

await buildTencentProvisioner({ binPath: process.env.OPL_TENCENT_PROVISIONER_BIN });

const envReport = validateStagingLocalEnv(process.env);
if (!envReport.ready) {
  console.error(JSON.stringify({
    ok: false,
    error: "staging_local_env_not_ready",
    envFile,
    ...envReport
  }, null, 2));
  process.exit(1);
}

const {
  appStore,
  createAuthController,
  createRequestHandler,
  createUpgradeHandler
} = await import("../packages/console/api/server.js");

const auth = createAuthController({ env: process.env, store: appStore });
const server = createServer(createRequestHandler({ auth }));
server.on("upgrade", createUpgradeHandler());
server.listen(Number(process.env.PORT), () => {
  console.log(`OPL Console local-to-staging API listening on http://127.0.0.1:${process.env.PORT}`);
  console.log(`Env file: ${envFile}`);
  console.log(`Runtime provider: ${process.env.OPL_RUNTIME_PROVIDER}`);
  console.log(`Database: ${process.env.DATABASE_URL ? "staging PostgreSQL configured" : "missing"}`);
  console.log(`Provisioner: ${process.env.OPL_TENCENT_PROVISIONER_BIN}`);
  console.log(`Workspace domain: ${process.env.OPL_WORKSPACE_DOMAIN}`);
  console.log(`Repo: ${root}`);
});
