import { spawn } from "node:child_process";
import { access } from "node:fs/promises";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

function compactId(value) {
  return String(value)
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-|-$/g, "")
    .slice(0, 48);
}

async function defaultRunner({ command, args, cwd, env }) {
  return new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd, env: { ...process.env, ...env }, stdio: "pipe" });
    let stdout = "";
    let stderr = "";
    child.stdout.on("data", (chunk) => {
      stdout += chunk.toString();
    });
    child.stderr.on("data", (chunk) => {
      stderr += chunk.toString();
    });
    child.on("error", reject);
    child.on("close", (code) => {
      if (code === 0) resolve(stdout.trim());
      else reject(new Error(`${command} ${args.join(" ")} failed: ${stderr.trim()}`));
    });
  });
}

async function defaultCommandExists(command) {
  const paths = String(process.env.PATH || "").split(":").filter(Boolean);
  for (const path of paths) {
    try {
      await access(join(path, command));
      return true;
    } catch {
      // Try the next PATH entry.
    }
  }
  return false;
}

function outputValue(outputs, key) {
  const value = outputs?.[key];
  if (value && typeof value === "object" && "value" in value) return value.value;
  return value;
}

export class TencentCvmProvider {
  constructor({
    env = process.env,
    runner = defaultRunner,
    commandExists = defaultCommandExists,
    infraDir = join(dirname(fileURLToPath(import.meta.url)), "../../../..", "infra", "tencent-cvm")
  } = {}) {
    this.name = "tencent-cvm";
    this.env = env;
    this.runner = runner;
    this.commandExists = commandExists;
    this.infraDir = infraDir;
  }

  async createWorkspaceRuntime({ workspaceId, ownerAccountId = "unknown", workspaceName, packagePlan, token }) {
    this.requireExecutionBoundary();
    await this.requireTools(["tofu", "ansible-playbook"]);

    const slug = compactId(workspaceName);
    const oplImage = this.env.OPL_WORKSPACE_IMAGE || "ghcr.io/gaofeng21cn/one-person-lab-webui:latest";
    const common = {
      cwd: this.infraDir,
      env: this.env
    };
    const vars = [
      ["workspace_id", workspaceId],
      ["workspace_slug", slug],
      ["workspace_token", token],
      ["workspace_domain", this.env.OPL_WORKSPACE_DOMAIN],
      ["owner_account_id", this.env.OPL_OWNER_ACCOUNT_ID || ownerAccountId],
      ["package_id", packagePlan.id],
      ["opl_image", oplImage],
      ["region", this.env.TENCENTCLOUD_REGION],
      ["availability_zone", this.env.OPL_AVAILABILITY_ZONE],
      ["image_id", this.env.OPL_IMAGE_ID],
      ["vpc_id", this.env.OPL_VPC_ID],
      ["subnet_id", this.env.OPL_SUBNET_ID],
      ["security_group_id", this.env.OPL_SECURITY_GROUP_ID],
      ["key_id", this.env.OPL_SSH_KEY_ID || ""]
    ].flatMap(([key, value]) => ["-var", `${key}=${value}`]);

    await this.runner({ command: "tofu", args: ["init", "-input=false"], ...common });
    await this.runner({ command: "tofu", args: ["apply", "-auto-approve", "-input=false", ...vars], ...common });
    const rawOutputs = await this.runner({ command: "tofu", args: ["output", "-json"], ...common });
    const outputs = JSON.parse(rawOutputs);
    const serverId = outputValue(outputs, "server_id");
    const diskId = outputValue(outputs, "disk_id");
    const publicIp = outputValue(outputs, "public_ip");
    const url = outputValue(outputs, "workspace_url");
    if (!serverId || !diskId || !publicIp || !url) {
      throw new Error("tencent_cvm_provider_incomplete_outputs");
    }

    await this.runner({
      command: "ansible-playbook",
      args: [
        "-i",
        `${publicIp},`,
        "ansible/workspace.yml",
        "-u",
        "root",
        "--extra-vars",
        [
          `workspace_id=${workspaceId}`,
          `workspace_slug=${slug}`,
          `workspace_token=${token}`,
          `workspace_domain=${this.env.OPL_WORKSPACE_DOMAIN}`,
          `opl_image=${oplImage}`
        ].join(" ")
      ],
      ...common
    });

    return {
      provider: this.name,
      server: {
        id: serverId,
        status: "running",
        billingStatus: "active",
        spec: packagePlan.server,
        publicIp
      },
      docker: {
        id: `docker-${workspaceId}`,
        image: oplImage,
        status: "running",
        remoteHost: publicIp
      },
      disk: {
        id: diskId,
        status: "attached_retained",
        billingStatus: "active",
        sizeGb: packagePlan.diskGb,
        mountPath: "/data"
      },
      url,
      slug
    };
  }

  async stopServer() {
    throw new Error("tencent_cvm_provider_not_configured");
  }

  async restartServer() {
    throw new Error("tencent_cvm_provider_not_configured");
  }

  async destroyServer() {
    throw new Error("tencent_cvm_provider_not_configured");
  }

  async destroyDisk() {
    throw new Error("tencent_cvm_provider_not_configured");
  }

  requireExecutionBoundary() {
    const required = [
      "TENCENTCLOUD_SECRET_ID",
      "TENCENTCLOUD_SECRET_KEY",
      "TENCENTCLOUD_REGION",
      "OPL_WORKSPACE_DOMAIN",
      "OPL_VPC_ID",
      "OPL_SUBNET_ID",
      "OPL_SECURITY_GROUP_ID"
    ];
    const missing = required.filter((key) => !this.env[key]);
    if (missing.length > 0) {
      throw new Error(`tencent_cvm_provider_missing_env:${missing.join(",")}`);
    }
  }

  async requireTools(commands) {
    const missing = [];
    for (const command of commands) {
      if (!(await this.commandExists(command))) missing.push(command);
    }
    if (missing.length > 0) {
      throw new Error(`tencent_cvm_provider_missing_tools:${missing.join(",")}`);
    }
  }
}
