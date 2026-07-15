import assert from "node:assert/strict";
import test from "node:test";

import * as formatters from "../../apps/console-ui/src/pages/shared/formatters.ts";

test("runtime status opens only the local Workspace view without mutating persisted state", () => {
  const mergeWorkspaceRuntime = formatters.mergeWorkspaceRuntime;
  assert.equal(typeof mergeWorkspaceRuntime, "function");

  const selected = { id: "ws-alpha", state: "unready", openable: false, accessState: "distributing", access: { account: "opl" } };
  const merged = mergeWorkspaceRuntime(selected, {
    workspaceId: "ws-alpha",
    status: "running",
    ready: true,
    url: "https://workspace.medopl.cn/w/ws-alpha/",
    access: { username: "opl", password: "runtime-password-alpha" }
  });

  assert.equal(merged.openable, true);
  assert.equal(merged.accessState, "available");
  assert.equal(merged.access.password, "runtime-password-alpha");
  assert.equal(selected.access.password, undefined);
});
