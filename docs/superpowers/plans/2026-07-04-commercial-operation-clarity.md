# Commercial Operation Clarity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make every paid or destructive OPL Console operation show price impact, confirmation, progress, failure reason, and next action with concise Chinese UI copy.

**Architecture:** Keep the accepted Console shell and page layout. Add mutation semantics to route/action contracts first, normalize write API responses at the UI client boundary, then wire existing resource pages to shared confirmation, progress, result, and failure components.

**Tech Stack:** React, Ant Design, existing OPL route/action registries, Node API routes, `node:test` contract/UI tests.

---

### Task 1: Route And Action Mutation Contract

**Files:**
- Modify: `packages/console/ui/routes/opl-actions.js`
- Modify: `packages/console/ui/routes/opl-routes.js`
- Modify: `packages/contracts/opl-cloud-route-api-contract.json`
- Test: `tests/ui/commercial-console-surface.test.js`
- Test: `tests/contracts/route-api-contract.test.js`

- [ ] Add failing tests that require every paid or destructive action to declare `mutation`, `confirmation`, `operationTimeline`, and `failureVisible`.
- [ ] Run targeted tests and confirm they fail because the metadata is missing.
- [ ] Add metadata for compute create/destroy, storage create/destroy, attach/detach, workspace create, URL reset/delete, and admin cleanup.
- [ ] Use Chinese user-facing labels in action and route labels.
- [ ] Run targeted tests and confirm they pass.

### Task 2: API Operation Envelope

**Files:**
- Modify: `packages/console/ui/api/resources-api.js`
- Modify: `packages/console/ui/api/workspaces-api.js`
- Modify: `packages/console/ui/api/console-api.js`
- Test: `tests/ui/commercial-console-surface.test.js`

- [ ] Add failing tests that require write API clients to return an operation envelope with `ok`, `status`, `operationId`, `resourceId`, `failureReason`, `costImpact`, and `next` fields when available.
- [ ] Run targeted tests and confirm they fail because write clients return raw payloads.
- [ ] Add a small `operationEnvelope()` helper at the UI API boundary without changing server resource ownership logic.
- [ ] Wrap resource and workspace write clients with the helper.
- [ ] Run targeted tests and confirm they pass.

### Task 3: Shared UI Components

**Files:**
- Modify: `packages/console/ui/pages/shared/commercial-console.jsx`
- Test: `tests/ui/commercial-console-surface.test.js`

- [ ] Add failing tests for `OperationConfirmButton`, `OperationResultPanel`, and concise Chinese operation labels.
- [ ] Run targeted tests and confirm they fail because the components are missing.
- [ ] Implement `OperationConfirmButton` using Ant Design `Popconfirm`; use strong confirm text display for data-loss operations.
- [ ] Implement `OperationResultPanel` for submitted/completed/failed responses.
- [ ] Translate operation timeline status labels to Chinese.
- [ ] Run targeted tests and confirm they pass.

### Task 4: Resource Page Integration

**Files:**
- Modify: `packages/console/ui/pages/resources/ResourceProvisioningPages.jsx`
- Test: `tests/ui/commercial-console-surface.test.js`

- [ ] Add failing tests that require create/destroy/attach/detach pages to use confirmation, page-level pending state, result panel, and Chinese visible copy.
- [ ] Run targeted tests and confirm they fail on missing confirmation and English copy.
- [ ] Replace raw submit handlers with local pending/result state and confirmed submit controls.
- [ ] Replace English visible copy with concise Chinese copy while keeping technical IDs in metadata where useful.
- [ ] Make storage destroy strong-confirm and explicitly state `/data` data deletion.
- [ ] Run targeted tests and confirm they pass.

### Task 5: Verification And Commit

**Files:**
- All changed files

- [ ] Run `npm test`.
- [ ] Run `npm run build`.
- [ ] Run `git diff --check`.
- [ ] Commit the worktree with a focused message.
- [ ] Report branch name, commit, verification, and whether a preview server is running.
