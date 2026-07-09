import React from "react";
import { Button } from "antd";
import { ExternalLink, KeyRound } from "lucide-react";
import {
  ActionGroup,
  ConsoleSurface,
  InsightPanel,
  MetricStrip,
  ResourceSplit,
  StatusPill
} from "../shared/commercial-console.tsx";

export function GatewayPage({ state = {} }: any) {
  const gateway = state.gateway || {};
  const gatewayUrl = gateway.url || "";
  const keyCount = gateway.availableKeys ?? gateway.keyCount ?? "-";
  const gatewayStatus = gateway.status || "未接入";
  return (
    <ConsoleSurface
      title="网关"
      eyebrow="外部集成"
      subtitle="one-person-lab-cloud 管理请求级产品能力"
      extra={<Button type="primary" icon={<ExternalLink size={15} />} disabled={!gatewayUrl} onClick={() => window.open(gatewayUrl, "_blank", "noopener,noreferrer")}>打开网关</Button>}
    >
      <MetricStrip
        items={[
          { label: "可用密钥", value: keyCount, caption: "后端网关状态", icon: <KeyRound size={16} />, tone: keyCount === "-" ? "neutral" : "good" },
          { label: "外部入口", value: gatewayStatus, caption: gateway.host || "-", tone: gatewayStatus === "online" ? "good" : "neutral" },
          { label: "计费边界", value: "外部", caption: "请求级账本不在 OPL Cloud", tone: "info" }
        ]}
      />

      <div className="consoleGrid">
        <InsightPanel
          title="接入状态"
          eyebrow="网关"
          actions={<StatusPill label="可用" tone="good" />}
        >
          <ResourceSplit
            items={[
              { label: "入口", value: gateway.host || "-", meta: "后端返回", status: gatewayStatus, tone: gatewayStatus === "online" ? "good" : "neutral" },
              { label: "作用域", value: gateway.scope || "-", meta: "由后端返回", status: gateway.scope ? "已限定" : "未接入", tone: gateway.scope ? "good" : "neutral" },
              { label: "OPL Cloud 账本", value: "资源租赁", meta: "只记录计算和存储", status: "解耦", tone: "info" },
              { label: "密钥", value: keyCount === "-" ? "-" : `${keyCount} 个可用`, meta: "由后端返回", status: keyCount === "-" ? "未接入" : "就绪", tone: keyCount === "-" ? "neutral" : "good" }
            ]}
          />
          <ActionGroup actions={[
            { label: "打开网关", type: "primary", icon: <ExternalLink size={15} />, disabled: !gatewayUrl, onClick: () => window.open(gatewayUrl, "_blank", "noopener,noreferrer") }
          ]} />
        </InsightPanel>

        <InsightPanel title="账本边界" eyebrow="说明">
          <ResourceSplit
            items={[
              { label: "Fabric", value: "计算 / 存储", meta: "OPL Cloud 开通和销毁", status: "本仓库", tone: "good" },
              { label: "Ledger", value: "资源租赁", meta: "冻结、扣费、释放、对账", status: "本仓库", tone: "good" },
              { label: "请求级产品", value: "外部", meta: "由 one-person-lab-cloud 负责", status: "解耦", tone: "info" }
            ]}
          />
        </InsightPanel>
      </div>
    </ConsoleSurface>
  );
}
