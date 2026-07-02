import React, { useEffect, useMemo, useState } from "react";
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  ClipboardList,
  Copy,
  CreditCard,
  Database,
  HardDrive,
  History,
  KeyRound,
  Link as LinkIcon,
  LogOut,
  Play,
  RefreshCw,
  RotateCw,
  Server,
  ShieldCheck,
  Square,
  Trash2,
  UserRound,
  WalletCards
} from "lucide-react";

async function api(path, body, csrfToken) {
  const response = await fetch(path, {
    method: "POST",
    headers: {
      "content-type": "application/json",
      "x-opl-csrf": csrfToken
    },
    body: JSON.stringify(body)
  });
  const payload = await response.json();
  if (!response.ok || payload.ok === false) throw new Error(payload.error || "request_failed");
  return payload;
}

async function getJson(path) {
  const response = await fetch(path);
  const payload = await response.json();
  if (!response.ok || payload.ok === false) throw new Error(payload.error || "request_failed");
  return payload;
}

function money(value) {
  return `¥${Number(value || 0).toFixed(2)}`;
}

function packageSummary(plan) {
  return `${plan.cpu} CPU / ${plan.memoryGb}GB 内存 / ${plan.diskGb}GB 存储`;
}

function statusLabel(workspace) {
  if (!workspace) return "暂无 OPL Workspace";
  const labels = {
    running: "运行中",
    stopped_server_disk_retained: "已停止，存储保留",
    server_destroyed_disk_retained: "计算已销毁，存储保留",
    storage_hold_exhausted: "存储冻结金额耗尽",
    stopped_storage_hold_exhausted: "已停止，存储冻结金额耗尽",
    destroyed: "已销毁",
    failed: "失败"
  };
  return labels[workspace.state] || workspace.state;
}

function valueLabel(value) {
  const labels = {
    active: "有效",
    available: "可用",
    running: "运行中",
    stopped: "已停止",
    destroyed: "已销毁",
    failed: "失败",
    retained: "已保留",
    attached_retained: "已挂载并保留",
    detached_retained: "已卸载并保留",
    restored_retained: "已从备份恢复",
    hold_exhausted: "冻结金额耗尽",
    not_required: "不需要",
    billing_reconciliation_not_required: "无需对账",
    ready: "已就绪",
    blocked: "阻塞"
  };
  return labels[value] || value;
}

function errorLabel(value) {
  const labels = {
    request_failed: "请求失败",
    login_failed: "登录失败",
    invalid_credentials: "邮箱或密码不正确",
    not_authenticated: "请先登录",
    csrf_token_invalid: "页面安全令牌已失效，请重新登录",
    admin_role_required: "需要管理员权限",
    insufficient_prepaid_hold_balance: "余额不足，无法完成 7 天预冻结",
    storage_backup_unsupported: "当前运行时暂不支持存储备份",
    storage_backup_delete_unsupported: "当前运行时暂不支持清理备份",
    storage_backup_not_found: "未找到存储备份",
    storage_backup_not_available: "存储备份不可用",
    workspace_not_found: "未找到 OPL Workspace",
    workspace_token_inactive: "OPL Workspace 访问令牌已失效",
    workspace_token_invalid: "OPL Workspace 访问令牌无效",
    billing_reconciliation_guard_blocked: "对账保护已阻止创建新的 OPL Workspace"
  };
  return labels[value] || value;
}

function packageLabel(packageId) {
  const labels = {
    basic: "Basic Workspace",
    pro: "Pro Workspace"
  };
  return labels[packageId] || packageId;
}

function eventTitle(type) {
  const labels = {
    credit: "充值",
    compute_hold: "计算冻结",
    storage_hold: "存储冻结",
    compute_debit: "计算扣费",
    storage_debit: "存储扣费",
    server_billing_stopped: "计算计费已停止",
    server_destroyed: "计算已销毁",
    missing_env: "缺少环境变量",
    missing_tool: "缺少工具",
    failed_check: "检查失败",
    stop_server: "停止计算",
    restart_server: "重启计算",
    recreate_server: "重建计算",
    destroy_server: "销毁计算",
    destroy_disk: "销毁存储",
    create_workspace_runtime: "创建运行时",
    restore_workspace_from_backup: "从备份恢复"
  };
  const key = String(type || "event").replaceAll(".", "_");
  return labels[key] || String(type || "事件").replaceAll("_", " ").replaceAll(".", " · ");
}

function readinessLabel(value) {
  if (!value) return "未检查";
  return value.ready ? "已就绪" : "阻塞";
}

function readinessTone(value) {
  if (!value) return "muted";
  return value.ready ? "ok" : "danger";
}

function backupForWorkspace(backups, workspaceId) {
  return (backups || []).filter((backup) => backup.workspaceId === workspaceId);
}

function workspaceEvents(state, workspaceId) {
  return [
    ...(state.notifications || []),
    ...(state.audit || []),
    ...(state.evidenceLedger || []),
    ...(state.runtimeOperations || [])
  ].filter((event) => !workspaceId || event.workspaceId === workspaceId);
}

function costRunwayDays(account, plan) {
  if (!account || !plan) return "暂无";
  const dailyCompute = Number(plan.price?.computeHourly || 0) * 24;
  const dailyStorage = Number(plan.price?.storageGbMonth || 0) * Number(plan.diskGb || 0) / 30;
  const daily = dailyCompute + dailyStorage;
  if (!daily) return "暂无";
  return `${Math.max(0, Number(account.balance || 0) / daily).toFixed(1)} 天`;
}

function usageAmount(logs, resourceType = "") {
  return (logs || [])
    .filter((log) => !resourceType || log.resourceType === resourceType)
    .reduce((sum, log) => sum + Number(log.amount || 0), 0);
}

function usageQuantity(logs, resourceType) {
  return (logs || [])
    .filter((log) => log.resourceType === resourceType)
    .reduce((sum, log) => sum + Number(log.quantity || 0), 0);
}

function usageCount(logs) {
  return (logs || []).length;
}

export default function ConsolePage({ session, onLogout }) {
  const [state, setState] = useState(null);
  const [selectedId, setSelectedId] = useState("");
  const [error, setError] = useState("");
  const [adminTopUp, setAdminTopUp] = useState({ accountId: "", amount: 200, reason: "手工充值" });
  const [adminOps, setAdminOps] = useState({ operator: null, runtime: null, production: null, error: "" });

  const isAdmin = session.user.role === "admin";

  async function refresh() {
    const next = await getJson("/api/state");
    setState(next);
    setSelectedId((current) => current || next.workspaces[0]?.id || "");
  }

  async function refreshAdminOps() {
    if (!isAdmin) return;
    try {
      const [operator, runtime, production] = await Promise.all([
        getJson("/api/operator/summary"),
        getJson("/api/runtime/readiness"),
        getJson("/api/production/readiness")
      ]);
      setAdminOps({ operator, runtime, production, error: "" });
    } catch (err) {
      setAdminOps((current) => ({ ...current, error: err.message }));
    }
  }

  async function run(action) {
    try {
      setError("");
      await action();
      await refresh();
      await refreshAdminOps();
    } catch (err) {
      setError(err.message);
    }
  }

  async function logout() {
    try {
      await api("/api/auth/logout", {}, session.csrfToken);
    } finally {
      onLogout();
      window.location.hash = "home";
    }
  }

  useEffect(() => {
    refresh().catch((err) => setError(err.message));
  }, []);

  useEffect(() => {
    refreshAdminOps();
  }, [isAdmin]);

  const selected = useMemo(() => state?.workspaces.find((item) => item.id === selectedId) || state?.workspaces[0], [state, selectedId]);
  const selectedPlan = useMemo(() => state?.packages.find((plan) => plan.id === selected?.packageId), [state, selected]);
  const selectedBackups = useMemo(() => backupForWorkspace(state?.storageBackups, selected?.id), [state, selected]);
  const selectedEvents = useMemo(() => workspaceEvents(state || {}, selected?.id), [state, selected]);
  const latestBackup = selectedBackups.find((backup) => backup.status === "available") || selectedBackups[0];
  const wallet = state?.wallet || state?.account || { balance: 0, frozen: 0, available: 0, holds: {} };
  const resourceUsageLogs = state?.resourceUsageLogs || [];
  const requestUsageLogs = state?.requestUsageLogs || [];
  const walletTransactions = state?.walletTransactions || [];
  const manualTopups = state?.manualTopups || [];

  if (!state) {
    return (
      <div className="shell">
        <div className="loading">正在加载 OPL Console...</div>
      </div>
    );
  }

  return (
    <div className="shell">
      <aside className="sidebar">
        <a className="brand" href="#home">
          <div className="brandIcon">OPL</div>
          <div>
            <strong>OPL Console</strong>
            <span>{session.user.email}</span>
          </div>
        </a>
        <nav>
          <a href="#overview">概览</a>
          <a href="#workspaces">OPL Workspace</a>
          <a href="#create">创建 OPL Workspace</a>
          <a href="#billing">账单</a>
          {isAdmin && <a href="#admin">管理员</a>}
        </nav>
        <button className="ghost wide" onClick={logout}><LogOut size={16} /> 退出登录</button>
      </aside>

      <main className="main">
        <header className="topbar" id="overview">
          <div>
            <p className="eyebrow">OPL Cloud</p>
            <h1>OPL Workspace 控制台</h1>
            <p className="topbarText">通过预付实验室账号创建、分发和运维托管 OPL Workspace。</p>
          </div>
          <div className="accountBadge">
            <strong>{session.user.name || session.user.email}</strong>
            <span>{isAdmin ? "管理员" : "实验室负责人"} · {state.account.id}</span>
          </div>
        </header>

        {error && <div className="error">{errorLabel(error)}</div>}

        <section className="metrics">
          <Metric icon={<WalletCards />} label="用户钱包" value={money(wallet.balance)} />
          <Metric icon={<CreditCard />} label="冻结金额" value={money(wallet.frozen)} />
          <Metric icon={<Server />} label="OPL Workspace" value={state.workspaces.length} />
          <Metric icon={<AlertTriangle />} label="告警" value={state.notifications.length} />
        </section>

        <section id="workspaces" className="band workspaceBand">
          <div className="sectionHeader">
            <div>
              <h2>OPL Workspace</h2>
              <p>一个 OPL Workspace 包含访问 URL、计算、存储、备份、计费状态和近期活动。</p>
            </div>
            <a className="secondaryLink" href="#create"><Play size={16} /> 新建</a>
          </div>

          <div className="workspaceLayout">
            <div className="workspaceList">
              {state.workspaces.length === 0 && <div className="empty">还没有 OPL Workspace。请先充值，再创建套餐。</div>}
              {state.workspaces.map((workspace) => (
                <button
                  className={`workspaceRow ${workspace.id === selected?.id ? "active" : ""}`}
                  key={workspace.id}
                  onClick={() => setSelectedId(workspace.id)}
                >
                  <span>{workspace.name}</span>
                  <strong>{statusLabel(workspace)}</strong>
                  <small>{packageLabel(workspace.packageId)} · {valueLabel(workspace.access.tokenStatus)}</small>
                </button>
              ))}
            </div>

            <WorkspaceDetail
              selected={selected}
              selectedPlan={selectedPlan}
              account={state.account}
              backups={selectedBackups}
              latestBackup={latestBackup}
              events={selectedEvents}
              session={session}
              run={run}
            />
          </div>
        </section>

        <section id="create" className="band">
          <div className="sectionHeader">
            <div>
              <h2>创建 OPL Workspace</h2>
              <p>选择计算与存储套餐。OPL Workspace 创建后才会显示运行时控制。</p>
            </div>
          </div>
          <div className="planGrid">
            {state.packages.map((plan) => (
              <article className="plan" key={plan.id}>
                <span>{plan.id === "basic" ? "试点默认" : "CPU 套餐"}</span>
                <h3>{plan.name}</h3>
                <p>{packageSummary(plan)}</p>
                <dl className="planFacts">
                  <div><dt>计算</dt><dd>{money(plan.price.computeHourly)}/小时</dd></div>
                  <div><dt>存储</dt><dd>{money(plan.price.storageGbMonth)}/GB/月</dd></div>
                  <div><dt>7 天冻结</dt><dd>{money((Number(plan.price.computeHourly) * 24 * 7) + (Number(plan.price.storageGbMonth) * Number(plan.diskGb) / 30 * 7))}</dd></div>
                </dl>
                <button
                  className="primary"
                  onClick={() => run(() => api("/api/workspaces", {
                    workspaceName: plan.id === "basic" ? "课题组实验室" : "蛋白实验室",
                    packageId: plan.id
                  }, session.csrfToken))}
                >
                  <Play size={16} /> 创建 {plan.id}
                </button>
              </article>
            ))}
          </div>
        </section>

        <section id="billing" className="band">
          <div className="sectionHeader">
            <div>
              <h2>账单</h2>
              <p>OPL Ledger 记录预付冻结、小时扣费、充值、对账状态和证据回执。</p>
            </div>
            <Database />
          </div>
          <PolicyGrid policy={state.billingPolicy} />
          <div className="accountSummary">
            <ResourceCard icon={<WalletCards />} title="用户钱包" value={money(wallet.balance)} detail={`可用 ${money(wallet.available ?? (Number(wallet.balance || 0) - Number(wallet.frozen || 0)))}`} />
            <ResourceCard icon={<CreditCard />} title="冻结金额" value={money(wallet.frozen)} detail={`累计充值 ${money(wallet.totalRecharged || 0)}`} />
            <ResourceCard icon={<Server />} title="Compute 小时" value={`${usageQuantity(resourceUsageLogs, "compute").toFixed(0)} 小时`} detail={`${money(usageAmount(resourceUsageLogs, "compute"))} 已扣费`} />
            <ResourceCard icon={<HardDrive />} title="Storage GB-hour" value={`${usageQuantity(resourceUsageLogs, "storage").toFixed(0)} GB-hour`} detail={`${money(usageAmount(resourceUsageLogs, "storage"))} 已扣费`} />
            <ResourceCard icon={<Activity />} title="请求用量" value={`${usageCount(requestUsageLogs)} 条`} detail={`${money(usageAmount(requestUsageLogs))} 已扣费`} />
          </div>
          <div className="splitGrid">
            <Panel title="资源用量" icon={<Server />}>
              <UsageList events={resourceUsageLogs} />
            </Panel>
            <Panel title="请求用量" icon={<Activity />}>
              <RequestUsageList events={requestUsageLogs} />
            </Panel>
            <Panel title="钱包流水" icon={<WalletCards />}>
              <WalletTransactionList events={walletTransactions} />
            </Panel>
            <Panel title="充值审计" icon={<CreditCard />}>
              <ManualTopupList events={manualTopups} />
            </Panel>
            <Panel title="OPL Ledger 事件" icon={<History />}>
              <EventList events={state.billingLedger} />
            </Panel>
            <Panel title="回执与告警" icon={<ClipboardList />}>
              <EventList events={[...state.notifications, ...state.audit, ...state.evidenceLedger]} />
            </Panel>
          </div>
        </section>

        {isAdmin && (
          <section id="admin" className="band">
            <div className="sectionHeader">
              <div>
                <h2>管理员</h2>
                <p>账号、手工充值、生产就绪、对账和运维证据仅管理员可见。</p>
              </div>
              <ShieldCheck />
            </div>
            {adminOps.error && <div className="error">{errorLabel(adminOps.error)}</div>}
            <AdminPanel
              state={state}
              adminTopUp={adminTopUp}
              setAdminTopUp={setAdminTopUp}
              adminOps={adminOps}
              session={session}
              run={run}
            />
          </section>
        )}
      </main>
    </div>
  );
}

function WorkspaceDetail({ selected, selectedPlan, account, backups, latestBackup, events, session, run }) {
  if (!selected) return <div className="workspaceDetail"><div className="empty">创建 OPL Workspace 后，可在这里查看 URL、计算、存储、备份和活动控制。</div></div>;

  return (
    <div className="workspaceDetail">
      <div className="detailHeader">
        <div>
          <p className="eyebrow">OPL Workspace 详情</p>
          <h3>{selected.name}</h3>
          <span className={`pill ${selected.state === "running" ? "ok" : "muted"}`}>{statusLabel(selected)}</span>
        </div>
        <div className="detailMeta">
          <span>{selected.id}</span>
          <strong>{packageLabel(selected.packageId)}</strong>
        </div>
      </div>

      <Panel title="OPL Workspace URL" icon={<LinkIcon />}>
        <div className="urlBox compact">
          <code>{selected.url}</code>
          <button disabled={selected.access.tokenStatus !== "active"} onClick={() => navigator.clipboard?.writeText(selected.url)}><Copy size={16} /> 复制</button>
          <button disabled={selected.access.tokenStatus !== "active"} onClick={() => window.open(selected.url, "_blank", "noopener,noreferrer")}><LinkIcon size={16} /> 打开</button>
          <button disabled={selected.access.tokenStatus !== "active"} onClick={() => run(() => api("/api/workspaces/reset-token", { workspaceId: selected.id }, session.csrfToken))}><RefreshCw size={16} /> 重置</button>
          <button className="danger" disabled={selected.access.tokenStatus !== "active"} onClick={() => run(() => api("/api/workspaces/delete-token", { workspaceId: selected.id }, session.csrfToken))}><Trash2 size={16} /> 删除</button>
        </div>
      </Panel>

      <div className="resourceGrid">
        <ResourceCard icon={<Server />} title="计算" value={`${selected.server.spec} / ${valueLabel(selected.server.status)}`} detail={valueLabel(selected.server.billingStatus)} />
        <ResourceCard icon={<HardDrive />} title="存储" value={`${selected.disk.sizeGb}GB / ${valueLabel(selected.disk.status)}`} detail={valueLabel(selected.disk.billingStatus)} />
        <ResourceCard icon={<KeyRound />} title="访问令牌" value={valueLabel(selected.access.tokenStatus)} detail="重置或删除前长期有效" />
        <ResourceCard icon={<WalletCards />} title="余额可运行时长" value={costRunwayDays(account, selectedPlan)} detail="按当前套餐价格估算" />
      </div>

      <Panel title="计算与存储生命周期" icon={<Server />}>
        <div className="actions">
          <button onClick={() => run(() => api("/api/workspaces/stop-server", { workspaceId: selected.id, confirm: true }, session.csrfToken))}><Square size={16} /> 停止计算</button>
          <button onClick={() => run(() => api("/api/workspaces/restart-server", { workspaceId: selected.id }, session.csrfToken))}><RotateCw size={16} /> 重启</button>
          <button className="danger" onClick={() => run(() => api("/api/workspaces/destroy-server", { workspaceId: selected.id, confirm: true }, session.csrfToken))}><Trash2 size={16} /> 销毁计算</button>
          <button className="danger strong" onClick={() => run(() => api("/api/workspaces/destroy-disk", { workspaceId: selected.id, confirmDataLoss: true }, session.csrfToken))}><Trash2 size={16} /> 销毁存储</button>
          <button onClick={() => run(() => api("/api/billing/settle", {
            workspaceId: selected.id,
            hours: 1,
            sourceEventId: `console_billing_tick_${Date.now()}`
          }, session.csrfToken))}><CreditCard size={16} /> 结算 1 小时</button>
        </div>
      </Panel>

      <Panel title="备份与恢复" icon={<Database />}>
        <div className="actions">
          <button onClick={() => run(() => api("/api/workspaces/storage-backups", {
            workspaceId: selected.id,
            reason: "manual_console",
            retentionPolicy: { retainLast: 2 }
          }, session.csrfToken))}><Database size={16} /> 创建备份</button>
          <button disabled={!latestBackup} onClick={() => run(() => api("/api/workspaces/restore-storage-backup", {
            backupId: latestBackup.id,
            workspaceName: `${selected.name} 恢复版`,
            packageId: selected.packageId
          }, session.csrfToken))}><RotateCw size={16} /> 恢复最新备份</button>
          <button disabled={backups.length === 0} onClick={() => run(() => api("/api/workspaces/prune-storage-backups", {
            workspaceId: selected.id
          }, session.csrfToken))}><Trash2 size={16} /> 清理旧备份</button>
        </div>
        <BackupList backups={backups} />
      </Panel>

      <Panel title="近期活动" icon={<Activity />}>
        <EventList events={events} />
      </Panel>
    </div>
  );
}

function AdminPanel({ state, adminTopUp, setAdminTopUp, adminOps, session, run }) {
  const operator = adminOps.operator;

  return (
    <div className="adminStack">
      <div className="adminMetrics">
        <Metric icon={<UserRound />} label="账号数" value={operator?.accounts?.total ?? "暂无"} />
        <Metric icon={<Server />} label="OPL Workspace 总数" value={operator?.workspaces?.total ?? state.workspaces.length} />
        <Metric icon={<CheckCircle2 />} label="运行时就绪" value={readinessLabel(adminOps.runtime)} tone={readinessTone(adminOps.runtime)} />
        <Metric icon={<ShieldCheck />} label="生产就绪" value={readinessLabel(adminOps.production)} tone={readinessTone(adminOps.production)} />
      </div>

      <Panel title="账号" icon={<UserRound />}>
        <div className="accountSummary">
          <ResourceCard icon={<WalletCards />} title="当前账号" value={state.account.id} detail={`${money(state.account.balance)} 余额`} />
          <ResourceCard icon={<CreditCard />} title="冻结金额" value={money(state.account.frozen)} detail="计算与存储预留" />
          <ResourceCard icon={<AlertTriangle />} title="需要关注" value={operator?.workspaces?.needsAttention ?? 0} detail="失败或额度耗尽的 OPL Workspace 状态" />
        </div>
      </Panel>

      <Panel title="手工充值" icon={<CreditCard />}>
        <div className="adminForm">
          <label>
            账号
            <input value={adminTopUp.accountId} onChange={(event) => setAdminTopUp({ ...adminTopUp, accountId: event.target.value })} />
          </label>
          <label>
            金额
            <input type="number" min="1" value={adminTopUp.amount} onChange={(event) => setAdminTopUp({ ...adminTopUp, amount: Number(event.target.value) })} />
          </label>
          <label>
            原因
            <input value={adminTopUp.reason} onChange={(event) => setAdminTopUp({ ...adminTopUp, reason: event.target.value })} />
          </label>
          <button className="primary" disabled={!adminTopUp.accountId} onClick={() => run(() => api("/api/accounts/credit", adminTopUp, session.csrfToken))}>
            <CreditCard size={16} /> 增加余额
          </button>
        </div>
      </Panel>

      <div className="splitGrid">
        <Panel title="生产就绪" icon={<ShieldCheck />}>
          <ReadinessBlock readiness={adminOps.production} />
        </Panel>
        <Panel title="运行时就绪" icon={<Server />}>
          <ReadinessBlock readiness={adminOps.runtime} />
        </Panel>
      </div>

      <Panel title="对账与验证证据" icon={<ClipboardList />}>
        <div className="evidenceGrid">
          <ResourceCard icon={<Database />} title="对账保护" value={valueLabel(state.billingReconciliation?.guard?.status || "not_required")} detail={valueLabel(state.billingReconciliation?.guard?.reason || "billing_reconciliation_not_required")} />
          <ResourceCard icon={<History />} title="运行时操作" value={operator?.runtimeOperations?.total ?? state.runtimeOperations.length} detail={`${operator?.runtimeOperations?.failed ?? 0} 个失败`} />
          <ResourceCard icon={<Database />} title="存储备份" value={operator?.storageBackups?.total ?? state.storageBackups.length} detail={`${operator?.storageBackups?.available ?? 0} 个可用`} />
        </div>
      </Panel>
    </div>
  );
}

function Metric({ icon, label, value, tone = "" }) {
  return <div className={`metric ${tone}`}>{icon}<span>{label}</span><strong>{value}</strong></div>;
}

function Panel({ title, icon, children }) {
  return (
    <section className="panel">
      <div className="panelHeader">{icon}<h3>{title}</h3></div>
      {children}
    </section>
  );
}

function ResourceCard({ icon, title, value, detail }) {
  return (
    <article className="resourceCard">
      <div>{icon}<span>{title}</span></div>
      <strong>{value}</strong>
      <small>{detail}</small>
    </article>
  );
}

function PolicyGrid({ policy }) {
  if (!policy) return null;
  return (
    <div className="policyGrid">
      <Metric icon={<CreditCard />} label="冻结周期" value={`${policy.prepaidHoldDays} 天`} />
      <Metric icon={<Activity />} label="最低计费" value={`${policy.minimumBillableHours} 小时`} />
      <Metric icon={<WalletCards />} label="加价比例" value={`${Math.round(policy.markup * 100)}%`} />
      <Metric icon={<Server />} label="计算策略" value="额度耗尽自动停止" />
    </div>
  );
}

function BackupList({ backups }) {
  if (!backups.length) return <div className="empty">暂无存储备份。</div>;
  return (
    <div className="eventList">
      {backups.slice().reverse().slice(0, 6).map((backup) => (
        <article className="event" key={backup.id}>
          <strong>{valueLabel(backup.status)}</strong>
          <span>{backup.id}</span>
          <em>{backup.createdAt || backup.updatedAt}</em>
        </article>
      ))}
    </div>
  );
}

function ReadinessBlock({ readiness }) {
  if (!readiness) return <div className="empty">尚未检查就绪状态。</div>;
  return (
    <div className="readinessBlock">
      <span className={`pill ${readiness.ready ? "ok" : "danger"}`}>{readiness.ready ? "已就绪" : "阻塞"}</span>
      <EventList events={[
        ...(readiness.missingEnv || []).map((name) => ({ id: `env-${name}`, type: "missing.env", accountId: name })),
        ...(readiness.missingTools || []).map((name) => ({ id: `tool-${name}`, type: "missing.tool", accountId: name })),
        ...(readiness.failedChecks || []).map((name) => ({ id: `check-${name}`, type: "failed.check", accountId: name }))
      ]} />
    </div>
  );
}

function UsageList({ events }) {
  if (!events.length) return <div className="empty">暂无资源用量。</div>;
  return (
    <div className="eventList">
      {events.slice().reverse().slice(0, 12).map((event) => (
        <article className="event" key={event.id}>
          <strong>{event.resourceType === "compute" ? "Compute 小时" : "Storage GB-hour"}</strong>
          <span>{event.workspaceId}</span>
          <em>{Number(event.quantity || 0).toFixed(2)} {event.unit} · {money(event.amount)}</em>
        </article>
      ))}
    </div>
  );
}

function RequestUsageList({ events }) {
  if (!events.length) return <div className="empty">暂无请求用量。</div>;
  return (
    <div className="eventList">
      {events.slice().reverse().slice(0, 12).map((event) => (
        <article className="event" key={event.id}>
          <strong>{event.model || event.provider || "请求用量"}</strong>
          <span>{event.requestId}</span>
          <em>{Number(event.inputTokens || 0) + Number(event.outputTokens || 0)} tokens · {money(event.amount)}</em>
        </article>
      ))}
    </div>
  );
}

function WalletTransactionList({ events }) {
  if (!events.length) return <div className="empty">暂无钱包流水。</div>;
  return (
    <div className="eventList">
      {events.slice().reverse().slice(0, 12).map((event) => (
        <article className="event" key={event.id}>
          <strong>{eventTitle(event.type)}</strong>
          <span>{event.workspaceId || event.accountId}</span>
          <em>{money(event.amount)} · {money(event.balanceAfter)} 余额</em>
        </article>
      ))}
    </div>
  );
}

function ManualTopupList({ events }) {
  if (!events.length) return <div className="empty">暂无充值审计。</div>;
  return (
    <div className="eventList">
      {events.slice().reverse().slice(0, 12).map((event) => (
        <article className="event" key={event.id}>
          <strong>{event.reason || "手工充值"}</strong>
          <span>{event.targetAccountId}</span>
          <em>{money(event.amount)} · {valueLabel(event.status)}</em>
        </article>
      ))}
    </div>
  );
}

function EventList({ events }) {
  if (!events.length) return <div className="empty">暂无事件。</div>;
  return (
    <div className="eventList">
      {events.slice().reverse().slice(0, 12).map((event) => (
        <article className="event" key={event.id}>
          <strong>{eventTitle(event.type || event.operationType)}</strong>
          <span>{event.workspaceId || event.accountId || event.status}</span>
          <em>{event.amount !== undefined ? money(event.amount) : event.createdAt || event.updatedAt || ""}</em>
        </article>
      ))}
    </div>
  );
}
