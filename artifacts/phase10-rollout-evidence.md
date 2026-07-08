# Phase 10 Rollout Evidence

Date: 2026-07-08

## Completed local gates

- `npm test`
- `npm run build`
- `go test -count=1 ./...` in `services/control-plane`
- `go test -count=1 ./...` in `services/ledger`
- `go test -count=1 ./...` in `services/fabric`
- `sentrux check .`
- `git diff --check`

## Local browser screenshots

Generated with Playwright against local Vite and mocked backend facts:

- `artifacts/phase10-screenshots/home.png`
- `artifacts/phase10-screenshots/billing.png`
- `artifacts/phase10-screenshots/workspace-detail.png`
- `artifacts/phase10-screenshots/admin-ledger.png`

The local browser pass had no console errors. `admin-ledger.png` was regenerated after fixing Ledger `amountCents` rendering so the Admin Ledger page no longer shows `NaN`.

## Real E2E blockers

These are external environment blockers, not passing evidence:

- `npm run staging:readiness` fails with `staging_env_file_missing:/home/dev/medopl-3/.env.staging.local`.
- `OPL_CONFIRM_REAL_CLOUD_E2E=1 npm run staging:e2e` fails with `staging_env_file_missing:/home/dev/medopl-3/.env.staging.local`.
- `npm run verify:production` fails with `origin_required` because `OPL_CONSOLE_ORIGIN` is not set.

## GitHub rollout sequence

After `commercial-chain-closure` is pushed, run the production workflows in this order:

1. `Release OPL Cloud Image`
2. `Deploy TKE Production`
3. `Diagnose TKE Production`
4. `Verify Production Chain`

The verifier now fails unless the live chain proves:

- Workspace token handoff redirects to a URL without `token`.
- Ledger debit entries include `pricingVersion`, `priceSnapshot`, quantity/unit, and provider cost evidence refs.
- Wallet transactions include wallet-after balance, frozen, available, and total spent fields.
- Fabric evidence includes operation IDs, provider cost tags, and links back to Ledger rows.

Recommended manual dispatch:

```bash
gh workflow run verify-production-chain.yml \
  --ref commercial-chain-closure \
  -f origin=https://cloud.medopl.cn \
  -f account_id=pi-production-verifier \
  -f package_id=basic
```

## Required next inputs

- Create `/home/dev/medopl-3/.env.staging.local` or set `OPL_STAGING_ENV_FILE` to a valid staging env file.
- Set `OPL_CONSOLE_ORIGIN=https://<console-domain>` for production verification.
- Keep `OPL_CONFIRM_REAL_CLOUD_E2E=1` only when intentionally creating and destroying real Tencent Cloud resources.

The staging env file can start from `deploy/tke/opl-cloud-staging.local.env.example`. The readiness gate requires these non-empty values:

- `OPL_RUNTIME_PROVIDER=tencent-tke`
- `DATABASE_URL`
- `OPL_WORKSPACE_IMAGE`
- `OPL_WORKSPACE_DOMAIN`
- `OPL_K8S_NAMESPACE`
- `OPL_INGRESS_CLASS`
- `OPL_WORKSPACE_STORAGE_CLASS`
- `OPL_IMAGE_PULL_SECRET_NAME`
- `OPL_TENCENT_PROVISIONER_BIN`
- `TENCENTCLOUD_SECRET_ID`
- `TENCENTCLOUD_SECRET_KEY`
- `TENCENTCLOUD_REGION`
- `TENCENT_DEPLOY_CLUSTER_ID`
- `TENCENT_DEPLOY_KUBECONFIG_REF`
- `TENCENT_CVM_SUBNET_ID`
- `TENCENT_CVM_SECURITY_GROUP_IDS`
- `OPL_BASIC_COMPUTE_INSTANCE_TYPE`
