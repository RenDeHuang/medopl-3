import React from "react";
import { Alert, Button, Empty, Typography } from "antd";
import { Eye, EyeOff, Headphones, Link as LinkIcon, WalletCards } from "lucide-react";
import { getWorkspaceRuntimeStatus } from "../../api/workspaces-api.ts";
import { navigate, routeTo } from "../../consoleRoutes.ts";
import {
  ActionGroup,
  ConsoleSurface,
  InsightPanel,
  ResourceSplit,
  StatusPill
} from "../shared/commercial-console.tsx";
import { mergeWorkspaceRuntime, moneyCents, packageText, paidThrough, statusColor, statusLabel, usdMicros, valueLabel, workspaceAccessLabel, workspaceAccessTone, workspaceOpenActionLabel, workspaceUrlReady } from "../shared/formatters.ts";

type AnyRecord = Record<string, any>;

function toneForStatus(value) {
  const color = statusColor(value);
  if (color === "green") return "good";
  if (color === "red") return "danger";
  if (color === "orange") return "warn";
  return "info";
}

function workspaceCredential(workspace: AnyRecord = {}) {
  return {
    account: workspace.access?.account
      || workspace.access?.username
      || workspace.login?.username
      || workspace.id
      || "-",
    password: workspace.access?.password
      || "未返回"
  };
}

export function WorkspaceDetailPage({ selected, selectedPlan, state, session }: any) {
  const [runtimeStatus, setRuntimeStatus] = React.useState<AnyRecord | null>(null);
  const [showPassword, setShowPassword] = React.useState(false);
  React.useEffect(() => {
    let active = true;
    let timer: number | undefined;
    setRuntimeStatus(null);
    setShowPassword(false);
    if (!selected?.id || ["suspended", "data_deleted", "unrecoverable", "storage_missing", "destroyed"].includes(selected.state)) {
      return () => { active = false; };
    }
    const poll = async () => {
      try {
        const current = await getWorkspaceRuntimeStatus({ workspaceId: selected.id }, session.csrfToken);
        if (!active) return;
        setRuntimeStatus(current);
        if (!current.ready) timer = window.setTimeout(poll, 10_000);
      } catch {
        if (active) timer = window.setTimeout(poll, 10_000);
      }
    };
    void poll();
    return () => {
      active = false;
      if (timer) window.clearTimeout(timer);
    };
  }, [selected?.id, selected?.state, session.csrfToken]);

  if (!selected) {
    return (
      <ConsoleSurface title="OPL Workspace" eyebrow="工作区">
        <Empty description="暂无工作区" />
      </ConsoleSurface>
    );
  }
  const workspace = mergeWorkspaceRuntime(selected, runtimeStatus);
  const credential = workspaceCredential(workspace);
  const computeId = workspace.currentComputeAllocationId || workspace.computeAllocationId;
  const compute = (state.computeAllocations || []).find((item) => item.id === computeId) || {};
  const storage = (state.storageVolumes || []).find((item) => item.id === workspace.storageId) || {};
  const supportPath = `${routeTo("support.create")}?category=Workspace&resourceId=${encodeURIComponent(workspace.id)}&operationId=${encodeURIComponent(workspace.currentAttachmentId || workspace.currentComputeAllocationId || "")}`;
  return (
    <ConsoleSurface
      title={workspace.name}
      eyebrow="OPL Workspace"
      subtitle="访问凭据、费用状态和支持"
      extra={<Button onClick={() => navigate(routeTo("workspace.list"))}>返回列表</Button>}
    >
      <div className="consoleGrid equal">
        <InsightPanel
          title="访问凭据"
          eyebrow="URL、账号、密码"
          actions={<StatusPill label={workspaceAccessLabel(workspace)} tone={workspaceAccessTone(workspace)} />}
        >
          <div className="stackList">
            {!workspaceUrlReady(workspace) && workspace.accessState === "distributing" && (
              <Alert
                type="info"
                showIcon
                message="正在分发 Docker"
                description="访问 URL 已生成，Runtime 仍在部署。通常需要 3-5 分钟，请稍后再打开。"
              />
            )}
            <div className="credentialStack">
              <span>URL</span>
              <Typography.Text copyable={workspaceUrlReady(workspace)} className="inlineCode">{workspace.url}</Typography.Text>
            </div>
            <div className="credentialStack">
              <span>账号</span>
              <Typography.Text copyable className="inlineCode">{credential.account}</Typography.Text>
            </div>
            <div className="credentialStack">
              <span>密码</span>
              <Typography.Text copyable={showPassword} className="inlineCode">{showPassword ? credential.password : "********"}</Typography.Text>
            </div>
            <ActionGroup
              actions={[
                { label: workspaceOpenActionLabel(workspace), icon: <LinkIcon size={15} />, disabled: !workspaceUrlReady(workspace), onClick: () => window.open(workspace.url, "_blank", "noopener,noreferrer") },
                { label: showPassword ? "隐藏密码" : "显示密码", icon: showPassword ? <EyeOff size={15} /> : <Eye size={15} />, onClick: () => setShowPassword(!showPassword) },
                { label: "提交工单", icon: <Headphones size={15} />, onClick: () => navigate(supportPath) }
              ]}
            />
          </div>
        </InsightPanel>

        <InsightPanel title="月度权益" eyebrow="计算与存储">
          <ResourceSplit
            items={[
              { label: "计算权益", value: valueLabel(compute.billingStatus), meta: `有效期至 ${paidThrough(compute.paidThrough)}`, status: compute.autoRenew ? "自动续费" : "到期停止", tone: compute.billingStatus === "active" ? "good" : "warn" },
              { label: "计算月价", value: moneyCents(compute.monthlyPriceCnyCents), meta: `Sub2API ${usdMicros(compute.chargeUsdMicros)}`, status: "月付", tone: "info" },
              { label: "存储权益", value: valueLabel(storage.billingStatus), meta: `有效期至 ${paidThrough(storage.paidThrough)}`, status: storage.autoRenew ? "自动续费" : "到期保留", tone: storage.billingStatus === "active" ? "good" : "warn" },
              { label: "存储月价", value: moneyCents(storage.monthlyPriceCnyCents), meta: `Sub2API ${usdMicros(storage.chargeUsdMicros)}`, status: "月付", tone: "info" },
              { label: "套餐", value: selectedPlan?.name || "-", meta: packageText(selectedPlan), status: "套餐", tone: "info" },
              { label: "状态", value: statusLabel(workspace), meta: workspace.state, status: "Workspace", tone: toneForStatus(workspace.state) },
              { label: "费用明细", value: "账单页", meta: "查看余额与权益", status: "可查看", tone: "good" }
            ]}
          />
          <ActionGroup
            actions={[
              { label: "查看账单", icon: <WalletCards size={15} />, onClick: () => navigate(routeTo("billing.overview")) },
              { label: "提交工单", icon: <Headphones size={15} />, onClick: () => navigate(supportPath) }
            ]}
          />
        </InsightPanel>
      </div>
    </ConsoleSurface>
  );
}
