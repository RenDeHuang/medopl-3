# OPL Console Design QA

> Historical/superseded checkpoint: the evidence below describes UI product commit
> `5fc1f7ab364d8e34b851d9c5c467ddcda88d9352`. It is not current Task 12 product truth.
> Task 13 replaces the final evidence with screenshots of the integrated implementation.

## Task 13 Final Local Evidence

- Available-state screenshots:
  - `output/design-qa/task13/customer-workspace-desktop.png`
  - `output/design-qa/task13/customer-api-overview-desktop.png`
  - `output/design-qa/task13/customer-api-usage-desktop.png`
  - `output/design-qa/task13/customer-billing-desktop.png`
  - `output/design-qa/task13/admin-overview-desktop.png`
  - `output/design-qa/task13/admin-accounts-desktop.png`
  - `output/design-qa/task13/admin-resources-desktop.png`
- Mobile screenshots:
  - `output/design-qa/task13/customer-overview-mobile.png`
  - `output/design-qa/task13/customer-workspace-mobile.png`
  - `output/design-qa/task13/customer-api-usage-mobile.png`
  - `output/design-qa/task13/admin-overview-mobile.png`
  - `output/design-qa/task13/admin-accounts-mobile.png`
- Unavailable-state screenshots:
  - `output/design-qa/task13/auth-unavailable-desktop.png`
  - `output/design-qa/task13/customer-catalog-unavailable-desktop.png`
  - `output/design-qa/task13/customer-workspace-runtime-unavailable-desktop.png`
  - `output/design-qa/task13/admin-system-runtime-unavailable-desktop.png`
- The browser used a local deterministic HTTP fixture that returned the strict
  Task 12 DTOs. It made no Sub2API, Tencent, production, payment, or model call.
- API values were asserted against the rendered Workspace, Wallet, Key, Usage,
  Usage Stats, balance-history, receipt, account, resource, and readiness facts.
- Each source was then made unavailable independently. The UI removed its prior
  value, rendered `暂不可用`, and preserved facts from the remaining sources.
- Runtime and API Key reveal were POST-only in the browser flow; plaintext was
  removed after hide or route leave and was never captured in a screenshot.
- Desktop `1440x900` and mobile `375x812` passed document-overflow, navigation,
  table containment, long-URL wrapping, and console-error checks in available states.

Task 13 browser result: passed locally; production evidence remains pending.

## Task 12 Visual Freeze

- `output/imagegen/task12-freeze-v2/customer-workspace-overview-v2.png`
- `output/imagegen/task12-freeze-v2/customer-api-service-v2.png`
- `output/imagegen/task12-freeze-v2/admin-accounts-resources.png`
- These images freeze visual hierarchy only. Their example values are not data, price, or status authority.

## Comparison Target

- Product checkpoint: `5fc1f7ab364d8e34b851d9c5c467ddcda88d9352`.
- Source visual truth:
  - `/home/dev/.codex/generated_images/opl-gateway-usage-20260717.png`
  - `/home/dev/.codex/generated_images/opl-gateway-api-keys-unified-shell-20260717.png`
  - `/home/dev/.codex/generated_images/opl-admin-operations-20260717-v2.png`
- Implementation screenshots:
  - `/home/dev/medopl-3/output/design-qa/gateway-usage-desktop.png`
  - `/home/dev/medopl-3/output/design-qa/gateway-keys-desktop.png`
  - `/home/dev/medopl-3/output/design-qa/admin-overview-desktop.png`
  - `/home/dev/medopl-3/output/design-qa/gateway-usage-tablet.png`
  - `/home/dev/medopl-3/output/design-qa/gateway-usage-mobile.png`
  - `/home/dev/medopl-3/output/design-qa/admin-overview-mobile.png`
- Full-view and focused comparison evidence: `/home/dev/medopl-3/output/design-qa/comparison.png`
- Desktop viewport: `1440x900`; responsive checks: `768x1024` and `375x812`.
- State: real local Control Plane session and API routes. Admin projections returned `200`; local Gateway upstream returned `502`, so Gateway screenshots show the implemented unavailable state rather than invented usage or Key data.
- Artifact policy: `output/` contains local generated screenshots and comparison files. It is intentionally ignored and is not part of the product commit.

## Findings

- No actionable P0, P1, or P2 findings remain.
- The ordinary Console navigation remains visible for the administrator, with an additive operations section.
- Gateway Usage and API Keys preserve the reference hierarchy, grouped metrics, tabs, tables, pagination, and unavailable states.
- Admin omits the reference's "latest verification" row because the current API has no trustworthy production E2E fact.

## Fidelity Surfaces

- Fonts and typography: existing Inter/system stack, weights, line heights, wrapping, and zero letter spacing are consistent across all routes.
- Spacing and layout: sidebar, tabs, grouped metrics, tables, and admin panels match the reference density; no page-level overflow at tested viewports.
- Colors and tokens: existing blue, neutral, success, and error tokens are reused; no new palette or decorative gradient was introduced.
- Image and icon quality: existing OPL logo and installed Lucide icon family are used; no placeholder, custom SVG, or CSS illustration was added.
- Copy and content: customer-facing internal terms remain removed; readiness is labeled as dependency status rather than production verification.

## Patches Made

- Cleared revealed Gateway Key state when leaving the API Keys route.
- Cleared any previously revealed Key before refresh or reveal requests so a failed request cannot leave plaintext on screen.
- Removed the redundant Gateway summary error from Usage while keeping independent stats and list retries.
- Renamed readiness rows to "运行依赖 / 生产依赖" to match the actual endpoint semantics.
- Grouped Gateway Usage metrics into one summary surface and aligned the period control with the reference.
- Verified the mobile sidebar after its 180ms transition; its final bounds are `0-292px` with no clipping.

## Follow-up Polish

- P3 only: the reference's decorative empty-state tray icon is intentionally omitted; the real empty/error copy remains clear without adding nonessential decoration.

historical checkpoint result: passed for `5fc1f7a` only
