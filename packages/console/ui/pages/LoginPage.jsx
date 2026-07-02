import React, { useState } from "react";
import { ArrowLeft, LogIn, ShieldCheck } from "lucide-react";

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
            <p className="eyebrow">Commercial Console</p>
            <h1>Sign in</h1>
          </div>
        </div>
        <form onSubmit={submit}>
          <label>
            Email
            <input value={email} onChange={(event) => setEmail(event.target.value)} type="email" autoComplete="email" required />
          </label>
          <label>
            Password
            <input value={password} onChange={(event) => setPassword(event.target.value)} type="password" autoComplete="current-password" required />
          </label>
          {error && <div className="error">{error}</div>}
          <button className="primary wide" disabled={submitting}>
            <LogIn size={16} /> {submitting ? "Signing in..." : "Sign in"}
          </button>
        </form>
        <div className="securityNote">
          <ShieldCheck size={16} />
          <span>Session uses an HttpOnly cookie and CSRF token for Console actions.</span>
        </div>
      </main>
    </div>
  );
}
