import React from "react";
import { Button, Empty } from "antd";
import { Cable, Database, Plus, Server } from "lucide-react";
import { navigate, routeTo } from "../../consoleRoutes.js";
import { ActionGroup, ConsoleSurface, InsightPanel, MetricStrip, ObjectTable, ResourceSplit, StatusPill } from "../shared/commercial-console.jsx";

function resourceStatus(value) {
  const normalized = String(value || "pending");
  if (["running", "bound", "attached", "ready", "active"].includes(normalized)) return "good";
  if (["destroyed", "failed", "detached"].includes(normalized)) return "danger";
  if (["creating", "attaching", "pending"].includes(normalized)) return "warn";
  return "info";
}

function selectedResource(path, items) {
  const id = path.split("/").at(-1);
  return items.find((item) => item.id === id);
}

export function ComputeResourcesPage({ state }) {
  const computeResources = state.computeResources || [];
  return (
    <ConsoleSurface
      title="Compute"
      eyebrow="TKE resources"
      subtitle="Account-scoped runtime compute resources"
      extra={<Button type="primary" icon={<Plus size={15} />} onClick={() => navigate(routeTo("compute.create"))}>开通计算</Button>}
    >
      <MetricStrip
        items={[
          { label: "计算资源", value: computeResources.length, caption: "owned by this account", tone: computeResources.length ? "info" : "neutral" },
          { label: "运行中", value: computeResources.filter((item) => item.status === "running").length, caption: "billable runtime", tone: "good" }
        ]}
      />
      <InsightPanel title="计算资源" eyebrow="ComputeResource">
        <ObjectTable
          data={computeResources}
          emptyText="暂无计算资源"
          columns={[
            { title: "名称", dataIndex: "name", render: (_, row) => <Button type="link" onClick={() => navigate(routeTo("compute.detail", { id: row.id }))}>{row.name || row.id}</Button> },
            { title: "规格", dataIndex: "spec" },
            { title: "状态", dataIndex: "status", render: (value) => <StatusPill label={value || "pending"} tone={resourceStatus(value)} /> },
            { title: "云资源", dataIndex: "providerResourceId", ellipsis: true }
          ]}
        />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function CreateComputeResourcePage({ state }) {
  return (
    <ConsoleSurface title="Create Compute" eyebrow="Provision" subtitle="Choose a verified TKE compute package" compact>
      <InsightPanel title="开通计算" eyebrow="ComputeResource">
        <ResourceSplit
          items={(state.packages || []).filter((plan) => plan.available).map((plan) => ({
            label: plan.name,
            value: plan.server,
            meta: `${plan.cpu} CPU / ${plan.memoryGb}GB`,
            status: "verified",
            tone: "good"
          }))}
        />
        <ActionGroup actions={[{ label: "选择套餐后开通", type: "primary", icon: <Server size={15} />, disabled: true }]} />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function ComputeResourceDetailPage({ state, path }) {
  const resource = selectedResource(path, state.computeResources || []);
  if (!resource) return <ConsoleSurface title="Compute" eyebrow="ComputeResource"><Empty description="未找到计算资源" /></ConsoleSurface>;
  return (
    <ConsoleSurface title={resource.name || resource.id} eyebrow="Compute detail" extra={<Button onClick={() => navigate(routeTo("compute.list"))}>返回列表</Button>}>
      <InsightPanel title="计算资源" eyebrow="TKE">
        <ResourceSplit
          items={[
            { label: "状态", value: resource.status || "-", status: resource.status || "pending", tone: resourceStatus(resource.status) },
            { label: "规格", value: resource.spec || "-", meta: resource.packageId },
            { label: "云资源", value: resource.providerResourceId || "-", meta: resource.provider || "tencent-tke" }
          ]}
        />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function StorageVolumesPage({ state }) {
  const storageVolumes = state.storageVolumes || [];
  return (
    <ConsoleSurface
      title="Storage"
      eyebrow="TKE resources"
      subtitle="Persistent storage volumes owned by this account"
      extra={<Button type="primary" icon={<Plus size={15} />} onClick={() => navigate(routeTo("storage.create"))}>开通存储</Button>}
    >
      <InsightPanel title="存储卷" eyebrow="StorageVolume">
        <ObjectTable
          data={storageVolumes}
          emptyText="暂无存储资源"
          columns={[
            { title: "名称", dataIndex: "name", render: (_, row) => <Button type="link" onClick={() => navigate(routeTo("storage.detail", { id: row.id }))}>{row.name || row.id}</Button> },
            { title: "容量", dataIndex: "sizeGb", render: (value) => `${value || 0}GB` },
            { title: "状态", dataIndex: "status", render: (value) => <StatusPill label={value || "pending"} tone={resourceStatus(value)} /> },
            { title: "云资源", dataIndex: "providerResourceId", ellipsis: true }
          ]}
        />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function CreateStorageVolumePage() {
  return (
    <ConsoleSurface title="Create Storage" eyebrow="Provision" subtitle="Create a retained TKE storage volume" compact>
      <InsightPanel title="开通存储" eyebrow="StorageVolume">
        <ResourceSplit items={[{ label: "默认容量", value: "10GB", meta: "retained persistent storage", status: "billable", tone: "info" }]} />
        <ActionGroup actions={[{ label: "确认开通存储", type: "primary", icon: <Database size={15} />, disabled: true }]} />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function StorageVolumeDetailPage({ state, path }) {
  const resource = selectedResource(path, state.storageVolumes || []);
  if (!resource) return <ConsoleSurface title="Storage" eyebrow="StorageVolume"><Empty description="未找到存储资源" /></ConsoleSurface>;
  return (
    <ConsoleSurface title={resource.name || resource.id} eyebrow="Storage detail" extra={<Button onClick={() => navigate(routeTo("storage.list"))}>返回列表</Button>}>
      <InsightPanel title="存储资源" eyebrow="TKE">
        <ResourceSplit
          items={[
            { label: "状态", value: resource.status || "-", status: resource.status || "pending", tone: resourceStatus(resource.status) },
            { label: "容量", value: `${resource.sizeGb || 0}GB`, meta: resource.storageClassId },
            { label: "云资源", value: resource.providerResourceId || "-", meta: resource.provider || "tencent-tke" }
          ]}
        />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function StorageAttachmentsPage({ state }) {
  const attachments = state.storageAttachments || [];
  return (
    <ConsoleSurface
      title="Attachments"
      eyebrow="Mounts"
      subtitle="Attach storage volumes to compute resources"
      extra={<Button type="primary" icon={<Plus size={15} />} onClick={() => navigate(routeTo("attachment.create"))}>挂载存储</Button>}
    >
      <InsightPanel title="挂载关系" eyebrow="StorageAttachment">
        <ObjectTable
          data={attachments}
          emptyText="暂无挂载关系"
          columns={[
            { title: "挂载", dataIndex: "id" },
            { title: "计算", dataIndex: "computeId", render: (_, row) => <Button type="link" onClick={() => navigate(routeTo("attachment.detail", { id: row.id }))}>{row.computeId}</Button> },
            { title: "存储", dataIndex: "storageId" },
            { title: "路径", dataIndex: "mountPath" },
            { title: "状态", dataIndex: "status", render: (value) => <StatusPill label={value || "pending"} tone={resourceStatus(value)} /> }
          ]}
        />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function StorageAttachmentDetailPage({ state, path }) {
  const attachment = selectedResource(path, state.storageAttachments || []);
  if (!attachment) return <ConsoleSurface title="Attachment" eyebrow="StorageAttachment"><Empty description="未找到挂载关系" /></ConsoleSurface>;
  return (
    <ConsoleSurface title={attachment.id} eyebrow="Attachment detail" extra={<Button onClick={() => navigate(routeTo("attachment.list"))}>返回列表</Button>}>
      <InsightPanel title="挂载关系" eyebrow="StorageAttachment">
        <ResourceSplit
          items={[
            { label: "状态", value: attachment.status || "-", status: attachment.status || "pending", tone: resourceStatus(attachment.status) },
            { label: "计算", value: attachment.computeId || "-", meta: "ComputeResource" },
            { label: "存储", value: attachment.storageId || "-", meta: attachment.mountPath || "/data" }
          ]}
        />
      </InsightPanel>
    </ConsoleSurface>
  );
}

export function CreateStorageAttachmentPage() {
  return (
    <ConsoleSurface title="Attach Storage" eyebrow="Mount" subtitle="Select one compute resource and one storage volume" compact>
      <InsightPanel title="挂载存储" eyebrow="StorageAttachment">
        <ResourceSplit items={[{ label: "挂载路径", value: "/data", meta: "one-person-lab-app persistent state", status: "required", tone: "info" }]} />
        <ActionGroup actions={[{ label: "确认挂载", type: "primary", icon: <Cable size={15} />, disabled: true }]} />
      </InsightPanel>
    </ConsoleSurface>
  );
}
