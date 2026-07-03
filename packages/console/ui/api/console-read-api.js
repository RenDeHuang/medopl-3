import { getJson, postJson } from "./console-api.js";

export function getConsoleState() {
  return getJson("/api/state");
}

export function getOperatorSummary() {
  return getJson("/api/operator/summary");
}

export function getRuntimeReadiness() {
  return getJson("/api/runtime/readiness");
}

export function getProductionReadiness() {
  return getJson("/api/production/readiness");
}

export function getManagementState(organizationId) {
  const params = new URLSearchParams();
  if (organizationId) params.set("organizationId", organizationId);
  const query = params.toString();
  return getJson(`/api/management/state${query ? `?${query}` : ""}`);
}

export function createUser(input, csrfToken) {
  return postJson("/api/users", input, csrfToken);
}
