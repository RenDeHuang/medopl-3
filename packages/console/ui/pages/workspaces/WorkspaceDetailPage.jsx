import React from "react";
import { Button, Empty, Typography } from "antd";
import { Cable, HardDrive, Link as LinkIcon, RefreshCw, Server, Trash2 } from "lucide-react";
import {
  deleteWorkspaceToken,
  resetWorkspaceToken
} from "../../api/workspaces-api.js";
import { navigate, routeTo } from "../../consoleRoutes.js";
import {
  ActionGroup,
  ConsoleSurface,
  DataRetentionPolicyPanel,
  InsightPanel,
  ResourceSplit,
  StatusPill
} from "../shared/commercial-console.jsx";
import { packageText, statusColor, statusLabel, valueLabel } from "../shared/formatters.js";

function toneForStatus(value) {
  const color = statusColor(value);
  if (color === "green") return "good";
  if (color === "red") return "danger";
  if (color === "orange") return "warn";
  return "info";
}

function WorkspaceLifecyclePanel({ workspace, compute, storage, attachment }) {
  return (
    <InsightPanel title="访问生命周期" eyebrow="生命周期">
      <ResourceSplit
        items={[
          { label: "URL 状态", value: workspace.access?.tokenStatus || "-", meta: "访问状态", status: workspace.access?.tokenStatus || "未知", tone: workspace.access?.tokenStatus === "active" ? "good" : "warn" },
          { label: "计算资源", value: compute?.status || "缺失", meta: workspace.computeAllocationId, status: "计算", tone: toneForStatus(compute?.status) },
          { label: "存储资源", value: storage?.status || "缺失", meta: workspace.storageId, status: "存储", tone: toneForStatus(storage?.status) },
          { label: "挂载状态", value: attachment?.status || "缺失", meta: attachment?.mountPath || "/data", status: "挂载", tone: toneForStatus(attachment?.status) }
        ]}
      />
    </InsightPanel>
  );
}

export function WorkspaceDetailPage({ selected, selectedPlan, state, session, runAction }) {
  if (!selected) {
    return (
      <ConsoleSurface title="工作区入口" eyebrow="交付">
        <Empty description="暂无工作区入口" />
      </ConsoleSurface>
    );
  }
  const computeAllocationId = selected.computeAllocationId;
  const compute = (state.computeAllocations || []).find((item) => item.id === computeAllocationId);
  const storage = (state.storageVolumes || []).find((item) => item.id === selected.storageId);
  const attachment = (state.storageAttachments || []).find((item) => item.id === selected.attachmentId);
  return (
    <ConsoleSurface
      title={selected.name}
      eyebrow="工作区详情"
      subtitle="访问 URL 已绑定计算、存储和挂载关系"
      extra={<Button onClick={() => navigate(routeTo("workspace.list"))}>返回列表</Button>}
    >
      <div className="consoleGrid equal">
        <InsightPanel
          title="访问 URL"
          eyebrow="访问"
          actions={<StatusPill label={valueLabel(selected.access?.tokenStatus)} tone={selected.access?.tokenStatus === "active" ? "good" : "warn"} />}
        >
          <div className="stackList">
            <Typography.Text copyable={selected.access?.tokenStatus === "active"} className="inlineCode">{selected.url}</Typography.Text>
            <ActionGroup
              actions={[
                { label: "打开", icon: <LinkIcon size={15} />, disabled: selected.access?.tokenStatus !== "active", onClick: () => window.open(selected.url, "_blank", "noopener,noreferrer") },
                { label: "重置", icon: <RefreshCw size={15} />, disabled: selected.access?.tokenStatus !== "active", onClick: () => runAction(() => resetWorkspaceToken({ workspaceId: selected.id }, session.csrfToken), "URL 已重置") },
                { label: "停用", danger: true, icon: <Trash2 size={15} />, disabled: selected.access?.tokenStatus !== "active", onClick: () => runAction(() => deleteWorkspaceToken({ workspaceId: selected.id }, session.csrfToken), "URL 已停用") }
              ]}
            />
          </div>
        </InsightPanel>

        <InsightPanel title="计算与存储" eyebrow="资源">
          <ResourceSplit
            items={[
              { label: "状态", value: statusLabel(selected), meta: selected.state, status: "工作区", tone: toneForStatus(selected.state) },
              { label: "套餐", value: selectedPlan?.name || "-", meta: packageText(selectedPlan), status: "套餐", tone: "info" },
              { label: "计算", value: compute?.name || computeAllocationId || "-", meta: valueLabel(compute?.status || selected.server?.status), status: "计算", tone: toneForStatus(compute?.status || selected.server?.status) },
              { label: "存储", value: storage?.name || selected.storageId || "-", meta: `${selected.disk?.sizeGb || storage?.sizeGb || 0}GB`, status: "存储", tone: toneForStatus(storage?.status || selected.disk?.status) },
              { label: "挂载关系", value: attachment?.id || selected.attachmentId || "-", meta: attachment?.mountPath || selected.disk?.mountPath || "/data", status: "挂载", tone: toneForStatus(attachment?.status) }
            ]}
          />
        </InsightPanel>
      </div>

      <div className="consoleGrid equal">
        <InsightPanel title="资源对象" eyebrow="计算、存储、挂载">
          <ActionGroup
            actions={[
              { label: "查看计算分配", icon: <Server size={15} />, disabled: !computeAllocationId, onClick: () => navigate(routeTo("compute-allocations.detail", { id: computeAllocationId })) },
              { label: "查看存储资源", icon: <HardDrive size={15} />, disabled: !selected.storageId, onClick: () => navigate(routeTo("storage.detail", { id: selected.storageId })) },
              { label: "查看挂载关系", icon: <Cable size={15} />, disabled: !selected.attachmentId, onClick: () => navigate(routeTo("attachment.detail", { id: selected.attachmentId })) }
            ]}
          />
        </InsightPanel>
        <WorkspaceLifecyclePanel workspace={selected} compute={compute} storage={storage} attachment={attachment} />
      </div>
      <DataRetentionPolicyPanel />
    </ConsoleSurface>
  );
}
