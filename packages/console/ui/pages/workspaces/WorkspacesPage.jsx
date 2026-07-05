import React from "react";
import { Button, Typography } from "antd";
import { Link as LinkIcon, Plus, RefreshCw, Settings2, Trash2 } from "lucide-react";
import { deleteWorkspaceToken, resetWorkspaceToken } from "../../api/workspaces-api.js";
import { navigate, routeTo } from "../../consoleRoutes.js";
import {
  ActionGroup,
  ConsoleSurface,
  InsightPanel,
  MetricStrip,
  ObjectTable,
  ResourceRelationshipGraph,
  StatusPill
} from "../shared/commercial-console.jsx";
import { available, money, packageText, statusColor, statusLabel } from "../shared/formatters.js";

function statusTone(value) {
  const color = statusColor(value);
  if (color === "green") return "good";
  if (color === "red") return "danger";
  if (color === "orange") return "warn";
  return "info";
}

export function WorkspacesPage({ state, wallet, runAction, session }) {
  const planById = Object.fromEntries((state.packages || []).map((plan) => [plan.id, plan]));
  const computeById = new Map((state.computeAllocations || []).map((compute) => [compute.id, compute]));
  const storageById = new Map((state.storageVolumes || []).map((storage) => [storage.id, storage]));
  const attachmentById = new Map((state.storageAttachments || []).map((attachment) => [attachment.id, attachment]));
  const running = state.workspaces.filter((workspace) => workspace.state === "running").length;
  const activeUrls = state.workspaces.filter((workspace) => workspace.access?.tokenStatus === "active").length;
  const activeRuntimes = state.workspaces.filter((workspace) => workspace.currentComputeAllocationId && workspace.currentAttachmentId).length;

  return (
    <ConsoleSurface
      title="工作区入口"
      eyebrow="交付"
      subtitle="计算资源、存储资源、访问 URL、账单冻结"
      extra={<Button type="primary" icon={<Plus size={15} />} onClick={() => navigate(routeTo("workspace.create"))}>创建工作区入口</Button>}
    >
      <MetricStrip
        items={[
          { label: "总数", value: state.workspaces.length, caption: "当前实验室", tone: state.workspaces.length ? "info" : "neutral" },
          { label: "运行中", value: running, caption: "可访问入口", tone: running ? "good" : "neutral" },
          { label: "URL 可用", value: activeUrls, caption: "可分享链接", tone: activeUrls ? "good" : "warn" },
          { label: "运行时", value: activeRuntimes, caption: "当前计算绑定", tone: activeRuntimes ? "info" : "neutral" },
          { label: "可用余额", value: money(available(wallet)), caption: "扣除冻结后", tone: available(wallet) > 0 ? "good" : "warn" }
        ]}
      />
      <ResourceRelationshipGraph state={state} />

      <InsightPanel title="工作区入口" eyebrow="当前">
        <ObjectTable
          rowKey="id"
          data={state.workspaces}
          emptyText="暂无工作区入口"
          columns={[
            {
              title: "名称",
              dataIndex: "name",
              render: (_, row) => <Button type="link" onClick={() => navigate(routeTo("workspace.detail", { id: row.id }))}>{row.name}</Button>
            },
            {
              title: "状态",
              dataIndex: "state",
              render: (_, row) => <StatusPill label={statusLabel(row)} tone={statusTone(row.state)} />
            },
            { title: "拥有账号", dataIndex: "ownerAccountId", ellipsis: true },
            { title: "套餐", dataIndex: "packageId", render: (value) => packageText(planById[value]) },
            {
              title: "独占节点",
              render: (_, row) => {
                const compute = computeById.get(row.currentComputeAllocationId);
                return <Typography.Text ellipsis>{compute?.nodeName || compute?.cvmInstanceId || row.currentComputeAllocationId || row.server?.id || "-"}</Typography.Text>;
              }
            },
            {
              title: "存储卷",
              render: (_, row) => {
                const storage = storageById.get(row.storageId);
                return <Typography.Text ellipsis>{storage?.name || row.storageId || row.disk?.id || "-"}</Typography.Text>;
              }
            },
            {
              title: "当前挂载",
              render: (_, row) => {
                const attachment = attachmentById.get(row.currentAttachmentId);
                return <Typography.Text ellipsis>{attachment?.id || row.currentAttachmentId || "-"}</Typography.Text>;
              }
            },
            {
              title: "访问 URL",
              dataIndex: "url",
              ellipsis: true,
              render: (_, row) => <Typography.Text copyable={row.access?.tokenStatus === "active"} className="inlineCode">{row.url}</Typography.Text>
            },
            {
              title: "操作",
              valueType: "option",
              render: (_, row) => (
                <ActionGroup
                  actions={[
                    { label: "资源", icon: <Settings2 size={14} />, onClick: () => navigate(routeTo("workspace.detail", { id: row.id })) },
                    { label: "打开", icon: <LinkIcon size={14} />, disabled: row.access?.tokenStatus !== "active", onClick: () => window.open(row.url, "_blank", "noopener,noreferrer") },
                    { label: "重置", icon: <RefreshCw size={14} />, disabled: row.access?.tokenStatus !== "active", onClick: () => runAction(() => resetWorkspaceToken({ workspaceId: row.id }, session.csrfToken), "URL 已重置") },
                    { label: "停用", danger: true, icon: <Trash2 size={14} />, disabled: row.access?.tokenStatus !== "active", onClick: () => runAction(() => deleteWorkspaceToken({ workspaceId: row.id }, session.csrfToken), "URL 已停用") }
                  ]}
                />
              )
            }
          ]}
        />
      </InsightPanel>
    </ConsoleSurface>
  );
}
