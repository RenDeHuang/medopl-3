import { getJson, postJson } from "./console-api.ts";

export function getSupportTicketMappings({ all = false }: any = {}) {
  const params = all ? "?scope=all" : "";
  return getJson(`/api/support/tickets${params}`);
}

export function createSupportTicketMapping(input, csrfToken) {
  return postJson("/api/support/tickets", input, csrfToken);
}
