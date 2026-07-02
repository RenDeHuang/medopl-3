import React from "react";
import { ArrowRight, CheckCircle2, Cloud, Database, ShieldCheck, WalletCards } from "lucide-react";

export default function HomePage({ session }) {
  return (
    <div className="publicShell">
      <header className="publicNav">
        <a className="wordmark" href="#home">
          <span>OPL</span>
          <strong>OPL Cloud</strong>
        </a>
        <nav className="publicLinks">
          <a href="#home">Home</a>
          <a href="#console">Console</a>
          <a className="navButton" href={session ? "#console" : "#login"}>{session ? "Open Console" : "Login"}</a>
        </nav>
      </header>

      <main>
        <section className="homeHero">
          <div className="heroCopy">
            <p className="eyebrow">Managed OPL Workspace delivery</p>
            <h1>OPL Cloud</h1>
            <p>
              One PI account opens hosted OPL Workspaces, receives stable URL access,
              and pays from a prepaid balance while OPL Fabric and OPL Ledger handle the platform work behind the scenes.
            </p>
            <div className="heroActions">
              <a className="primaryLink" href={session ? "#console" : "#login"}>Open Console <ArrowRight size={16} /></a>
              <a className="secondaryLink" href="#console">View workspaces</a>
            </div>
          </div>
          <div className="heroPreview" aria-label="OPL Console preview">
            <div className="previewTop">
              <span />
              <strong>Workspace delivery</strong>
            </div>
            <div className="chainPreview">
              <PreviewStep icon={<WalletCards />} title="Prepaid balance" text="7-day compute and storage hold" />
              <PreviewStep icon={<Cloud />} title="OPL Fabric" text="Compute, Docker runtime, storage volume" />
              <PreviewStep icon={<ShieldCheck />} title="Stable URL" text="Token-gated Workspace access" />
              <PreviewStep icon={<Database />} title="OPL Ledger" text="Billing, audit, evidence receipts" />
            </div>
          </div>
        </section>

        <section className="homeBand">
          <article>
            <CheckCircle2 />
            <h2>Console for PI users</h2>
            <p>Create, stop, restart, and share OPL Workspaces without exposing low-level Kubernetes or provider evidence.</p>
          </article>
          <article>
            <CheckCircle2 />
            <h2>Built-in prepaid billing</h2>
            <p>Balances, frozen holds, hourly settlement, and activity history stay visible in one place.</p>
          </article>
          <article>
            <CheckCircle2 />
            <h2>Admin top-up path</h2>
            <p>Commercial recharge is handled by an admin account, so PI users never see demo credit controls.</p>
          </article>
        </section>
      </main>
    </div>
  );
}

function PreviewStep({ icon, title, text }) {
  return (
    <article className="previewStep">
      {icon}
      <div>
        <strong>{title}</strong>
        <span>{text}</span>
      </div>
    </article>
  );
}
