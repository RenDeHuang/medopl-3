import React from "react";
import { ProFormSelect, ProFormText, StepsForm } from "@ant-design/pro-components";
import { Alert } from "antd";
import { createWorkspace } from "../../api/workspaces-api.js";
import { navigate, routeTo } from "../../consoleRoutes.js";
import { ConsoleSurface, InsightPanel, ResourceSplit, StatusPill } from "../shared/commercial-console.jsx";
import { available, money, packageText, planHold } from "../shared/formatters.js";

export function CreateWorkspacePage({ state, wallet, selectedCreatePlan, setCreatePackageId, session, runAction }) {
  const enough = available(wallet) >= planHold(selectedCreatePlan);
  return (
    <ConsoleSurface title="Create Workspace" eyebrow="Provision" subtitle="Package, URL, compute and storage hold" compact>
      <div className="consoleGrid">
        <InsightPanel title="开通流程" eyebrow="Workspace">
          <StepsForm
            onFinish={async (values) => {
              await runAction(
                () => createWorkspace({
                  workspaceName: values.workspaceName,
                  packageId: values.packageId
                }, session.csrfToken),
                "Workspace 已创建"
              );
              navigate(routeTo("workspace.list"));
              return true;
            }}
          >
            <StepsForm.StepForm name="name" title="Name">
              <ProFormText name="workspaceName" label="名称" initialValue="Lab Workspace" rules={[{ required: true }]} />
            </StepsForm.StepForm>
            <StepsForm.StepForm name="package" title="Package">
              <ProFormSelect
                name="packageId"
                label="套餐"
                initialValue={selectedCreatePlan?.id || "basic"}
                options={(state.packages || []).map((plan) => ({ label: `${plan.name} · ${packageText(plan)}`, value: plan.id }))}
                fieldProps={{ onChange: setCreatePackageId }}
                rules={[{ required: true }]}
              />
            </StepsForm.StepForm>
            <StepsForm.StepForm name="confirm" title="Confirm">
              {!enough && <Alert type="warning" showIcon message="余额不足，无法完成预冻结。" />}
              {enough && <Alert type="success" showIcon message="余额满足 7 天计算与存储预冻结。" />}
            </StepsForm.StepForm>
          </StepsForm>
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
