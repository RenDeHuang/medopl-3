import { access } from "node:fs/promises";
import { constants } from "node:fs";
import { delimiter, join } from "node:path";

const REQUIRED_TENCENT_ENV = [
  "TENCENTCLOUD_SECRET_ID",
  "TENCENTCLOUD_SECRET_KEY",
  "TENCENTCLOUD_REGION",
  "OPL_VPC_ID",
  "OPL_SUBNET_ID",
  "OPL_SECURITY_GROUP_ID",
  "OPL_AVAILABILITY_ZONE",
  "OPL_IMAGE_ID",
  "OPL_SSH_KEY_ID"
];

const REQUIRED_TOOLS = ["tofu", "ansible-playbook", "tccli", "caddy"];

function check(id, ok, message) {
  return { id, ok, message };
}

function normalizeRegistry(registry) {
  return String(registry || "").replace(/^https?:\/\//, "").replace(/\/$/, "");
}

function looksLikeHarborImage({ image, registry }) {
  const normalizedRegistry = normalizeRegistry(registry);
  return Boolean(
    image &&
    normalizedRegistry &&
    image.startsWith(`${normalizedRegistry}/`) &&
    image.includes("/") &&
    image.includes(":")
  );
}

function looksLikeProductionDomain(domain) {
  return Boolean(domain && domain.includes(".") && !domain.includes("localhost") && !domain.startsWith("127."));
}

function matchesOplAppContract(env) {
  return (
    String(env.OPL_WORKSPACE_WEBUI_PORT || "3000") === "3000" &&
    String(env.OPL_WORKSPACE_DATA_DIR || "/data") === "/data" &&
    String(env.OPL_WORKSPACE_PROJECTS_DIR || "/projects") === "/projects"
  );
}

async function commandExistsInPath(command, env) {
  const pathValue = env.PATH || process.env.PATH || "";
  for (const dir of pathValue.split(delimiter).filter(Boolean)) {
    try {
      await access(join(dir, command), constants.X_OK);
      return true;
    } catch {
      // Keep scanning PATH.
    }
  }
  return false;
}

export async function productionReadiness({ env = process.env, commandExists = (command) => commandExistsInPath(command, env) } = {}) {
  const missingEnv = [];
  const missingTools = [];

  const requiredEnv = [
    "OPL_RUNTIME_PROVIDER",
    "OPL_HARBOR_REGISTRY",
    "OPL_WORKSPACE_IMAGE",
    "OPL_WORKSPACE_DOMAIN",
    "DATABASE_URL",
    "OPENMETER_ENDPOINT",
    "OPENMETER_API_KEY",
    ...REQUIRED_TENCENT_ENV
  ];
  for (const key of requiredEnv) {
    if (!env[key]) missingEnv.push(key);
  }
  for (const tool of REQUIRED_TOOLS) {
    if (!(await commandExists(tool))) missingTools.push(tool);
  }

  const checks = [
    check("runtime_provider", env.OPL_RUNTIME_PROVIDER === "tencent-cvm", "OPL_RUNTIME_PROVIDER must be tencent-cvm"),
    check(
      "harbor_image",
      looksLikeHarborImage({ image: env.OPL_WORKSPACE_IMAGE, registry: env.OPL_HARBOR_REGISTRY }),
      "OPL_WORKSPACE_IMAGE must point to the configured Harbor production registry"
    ),
    check("opl_app_contract", matchesOplAppContract(env), "one-person-lab-app WebUI must expose port 3000 and persist /data plus /projects"),
    check("workspace_domain", looksLikeProductionDomain(env.OPL_WORKSPACE_DOMAIN), "OPL_WORKSPACE_DOMAIN must be a production wildcard domain"),
    check("database_url", Boolean(env.DATABASE_URL), "DATABASE_URL is required for production persistence"),
    check("openmeter", Boolean(env.OPENMETER_ENDPOINT && env.OPENMETER_API_KEY), "OpenMeter endpoint and API key are required"),
    check("tencent_env", REQUIRED_TENCENT_ENV.every((key) => Boolean(env[key])), "Tencent cloud environment is incomplete"),
    check("tools", missingTools.length === 0, "Required production tools are missing")
  ];
  const failedChecks = checks.filter((item) => !item.ok).map((item) => item.id);

  return {
    ready: missingEnv.length === 0 && missingTools.length === 0 && failedChecks.length === 0,
    missingEnv,
    missingTools,
    failedChecks,
    checks
  };
}
