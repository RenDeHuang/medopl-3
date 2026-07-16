import assert from "node:assert/strict";
import { afterEach, test } from "node:test";

import { previewPricing } from "../../apps/console-ui/src/api/console-read-api.ts";

const originalFetch = globalThis.fetch;
afterEach(() => { globalThis.fetch = originalFetch; });

test("storage quote comes from the authenticated pricing preview API", async () => {
  let request: RequestInit | undefined;
  let url = "";
  globalThis.fetch = async (input, init) => {
    url = String(input);
    request = init;
    return new Response(JSON.stringify({ chargeUsdMicros: 25_714_286 }), {
      status: 200,
      headers: { "content-type": "application/json" }
    });
  };

  const quote = await previewPricing({ resourceType: "storage", packageId: "pro", sizeGb: 100 }, "csrf-1");

  assert.equal(url, "/api/pricing/preview");
  assert.equal(request?.method, "POST");
  assert.equal(new Headers(request?.headers).get("x-opl-csrf"), "csrf-1");
  assert.deepEqual(JSON.parse(String(request?.body)), { resourceType: "storage", packageId: "pro", sizeGb: 100 });
  assert.equal(quote.chargeUsdMicros, 25_714_286);
});
