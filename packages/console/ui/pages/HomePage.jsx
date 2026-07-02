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
          <a href="#home">首页</a>
          <a href="#console">OPL Console</a>
          <a className="navButton" href={session ? "#console" : "#login"}>{session ? "打开 OPL Console" : "登录"}</a>
        </nav>
      </header>

      <main>
        <section className="homeHero">
          <div className="heroCopy">
            <p className="eyebrow">托管式 OPL Workspace 交付</p>
            <h1>OPL Cloud</h1>
            <p>
              Lab owner 通过一个账号创建托管 OPL Workspace，获得稳定 URL，
              并使用预付余额计费。OPL Fabric 和 OPL Ledger 在后台处理资源与账务。
            </p>
            <div className="heroActions">
              <a className="primaryLink" href={session ? "#console" : "#login"}>打开 OPL Console <ArrowRight size={16} /></a>
              <a className="secondaryLink" href="#console">查看 OPL Workspace</a>
            </div>
          </div>
          <div className="heroPreview" aria-label="OPL Console 预览">
            <div className="previewTop">
              <span />
              <strong>OPL Workspace 交付链路</strong>
            </div>
            <div className="chainPreview">
              <PreviewStep icon={<WalletCards />} title="预付余额" text="7 天计算与存储预冻结" />
              <PreviewStep icon={<Cloud />} title="OPL Fabric" text="计算资源、Docker 运行时、存储卷" />
              <PreviewStep icon={<ShieldCheck />} title="稳定 URL" text="通过令牌控制 OPL Workspace 访问" />
              <PreviewStep icon={<Database />} title="OPL Ledger" text="计费、审计、证据回执" />
            </div>
          </div>
        </section>

        <section className="homeBand">
          <article>
            <CheckCircle2 />
            <h2>面向 lab owner 的 OPL Console</h2>
            <p>创建、停止、重启和分享 OPL Workspace，不向普通用户暴露底层 Kubernetes 或云厂商证据。</p>
          </article>
          <article>
            <CheckCircle2 />
            <h2>内置预付计费</h2>
            <p>余额、冻结金额、小时结算和操作历史集中展示。</p>
          </article>
          <article>
            <CheckCircle2 />
            <h2>管理员充值路径</h2>
            <p>商业版充值由管理员账号处理，lab owner 不会看到演示额度控件。</p>
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
