# Launch Invariants

This file is the mandatory human-readable launch contract for this implementation repository. The target product boundaries come from `https://github.com/gaofeng21cn/one-person-lab-cloud`; the revision reviewed for this freeze is `fdeb0e4df3e4905fb1c3551337b9dfda65bb2119`.

The upstream repository owns product architecture. This repository owns its selected backend, prices, provider procurement, delivery state, and runtime evidence. A frozen target is not a readiness claim. The current state and missing evidence for every launch slide are recorded below and in `packages/contracts/opl-cloud-launch-freeze-contract.json`.

## Console

- Console calls only Control Plane product APIs.
- Control Plane owns account-to-Sub2API mapping, service-plan quotes, monthly settlement orchestration, entitlements, renewal, expiry, and operator review.
- Console displays the live Sub2API USD balance. It never creates a second wallet, top-up ledger, or copied request-usage database.
- Basic is a `2c4g` customer service plan. Pro is an `8c16g` customer service plan.
- Basic is currently available. Pro is an approved product definition whose current implementation and launch evidence are incomplete; it must not be presented as ready until those gaps close.
- The internal Verification Slot is not a customer product and must never appear in the Console catalog or quote path.

## Fabric

- Fabric owns Tencent capacity checks, prepaid CVM/CBS procurement, provider operations, resource claims, attachments, and Workspace runtimes.
- Customer and verification CVM/CBS procurement uses `PREPAID`, a one-month period, and `NOTIFY_AND_MANUAL_RENEW`.
- `POSTPAID_BY_HOUR` is forbidden for customer and verification CVM/CBS resources.
- Capacity preflight is read-only. It must not buy or reserve a Tencent resource.
- Fabric must read back the CVM ID, CBS ID, charge type, billing period, and ownership tags before it reports a complete claim.
- Fabric owns provider facts and never changes Sub2API balance or Control Plane entitlement state.

## Ledger

- Ledger owns append-only reservation, fulfillment, claim, capture, release, renewal, expiry, review, and verification evidence.
- Ledger never owns or changes spendable balance.
- Receipts contain stable account, Workspace, billing-operation, provider-operation, resource, pricing, period, and redacted Gateway request references.
- API keys, passwords, raw tokens, provider secrets, and raw Sub2API responses are forbidden in evidence.
- A missing Ledger receipt is retryable and never repeats a reserve, capture, release, provider purchase, or provider renewal.

## Gateway

- OPL Gateway uses the externally deployed Sub2API backend.
- Sub2API is the only owner of spendable USD balance, reserved balance, API keys, model routing, and request usage.
- Gateway model requests use native Sub2API metering. Resource monthly settlement is a separate service-plan transaction even though it uses the same wallet.
- Monthly settlement requires Sub2API-owned `reserve`, `capture`, and `release`; a Control Plane-only hold is forbidden because concurrent model requests could spend the same balance.
- Workspace model traffic goes to Sub2API and must not be proxied through a generic Control Plane route.

## Monthly Settlement

The approved purchase protocol is:

```text
validate and quote
-> read-only prepaid capacity preflight
-> reserve exact monthly amount in Sub2API
-> provision prepaid CVM and CBS
-> claim and read back all provider resources
-> capture the reservation exactly once
-> activate entitlement
-> record receipts
```

- `reserve`, provider mutation, claim, `capture`, and `release` each have stable operation-scoped idempotency identities.
- Capture does not debit available balance a second time; it settles the amount already moved into reserved balance.
- Release is allowed only after Fabric proves that no billable resource exists or that provider cancellation and refund completed.
- A partial or ambiguous provider result keeps the balance reserved and enters `manual_review`.
- Retrying a provider or Ledger operation never reserves, captures, renews, or purchases twice.
- The current implementation does not satisfy this protocol: it prepares Fabric before a direct negative balance adjustment and has no generic Sub2API hold/capture/release client.

## Products And Lifecycle

- Basic resources are `2 CPU / 4 GB RAM`; Pro resources are `8 CPU / 16 GB RAM`.
- Customer prices remain exact integer CNY cents and USD micros under the versioned pricing contract.
- Provider SKU selection may vary by approved environment, but it must satisfy the customer CPU and memory contract.
- Renewal follows the same protocol: preflight, reserve, provider manual renewal, claim, capture, then extend `paidThrough`.
- Tencent automatic renewal is forbidden.
- Expired compute is stopped or destroyed under the approved provider lifecycle. Expired storage is retained and inaccessible until reactivated or explicitly destroyed.
- A verifier never deletes, replaces, renews, or reopens a customer resource.

## Workspace Access And Secrets

- Workspace URLs are stable and require Runtime password login.
- A routing cookie selects a Runtime Service and is not an authentication credential.
- Persisted Workspace, list, public, and evidence responses never expose raw credentials; only the authorized runtime-status command returns a password transiently.
- Workspace access requires active entitlement and real runtime readiness.
- Production manifests contain secret references only.
- Every production Console account has one positive integer `sub2apiUserId`.

## Verification Slot

`verification-slot-01` is the only real-resource launch verification slot:

| Property | Frozen value |
| --- | --- |
| CVM | `SA5.MEDIUM2` (`2c2g`) |
| CBS | `10GB` minimum prepaid volume |
| Provider billing | `PREPAID`, one month |
| Renewal | `NOTIFY_AND_MANUAL_RENEW` |
| Customer product | No |
| Concurrency | One verification run |
| Lifetime | Retain and reuse for the full paid period |

- Ordinary CI and commercial-chain E2E use fake Sub2API settlement and fake provider mutations.
- Runtime E2E reuses the fixed Slot, deploys the candidate Workspace image, authenticates, proves WebSocket behavior, makes one real model request with a dedicated test key, reads evidence, and removes only the temporary workload and test data.
- A run never purchases or deletes Tencent CVM, CBS, node-pool, or PV resources.
- Creating, renewing, or changing prepaid procurement for the Slot requires a separate explicit manual Provider Acceptance run.
- Production smoke is read-only and never changes monthly balance or provider resources.
- The current paid production verifier violates this contract and is forbidden for launch verification until replaced.

## Launch Slides

| Slide | Business | Owners | Current state | Required output and evidence |
| --- | --- | --- | --- | --- |
| 1. Offer and identity | A mapped customer sees the approved Basic and Pro definitions without an internal test SKU. | Console, Gateway | Basic login/mapping works; Pro was removed from the active catalog and needs implementation evidence. | Product contract, tenant tests, deployed mapped-account readback. |
| 2. Wallet and quote | Show live Sub2API balance and an exact monthly quote before side effects. | Console, Gateway | Live balance and Basic quote work; Pro quote is inactive. | Balance/quote contracts, unavailable-state UI, period and retention disclosure. |
| 3. Balance reservation | Protect the monthly amount from concurrent Gateway usage. | Console, Gateway, Ledger | Missing generic reserve/capture/release integration. | Sub2API APIs, stable reservation ID, concurrency/replay evidence, reservation receipt. |
| 4. Prepaid fulfillment | Open one-month prepaid CVM/CBS after reservation. | Fabric, Console | Blocked by `POSTPAID_BY_HOUR`; prepaid CBS is unproven. | PREPAID request tests, provider IDs/type/period/tags, duplicate-purchase protection. |
| 5. Claim and capture | Settle only after every provider resource is assigned. | Fabric, Console, Gateway, Ledger | Missing claim/capture protocol. | Claim identity, capture replay, safe release, partial-failure manual review. |
| 6. Workspace access | Open and authenticate to a ready Workspace. | Fabric, Console, Ledger | Readiness/password path exists; production browser/WebSocket proof is missing. | Post-login assertion, WebSocket 101, secret rotation, immutable image digest. |
| 7. Gateway usage | Use a dedicated Sub2API key and retain usage ownership in Sub2API. | Gateway, Console, Ledger | Balance/portal integration exists; model request and attribution are unproven. | Real model response, request ID, usage readback, no secret leakage. |
| 8. Renewal and recovery | Renew customer and Tencent periods once, with deterministic recovery. | All four layers | Customer entitlement worker exists; prepaid provider renewal and reservation settlement do not. | Renewal replay, provider readback, expiry, release/manual-review receipts. |
| 9. Reusable verification | Prove the release without buying or deleting monthly resources per run. | All four layers | Current verifier charges real monthly balance and creates/deletes resources. | Persistent SA5.MEDIUM2/CBS Slot, fake monthly settlement, real Workspace/Gateway proof, stable provider IDs. |
| 10. Production release | Declare ready from immutable artifacts and runtime/operations evidence. | All four layers | CI/rollout exists; readiness, restore, CLB/WebSocket, prepaid quota, and safe E2E evidence are incomplete. | CI, rollout, restore, monitoring, read-only smoke, and verification receipts. |

## Completion Rule

A slide is complete only when all of the following are true:

1. Its human and machine contracts are current.
2. The implementation and focused tests satisfy the contract.
3. Full CI passes on the merged revision.
4. The exact image digest is deployed.
5. Runtime and owner evidence listed by the slide is recorded.

Documentation, architecture alignment, a passing mock, or a green CI run alone never proves production delivery.
