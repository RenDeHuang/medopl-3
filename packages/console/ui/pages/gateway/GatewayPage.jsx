import React from "react";
import { Button, Typography } from "antd";
import { ExternalLink, KeyRound } from "lucide-react";
import {
  ActionGroup,
  ConsoleSurface,
  InsightPanel,
  MetricStrip,
  ObjectTable,
  ResourceSplit,
  StatusPill
} from "../shared/commercial-console.jsx";
import { money, usageAmount } from "../shared/formatters.js";

export function GatewayPage({ state }) {
  const requestUsage = state.requestUsageLogs || [];
  return (
    <ConsoleSurface
      title="网关"
      eyebrow="外部集成"
      subtitle="gflabtoken.cn 请求用量和扣费"
      extra={<Button type="primary" icon={<ExternalLink size={15} />} onClick={() => window.open("https://gflabtoken.cn/", "_blank", "noopener,noreferrer")}>打开网关</Button>}
    >
      <MetricStrip
        items={[
          { label: "请求", value: requestUsage.length, caption: "已计量 API 调用", tone: requestUsage.length ? "info" : "neutral" },
          { label: "扣费", value: money(usageAmount(requestUsage)), caption: "请求扣费", tone: usageAmount(requestUsage) ? "warn" : "neutral" },
          { label: "可用密钥", value: 1, caption: "当前实验室范围", icon: <KeyRound size={16} />, tone: "good" },
          { label: "外部入口", value: "在线", caption: "gflabtoken.cn", tone: "good" },
          { label: "账单绑定", value: "钱包", caption: "同一账号余额", tone: "info" }
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
              { label: "入口", value: "gflabtoken.cn", meta: "外部网关", status: "外部", tone: "info" },
              { label: "作用域", value: "当前实验室", meta: "计费账号范围", status: "已限定", tone: "good" },
              { label: "扣费", value: money(usageAmount(requestUsage)), meta: "请求用量合计", status: "计量", tone: usageAmount(requestUsage) ? "warn" : "neutral" },
              { label: "密钥", value: "1 个可用", meta: "由网关管理", status: "就绪", tone: "good" }
            ]}
          />
          <ActionGroup actions={[
            { label: "打开网关", type: "primary", icon: <ExternalLink size={15} />, onClick: () => window.open("https://gflabtoken.cn/", "_blank", "noopener,noreferrer") }
          ]} />
        </InsightPanel>

        <InsightPanel title="最近请求" eyebrow="用量">
          <ObjectTable
            rowKey={(row) => row.id}
            data={requestUsage.slice(-10).reverse()}
            emptyText="暂无请求用量"
            columns={[
              { title: "请求", dataIndex: "requestId", ellipsis: true, render: (value) => <Typography.Text ellipsis>{value}</Typography.Text> },
              { title: "工作区", dataIndex: "workspaceId", ellipsis: true },
              { title: "金额", dataIndex: "amount", render: (value) => money(value) }
            ]}
          />
        </InsightPanel>
      </div>
    </ConsoleSurface>
  );
}
