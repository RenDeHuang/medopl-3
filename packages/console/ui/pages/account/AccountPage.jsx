import React from "react";
import { ShieldCheck, UserRound, WalletCards } from "lucide-react";
import {
  ConsoleSurface,
  InsightPanel,
  MetricStrip,
  ResourceSplit,
  TimelineList
} from "../shared/commercial-console.jsx";
import { available, money } from "../shared/formatters.js";

export function AccountPage({ state, wallet, session }) {
  return (
    <ConsoleSurface title="账号与实验室" eyebrow="身份" subtitle="用户、钱包归属、实验室策略">
      <MetricStrip
        items={[
          { label: "角色", value: session.user.role === "admin" ? "管理员" : "实验室负责人", caption: "权限边界", icon: <ShieldCheck size={16} />, tone: session.user.role === "admin" ? "info" : "good" },
          { label: "账号", value: state.account.id, caption: "计费账号", icon: <UserRound size={16} />, tone: "neutral" },
          { label: "可用余额", value: money(available(wallet)), caption: `${money(wallet.frozen)} 已冻结`, icon: <WalletCards size={16} />, tone: available(wallet) > 0 ? "good" : "warn" },
          { label: "工作区入口", value: state.workspaces.length, caption: "当前账号", tone: "info" },
          { label: "工单", value: state.supportTickets?.length || 0, caption: "账号范围", tone: "neutral" }
        ]}
      />

      <div className="consoleGrid equal">
        <InsightPanel title="账户身份" eyebrow="用户">
          <ResourceSplit
            items={[
              { label: "邮箱", value: session.user.email, meta: "登录身份", status: "可用", tone: "good" },
              { label: "角色", value: session.user.role === "admin" ? "管理员" : "实验室负责人", meta: "路由和操作范围", status: "已限定", tone: "info" },
              { label: "账号", value: state.account.id, meta: "钱包归属", status: "计费", tone: "neutral" },
              { label: "会话", value: "安全", meta: "Cookie + CSRF", status: "已保护", tone: "good" }
            ]}
          />
        </InsightPanel>

        <InsightPanel title="实验室策略" eyebrow="策略">
          <TimelineList
            items={[
              { title: "访问 URL 可分发", description: "每个工作区入口使用独立 URL token", meta: "访问", tone: "good" },
              { title: "计算和存储分开管理", description: "先开通资源，再挂载存储并创建工作区入口", meta: "生命周期", tone: "info" },
              { title: "7 天资源预冻结", description: "开通前冻结计算和存储预算", meta: "计费", tone: "warn" },
              { title: "账单按小时解释", description: "钱包、用量、账本保持同一账户口径", meta: "账本", tone: "info" }
            ]}
          />
        </InsightPanel>
      </div>
    </ConsoleSurface>
  );
}
