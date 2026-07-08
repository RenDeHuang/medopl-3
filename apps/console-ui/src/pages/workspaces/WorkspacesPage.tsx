import React from "react";
import { Button, Empty, Typography } from "antd";
import { Cable, HardDrive, Link as LinkIcon, Plus, Server, Settings2, WalletCards } from "lucide-react";
import { navigate, routeTo } from "../../consoleRoutes.ts";
import {
  ActionGroup,
  ConsoleSurface,
  InsightPanel,
  MetricStrip,
  ObjectTable,
  ResourceSplit,
  StatusPill
} from "../shared/commercial-console.tsx";
import { available, money, statusColor, statusLabel, workspaceAccessLabel, workspaceAccessTone, workspaceOpenActionLabel, workspaceUrlReady } from "../shared/formatters.ts";

type AnyRecord = Record<string, any>;

function statusTone(value) {
  const color = statusColor(value);
  if (color === "green") return "good";
  if (color === "red") return "danger";
  if (color === "orange") return "warn";
  return "info";
}

function workspaceCredential(workspace: AnyRecord = {}) {
  const account = workspace.access?.account
    || workspace.access?.username
    || workspace.login?.username
    || workspace.id
    || "-";
  const ready = Boolean(workspace.access?.password);
  return { account, ready };
}

export function WorkspacesPage({ state, wallet }: any) {
  const running = state.workspaces.filter((workspace) => workspace.state === "running").length;
  const billingSummary = state.billingSummary || {};
  const billedTotal = Number(billingSummary.recentResourceDebitTotal || 0);
  const hourlyTotal = Number(billingSummary.activeHourlyEstimate || 0);
  const computeResources = (state.computeAllocations || []).filter((item) => item.status !== "destroyed");
  const storageResources = (state.storageVolumes || []).filter((item) => item.status !== "destroyed");
  const attachments = state.storageAttachments || [];
  const activeAttachments = attachments.filter((item) => item.status === "attached");

  return (
    <ConsoleSurface
      title="工作区"
      eyebrow="工作区"
      subtitle="创建、访问和管理工作区"
      extra={<Button type="primary" icon={<Plus size={15} />} onClick={() => navigate(routeTo("workspace.create"))}>创建工作区</Button>}
    >
      <MetricStrip
        items={[
          { label: "工作区", value: state.workspaces.length, caption: "访问入口", tone: state.workspaces.length ? "info" : "neutral" },
          { label: "运行中", value: running, caption: "可直接访问", tone: running ? "good" : "neutral" },
          { label: "当前费用", value: money(billedTotal), caption: "按工作区汇总", icon: <WalletCards size={16} />, tone: billedTotal > 0 ? "info" : "neutral" },
          { label: "预计每小时", value: money(hourlyTotal), caption: "当前活跃资源", tone: hourlyTotal > 0 ? "warn" : "neutral" },
          { label: "可用余额", value: money(available(wallet)), caption: "扣除冻结后", tone: available(wallet) > 0 ? "good" : "warn" }
        ]}
      />

      <InsightPanel title="资源管理" eyebrow="工作区下的计算、存储、挂载">
        <ResourceSplit
          items={[
            { label: "计算资源", value: `${computeResources.length} 个`, meta: "开通和查看计算节点", status: computeResources.length ? "可查看" : "未开通", tone: computeResources.length ? "info" : "neutral" },
            { label: "云硬盘", value: `${storageResources.length} 个`, meta: "开通和查看数据盘", status: storageResources.length ? "可查看" : "未开通", tone: storageResources.length ? "info" : "neutral" },
            { label: "挂载关系", value: `${activeAttachments.length} 个`, meta: "存储挂到计算后才能创建入口", status: activeAttachments.length ? "已挂载" : "待挂载", tone: activeAttachments.length ? "good" : "warn" },
            { label: "资源关系", value: "链路图", meta: "账号、计算、存储、挂载、工作区", status: "可查看", tone: "info" }
          ]}
        />
        <ActionGroup
          actions={[
            { label: "计算资源", icon: <Server size={15} />, onClick: () => navigate(routeTo("compute-allocations.list")) },
            { label: "云硬盘", icon: <HardDrive size={15} />, onClick: () => navigate(routeTo("storage.list")) },
            { label: "挂载关系", icon: <Cable size={15} />, onClick: () => navigate(routeTo("attachment.list")) },
            { label: "资源关系", icon: <Settings2 size={15} />, onClick: () => navigate(routeTo("resources.relationships")) }
          ]}
        />
      </InsightPanel>

      <InsightPanel title="工作区列表" eyebrow="访问入口、状态、费用">
        <div className="mobileWorkspaceList">
          {state.workspaces.length ? state.workspaces.map((workspace) => {
            return (
              <article className="mobileWorkspaceCard" key={workspace.id}>
                <div className="mobileWorkspaceHeader">
                  <strong>{workspace.name}</strong>
                  <StatusPill label={statusLabel(workspace)} tone={statusTone(workspace.state)} />
                </div>
                <div className="mobileWorkspaceFacts">
                  <span>当前费用 <b>{money(workspace.billing?.currentChargeTotal)}</b></span>
                  <span>预计每小时 <b>{money(workspace.billing?.activeHourlyEstimate)}</b></span>
                  <span>计算 <b>{workspace.currentComputeAllocationId || "-"}</b></span>
                  <span>存储 <b>{workspace.storageId || "-"}</b></span>
                  <span>账号 <b>{workspaceCredential(workspace).account}</b></span>
                  <span>访问入口 <b>{workspaceAccessLabel(workspace)}</b></span>
                </div>
                <ActionGroup
                  actions={[
                    { label: workspaceOpenActionLabel(workspace), icon: <LinkIcon size={14} />, disabled: !workspaceUrlReady(workspace), onClick: () => window.open(workspace.url, "_blank", "noopener,noreferrer") },
                    { label: "详情", icon: <Settings2 size={14} />, onClick: () => navigate(routeTo("workspace.detail", { id: workspace.id })) }
                  ]}
                />
              </article>
            );
          }) : <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无工作区" />}
        </div>
        <ObjectTable
          className="objectTable workspaceTable desktopWorkspaceTable"
          rowKey="id"
          data={state.workspaces}
          tableLayout="fixed"
          scroll={{ x: 980 }}
          emptyText="暂无工作区"
          columns={[
            {
              title: "名称",
              dataIndex: "name",
              width: 150,
              render: (_, row) => (
                <div className="workspaceNameCell">
                  <Button type="link" onClick={() => navigate(routeTo("workspace.detail", { id: row.id }))}>{row.name}</Button>
                  <Typography.Text type="secondary" ellipsis>{row.id}</Typography.Text>
                </div>
              )
            },
            {
              title: "状态",
              dataIndex: "state",
              width: 88,
              render: (_, row) => <StatusPill label={statusLabel(row)} tone={statusTone(row.state)} />
            },
            {
              title: "当前费用",
              width: 105,
              render: (_, row) => money(row.billing?.currentChargeTotal)
            },
            {
              title: "预计每小时",
              width: 105,
              render: (_, row) => money(row.billing?.activeHourlyEstimate)
            },
            {
              title: "计算",
              width: 120,
              render: (_, row) => <Typography.Text ellipsis>{row.currentComputeAllocationId || "-"}</Typography.Text>
            },
            {
              title: "存储",
              width: 120,
              render: (_, row) => <Typography.Text ellipsis>{row.storageId || "-"}</Typography.Text>
            },
            {
              title: "访问入口",
              width: 125,
              render: (_, row) => <StatusPill label={workspaceAccessLabel(row)} tone={workspaceAccessTone(row)} />
            },
            {
              title: "账号",
              width: 100,
              render: (_, row) => <Typography.Text copyable className="inlineCode credentialCell">{workspaceCredential(row).account}</Typography.Text>
            },
            {
              title: "操作",
              valueType: "option",
              width: 150,
              render: (_, row) => (
                <ActionGroup
                  actions={[
                    { label: workspaceOpenActionLabel(row), icon: <LinkIcon size={14} />, disabled: !workspaceUrlReady(row), onClick: () => window.open(row.url, "_blank", "noopener,noreferrer") },
                    { label: "详情", icon: <Settings2 size={14} />, onClick: () => navigate(routeTo("workspace.detail", { id: row.id })) }
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
