import { postJson } from "./console-api.ts";

export function previewPricing(input, csrfToken = "") {
  return postJson("/api/pricing/preview", input, csrfToken);
}
