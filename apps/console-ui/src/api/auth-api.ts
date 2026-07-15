import { postJson } from "./console-api.ts";

export async function currentSession() {
  const response = await fetch("/api/auth/me", { signal: AbortSignal.timeout(10_000) });
  if (response.status === 401) return null;
  const payload = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(payload.safeMessage || payload.error || "session_check_failed");
  return payload?.user ? payload : null;
}

export function login(credentials) {
  return postJson("/api/auth/login", credentials);
}

export function logout(csrfToken) {
  return postJson("/api/auth/logout", {}, csrfToken);
}
