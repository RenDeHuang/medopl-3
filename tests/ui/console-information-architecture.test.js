import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const sourcePath = new URL("../../packages/console/ui/pages/ConsolePage.jsx", import.meta.url);

test("OPL Console navigation fixes Workspace delivery as the primary commercial IA", async () => {
  const source = await readFile(sourcePath, "utf8");

  assert.match(source, /href="#workspaces"[\s\S]*OPL Workspace/);
  assert.match(source, /href="#create"[\s\S]*创建 OPL Workspace/);
  assert.match(source, /href="#billing"[\s\S]*账单/);
  assert.match(source, /href="#admin"[\s\S]*管理员/);

  assert.doesNotMatch(source, /href="#access"[\s\S]*Access URL/);
  assert.doesNotMatch(source, /href="#activity"[\s\S]*Activity/);
  assert.doesNotMatch(source, /Compute and storage<\/h2>/);

  assert.match(source, /id="workspaces"[\s\S]*OPL Workspace URL[\s\S]*计算[\s\S]*存储[\s\S]*备份/);
  assert.match(source, /id="admin"[\s\S]*账号[\s\S]*手工充值[\s\S]*生产就绪/);
});
