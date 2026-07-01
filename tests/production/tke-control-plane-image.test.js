import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("OPL Cloud control-plane image build includes app build output and kubectl", async () => {
  const dockerfile = await readFile("Dockerfile", "utf8");
  const dockerignore = await readFile(".dockerignore", "utf8");

  assert.match(dockerfile, /FROM node:22/);
  assert.match(dockerfile, /npm ci --no-audit --no-fund --fetch-retries=5 --fetch-retry-mintimeout=20000 --fetch-retry-maxtimeout=120000/);
  assert.match(dockerfile, /npm ci --omit=dev --no-audit --no-fund --fetch-retries=5 --fetch-retry-mintimeout=20000 --fetch-retry-maxtimeout=120000/);
  assert.match(dockerfile, /npm run build/);
  assert.match(dockerfile, /kubectl/);
  assert.match(dockerfile, /EXPOSE 8787/);
  assert.match(dockerfile, /node", "services\/api\/server\.js"/);
  assert.match(dockerignore, /^\.env/m);
  assert.match(dockerignore, /^\.runtime/m);
  assert.match(dockerignore, /^node_modules/m);
});
