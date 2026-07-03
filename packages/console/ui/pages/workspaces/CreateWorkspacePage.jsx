import React from "react";
import { Alert, Button, Form, Input, Select } from "antd";
import { Plus } from "lucide-react";
import { createWorkspace } from "../../api/workspaces-api.js";
import { navigate, routeTo } from "../../consoleRoutes.js";
import { ConsoleSurface, InsightPanel, ResourceSplit, StatusPill } from "../shared/commercial-console.jsx";
import { available, money, packageText, planHold } from "../shared/formatters.js";

export function CreateWorkspacePage({ state, wallet, selectedCreatePlan, setCreatePackageId, session, runAction }) {
  const enough = available(wallet) >= planHold(selectedCreatePlan);
  const initialPackageId = selectedCreatePlan?.id || "basic";
  return (
    <ConsoleSurface title="Create Workspace" eyebrow="Provision" subtitle="Package, URL, compute and storage hold" compact>
      <div className="consoleGrid">
        <InsightPanel title="开通流程" eyebrow="Workspace">
          <Form
            layout="vertical"
            initialValues={{ workspaceName: "Lab Workspace", packageId: initialPackageId }}
            onValuesChange={(_, values) => {
              if (values.packageId) setCreatePackageId(values.packageId);
            }}
            onFinish={async (values) => {
              const created = await runAction(
                () => createWorkspace({
                  workspaceName: values.workspaceName,
                  packageId: values.packageId
                }, session.csrfToken),
                "Workspace 已创建"
              );
              if (created) navigate(routeTo("workspace.list"));
            }}
          >
            <Form.Item name="workspaceName" label="名称" rules={[{ required: true, message: "请输入 Workspace 名称" }]}>
              <Input placeholder="Lab Workspace" />
            </Form.Item>
            <Form.Item name="packageId" label="套餐" rules={[{ required: true, message: "请选择套餐" }]}>
              <Select
                options={(state.packages || []).map((plan) => ({ label: `${plan.name} · ${packageText(plan)}`, value: plan.id }))}
              />
            </Form.Item>
            {!enough && <Alert type="warning" showIcon message="余额不足，无法完成 7 天计算与存储预冻结。" />}
            {enough && <Alert type="success" showIcon message="创建后会生成计算资源、保留存储和 Workspace URL。" />}
            <Button className="formSubmit" type="primary" htmlType="submit" icon={<Plus size={15} />} disabled={!enough}>
              创建 Workspace
            </Button>
          </Form>
        </InsightPanel>

        <InsightPanel
          title="开通前检查"
          eyebrow="Hold"
          actions={<StatusPill label={enough ? "Ready" : "Blocked"} tone={enough ? "good" : "warn"} />}
        >
          <ResourceSplit
            items={[
              { label: "套餐", value: selectedCreatePlan?.name || "-", meta: packageText(selectedCreatePlan), status: "selected", tone: "info" },
              { label: "计算", value: `${money(selectedCreatePlan?.price?.computeHourly)}/h`, meta: "hourly compute billing", status: "compute", tone: "info" },
              { label: "存储", value: `${money(selectedCreatePlan?.price?.storageGbMonth)}/GB/mo`, meta: `${selectedCreatePlan?.diskGb || 0}GB retained disk`, status: "storage", tone: "good" },
              { label: "7 天冻结", value: money(planHold(selectedCreatePlan)), meta: `available ${money(available(wallet))}`, status: enough ? "covered" : "short", tone: enough ? "good" : "warn" }
            ]}
          />
        </InsightPanel>
      </div>
    </ConsoleSurface>
  );
}
