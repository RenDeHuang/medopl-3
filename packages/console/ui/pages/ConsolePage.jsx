import React, { useEffect, useMemo, useState } from "react";
import {
  Activity,
  Copy,
  CreditCard,
  HardDrive,
  Link as LinkIcon,
  LogOut,
  Play,
  RefreshCw,
  RotateCw,
  Server,
  ShieldCheck,
  Square,
  Trash2,
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

function money(value) {
  return `¥${Number(value || 0).toFixed(2)}`;
}

function packageSummary(plan) {
  return `${plan.cpu} CPU / ${plan.memoryGb}GB memory / ${plan.diskGb}GB storage`;
}

function statusLabel(workspace) {
  if (!workspace) return "No workspace";
  const labels = {
    running: "Running",
    stopped_server_disk_retained: "Stopped, storage retained",
    server_destroyed_disk_retained: "Compute destroyed, storage retained",
    storage_hold_exhausted: "Storage hold exhausted",
    stopped_storage_hold_exhausted: "Stopped, storage hold exhausted",
    destroyed: "Destroyed",
    failed: "Failed"
  };
  return labels[workspace.state] || workspace.state;
}

function eventTitle(type) {
  return String(type || "event").replaceAll("_", " ").replaceAll(".", " · ");
}

export default function ConsolePage({ session, onLogout }) {
  const [state, setState] = useState(null);
  const [selectedId, setSelectedId] = useState("");
  const [error, setError] = useState("");
  const [adminTopUp, setAdminTopUp] = useState({ accountId: "", amount: 200, reason: "manual_top_up" });

  async function refresh() {
    const response = await fetch("/api/state");
    const next = await response.json();
    if (!response.ok) throw new Error(next.error || "state_failed");
    setState(next);
    setSelectedId((current) => current || next.workspaces[0]?.id || "");
  }

  async function run(action) {
    try {
      setError("");
      await action();
      await refresh();
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

  const selected = useMemo(() => state?.workspaces.find((item) => item.id === selectedId) || state?.workspaces[0], [state, selectedId]);
  const isAdmin = session.user.role === "admin";

  if (!state) {
    return (
      <div className="shell">
        <div className="loading">Loading Console...</div>
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
          <a href="#overview">Overview</a>
          <a href="#workspaces">Workspaces</a>
          <a href="#create">New Workspace</a>
          <a href="#access">Access URL</a>
          <a href="#billing">Billing</a>
          <a href="#activity">Activity</a>
          {isAdmin && <a href="#admin">Admin top-up</a>}
        </nav>
        <button className="ghost wide" onClick={logout}><LogOut size={16} /> Sign out</button>
      </aside>

      <main className="main">
        <header className="topbar" id="overview">
          <div>
            <p className="eyebrow">OPL Cloud</p>
            <h1>Workspace Console</h1>
            <p className="topbarText">Create and manage hosted OPL Workspaces from one prepaid account.</p>
          </div>
          <div className="accountBadge">
            <strong>{session.user.name || session.user.email}</strong>
            <span>{isAdmin ? "Admin" : "PI account"} · {state.account.id}</span>
          </div>
        </header>

        {error && <div className="error">{error}</div>}

        <section className="metrics">
          <Metric icon={<WalletCards />} label="Balance" value={money(state.account.balance)} />
          <Metric icon={<CreditCard />} label="Frozen holds" value={money(state.account.frozen)} />
          <Metric icon={<Server />} label="Workspaces" value={state.workspaces.length} />
          <Metric icon={<Activity />} label="Alerts" value={state.notifications.length} />
        </section>

        {isAdmin && (
          <section id="admin" className="band">
            <div className="sectionHeader">
              <div>
                <h2>Admin top-up</h2>
                <p>Manual recharge for commercial pilot accounts.</p>
              </div>
              <CreditCard />
            </div>
            <div className="adminForm">
              <label>
                Account
                <input value={adminTopUp.accountId} onChange={(event) => setAdminTopUp({ ...adminTopUp, accountId: event.target.value })} />
              </label>
              <label>
                Amount
                <input type="number" min="1" value={adminTopUp.amount} onChange={(event) => setAdminTopUp({ ...adminTopUp, amount: Number(event.target.value) })} />
              </label>
              <label>
                Reason
                <input value={adminTopUp.reason} onChange={(event) => setAdminTopUp({ ...adminTopUp, reason: event.target.value })} />
              </label>
              <button className="primary" disabled={!adminTopUp.accountId} onClick={() => run(() => api("/api/accounts/credit", adminTopUp, session.csrfToken))}>
                <CreditCard size={16} /> Add balance
              </button>
            </div>
          </section>
        )}

        <section id="create" className="band">
          <div className="sectionHeader">
            <div>
              <h2>Create OPL Workspace</h2>
              <p>Choose a package. OPL Cloud reserves compute and storage balance before opening the Workspace.</p>
            </div>
          </div>
          <div className="planGrid">
            {state.packages.map((plan) => (
              <article className="plan" key={plan.id}>
                <span>{plan.id === "basic" ? "Default" : "CPU"}</span>
                <h3>{plan.name}</h3>
                <p>{packageSummary(plan)}</p>
                <p>{money(plan.price.computeHourly)}/compute hour · {money(plan.price.storageGbMonth)}/GB-month storage</p>
                <button
                  className="primary"
                  onClick={() => run(() => api("/api/workspaces", {
                    workspaceName: plan.id === "basic" ? "Grant Lab" : "Protein Lab",
                    packageId: plan.id
                  }, session.csrfToken))}
                >
                  <Play size={16} /> Create {plan.id}
                </button>
              </article>
            ))}
          </div>
        </section>

        <section id="workspaces" className="band">
          <div className="sectionHeader">
            <div>
              <h2>Workspaces</h2>
              <p>Each Workspace has its own compute lifecycle, persistent storage, and URL token.</p>
            </div>
          </div>
          <div className="workspaceList">
            {state.workspaces.length === 0 && <div className="empty">No OPL Workspace yet. Ask an admin to add balance, then create a package.</div>}
            {state.workspaces.map((workspace) => (
              <button
                className={`workspaceRow ${workspace.id === selected?.id ? "active" : ""}`}
                key={workspace.id}
                onClick={() => setSelectedId(workspace.id)}
              >
                <span>{workspace.name}</span>
                <strong>{statusLabel(workspace)}</strong>
              </button>
            ))}
          </div>
        </section>

        <section id="access" className="band">
          <div className="sectionHeader">
            <div>
              <h2>Workspace URL</h2>
              <p>Share this URL with Workspace members. Members do not need Console accounts.</p>
            </div>
            <LinkIcon />
          </div>
          {selected ? (
            <div className="urlBox">
              <code>{selected.url}</code>
              <button disabled={selected.access.tokenStatus !== "active"} onClick={() => navigator.clipboard?.writeText(selected.url)}><Copy size={16} /> Copy</button>
              <button disabled={selected.access.tokenStatus !== "active"} onClick={() => run(() => api("/api/workspaces/reset-token", { workspaceId: selected.id }, session.csrfToken))}><RefreshCw size={16} /> Reset</button>
              <button className="danger" disabled={selected.access.tokenStatus !== "active"} onClick={() => run(() => api("/api/workspaces/delete-token", { workspaceId: selected.id }, session.csrfToken))}><Trash2 size={16} /> Delete</button>
            </div>
          ) : <div className="empty">Create a Workspace to get a URL.</div>}
        </section>

        <section className="band">
          <div className="sectionHeader">
            <div>
              <h2>Compute and storage</h2>
              <p>Stopping compute keeps storage. Destroying storage requires explicit confirmation.</p>
            </div>
            <Server />
          </div>
          {selected ? (
            <>
              <div className="resourceGrid">
                <ResourceCard icon={<Server />} title="Compute" value={`${selected.server.spec} / ${selected.server.status}`} detail={selected.server.billingStatus} />
                <ResourceCard icon={<HardDrive />} title="Storage" value={`${selected.disk.sizeGb}GB / ${selected.disk.status}`} detail={selected.disk.billingStatus} />
                <ResourceCard icon={<ShieldCheck />} title="Access" value={selected.access.tokenStatus} detail="URL token" />
                <ResourceCard icon={<CreditCard />} title="Package" value={selected.packageId} detail="Hourly billing" />
              </div>
              <div className="actions">
                <button onClick={() => run(() => api("/api/workspaces/stop-server", { workspaceId: selected.id, confirm: true }, session.csrfToken))}><Square size={16} /> Stop compute</button>
                <button onClick={() => run(() => api("/api/workspaces/restart-server", { workspaceId: selected.id }, session.csrfToken))}><RotateCw size={16} /> Restart</button>
                <button className="danger" onClick={() => run(() => api("/api/workspaces/destroy-server", { workspaceId: selected.id, confirm: true }, session.csrfToken))}><Trash2 size={16} /> Destroy compute</button>
                <button className="danger strong" onClick={() => run(() => api("/api/workspaces/destroy-disk", { workspaceId: selected.id, confirmDataLoss: true }, session.csrfToken))}><Trash2 size={16} /> Destroy storage</button>
                <button onClick={() => run(() => api("/api/billing/settle", {
                  workspaceId: selected.id,
                  hours: 1,
                  sourceEventId: `console_billing_tick_${Date.now()}`
                }, session.csrfToken))}><CreditCard size={16} /> Settle 1h</button>
              </div>
            </>
          ) : <div className="empty">No resource binding yet.</div>}
        </section>

        <section id="billing" className="band">
          <div className="sectionHeader">
            <div>
              <h2>Billing</h2>
              <p>Prepaid holds, hourly usage charges, and balance changes.</p>
            </div>
          </div>
          <PolicyGrid policy={state.billingPolicy} />
          <EventList events={state.billingLedger} />
        </section>

        <section id="activity" className="band">
          <div className="sectionHeader">
            <div>
              <h2>Activity</h2>
              <p>Workspace lifecycle and account alerts.</p>
            </div>
          </div>
          <EventList events={[...state.notifications, ...state.audit]} />
        </section>
      </main>
    </div>
  );
}

function Metric({ icon, label, value }) {
  return <div className="metric">{icon}<span>{label}</span><strong>{value}</strong></div>;
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
      <Metric icon={<CreditCard />} label="Hold window" value={`${policy.prepaidHoldDays} days`} />
      <Metric icon={<Activity />} label="Minimum charge" value={`${policy.minimumBillableHours}h`} />
      <Metric icon={<WalletCards />} label="Markup" value={`${Math.round(policy.markup * 100)}%`} />
      <Metric icon={<Server />} label="Compute policy" value="Auto-stop on exhaustion" />
    </div>
  );
}

function EventList({ events }) {
  if (!events.length) return <div className="empty">No events yet.</div>;
  return (
    <div className="eventList">
      {events.slice().reverse().slice(0, 12).map((event) => (
        <article className="event" key={event.id}>
          <strong>{eventTitle(event.type)}</strong>
          <span>{event.workspaceId || event.accountId}</span>
          <em>{event.amount !== undefined ? money(event.amount) : event.createdAt}</em>
        </article>
      ))}
    </div>
  );
}
