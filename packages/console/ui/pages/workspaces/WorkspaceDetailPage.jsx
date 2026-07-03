import React from "react";
import { Button, Empty, Typography } from "antd";
import { Database, HardDrive, Link as LinkIcon, RefreshCw, RotateCw, Square, Trash2 } from "lucide-react";
import {
  createStorageBackup,
  deleteWorkspaceToken,
  destroyWorkspaceDisk,
  destroyWorkspaceServer,
  resetWorkspaceToken,
  restartWorkspaceServer,
  stopWorkspaceServer
} from "../../api/workspaces-api.js";
import { navigate, routeTo } from "../../consoleRoutes.js";
import {
  ActionGroup,
  ConsoleSurface,
  InsightPanel,
  ResourceSplit,
  StatusPill,
  TimelineList
} from "../shared/commercial-console.jsx";
import { packageText, statusColor, statusLabel, valueLabel } from "../shared/formatters.js";

function toneForStatus(value) {
  const color = statusColor(value);
  if (color === "green") return "good";
  if (color === "red") return "danger";
  if (color === "orange") return "warn";
  return "info";
}

export function WorkspaceDetailPage({ selected, selectedPlan, state, session, runAction }) {
  if (!selected) {
    return (
      <ConsoleSurface title="Workspace" eyebrow="Delivery">
        <Empty description="暂无 Workspace" />
      </ConsoleSurface>
    );
  }
  const backups = (state.storageBackups || []).filter((backup) => backup.workspaceId === selected.id);
  return (
    <ConsoleSurface
      title={selected.name}
      eyebrow="Workspace detail"
      subtitle="URL, compute lifecycle, retained storage"
      extra={<Button onClick={() => navigate(routeTo("workspace.list"))}>返回列表</Button>}
    >
      <div className="consoleGrid equal">
        <InsightPanel
          title="Workspace URL"
          eyebrow="Access"
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

        <InsightPanel title="计算与存储" eyebrow="Resources">
          <ResourceSplit
            items={[
              { label: "状态", value: statusLabel(selected), meta: selected.state, status: "Workspace", tone: toneForStatus(selected.state) },
              { label: "套餐", value: selectedPlan?.name || "-", meta: packageText(selectedPlan), status: "plan", tone: "info" },
              { label: "计算", value: selected.server?.spec || "-", meta: valueLabel(selected.server?.status), status: "hourly", tone: toneForStatus(selected.server?.status) },
              { label: "存储", value: `${selected.disk?.sizeGb || 0}GB`, meta: valueLabel(selected.disk?.status), status: "retained", tone: toneForStatus(selected.disk?.status) }
            ]}
          />
        </InsightPanel>
      </div>

      <div className="consoleGrid equal">
        <InsightPanel title="生命周期操作" eyebrow="Compute and storage">
          <ActionGroup
            actions={[
              { label: "停止计算", icon: <Square size={15} />, onClick: () => runAction(() => stopWorkspaceServer({ workspaceId: selected.id, confirm: true }, session.csrfToken), "计算已停止") },
              { label: "启动计算", icon: <RotateCw size={15} />, onClick: () => runAction(() => restartWorkspaceServer({ workspaceId: selected.id }, session.csrfToken), "计算已启动") },
              { label: "销毁计算", danger: true, icon: <Trash2 size={15} />, onClick: () => runAction(() => destroyWorkspaceServer({ workspaceId: selected.id, confirm: true }, session.csrfToken), "计算已销毁") },
              { label: "销毁存储", danger: true, icon: <HardDrive size={15} />, onClick: () => runAction(() => destroyWorkspaceDisk({ workspaceId: selected.id, confirmDataLoss: true }, session.csrfToken), "存储已销毁") }
            ]}
          />
        </InsightPanel>

        <InsightPanel
          title="备份"
          eyebrow="Storage"
          actions={<Button icon={<Database size={15} />} onClick={() => runAction(() => createStorageBackup({ workspaceId: selected.id, reason: "console", retentionPolicy: { retainLast: 2 } }, session.csrfToken), "备份已创建")}>创建备份</Button>}
        >
          <TimelineList
            emptyText="暂无备份"
            items={backups.slice(-5).reverse().map((backup) => ({
              title: backup.id,
              description: valueLabel(backup.status),
              meta: backup.createdAt,
              tone: String(backup.status).includes("failed") ? "danger" : "good"
            }))}
          />
        </InsightPanel>
      </div>
    </ConsoleSurface>
  );
}
