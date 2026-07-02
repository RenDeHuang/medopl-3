import React, { useState } from "react";
import { ArrowLeft, LogIn, ShieldCheck } from "lucide-react";

function errorLabel(value) {
  const labels = {
    login_failed: "登录失败",
    invalid_credentials: "邮箱或密码不正确",
    not_authenticated: "请先登录",
    csrf_token_invalid: "页面安全令牌已失效，请重新登录"
  };
  return labels[value] || value;
}

export default function LoginPage({ onLogin }) {
  const [email, setEmail] = useState("pi@opl.local");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [submitting, setSubmitting] = useState(false);

  async function submit(event) {
    event.preventDefault();
    setSubmitting(true);
    setError("");
    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ email, password })
      });
      const payload = await response.json();
      if (!response.ok) throw new Error(payload.error || "login_failed");
      onLogin(payload);
    } catch (err) {
      setError(err.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div className="loginShell">
      <a className="backLink" href="#home"><ArrowLeft size={16} /> OPL Cloud</a>
      <main className="loginPanel">
        <div className="loginBrand">
          <div className="brandIcon">OPL</div>
          <div>
            <p className="eyebrow">商业版 OPL Console</p>
            <h1>登录</h1>
          </div>
        </div>
        <form onSubmit={submit}>
          <label>
            邮箱
            <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="email" required />
          </label>
          <label>
            密码
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" required />
          </label>
          {error && <div className="error">{errorLabel(error)}</div>}
          <button className="primary wide" disabled={submitting}>
            <LogIn size={16} /> {submitting ? "登录中..." : "登录"}
          </button>
        </form>
        <div className="securityNote">
          <ShieldCheck size={16} />
          <span>会话使用 HttpOnly Cookie 和 CSRF 令牌保护 OPL Console 操作。</span>
        </div>
      </main>
    </div>
  );
}
