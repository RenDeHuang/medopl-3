import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const root = new URL("../../", import.meta.url);

async function text(path: string) {
  return readFile(new URL(path, root), "utf8");
}

async function json(path: string) {
  return JSON.parse(await text(path));
}

test("Pilot V2 contracts hard cut Gateway keys and source envelopes", async () => {
  const [freeze, sourceTruth, boundary, dtos] = await Promise.all([
    json("packages/contracts/opl-cloud-launch-freeze-contract.json"),
    json("packages/contracts/opl-cloud-console-source-truth-contract.json"),
    json("packages/contracts/opl-cloud-service-boundary-contract.json"),
    text("apps/console-ui/src/api/dtos.ts")
  ]);

  assert.deepEqual(freeze.deliveryEvidence, {
    required: true,
    codeComplete: false,
    pilotReady: false,
    productionProven: false,
    saleable: false
  });
  assert.equal(freeze.gateway.publicEndpoint.configEnv, "OPL_GATEWAY_PUBLIC_BASE_URL");
  assert.equal(freeze.gateway.publicEndpoint.productApi, "GET /api/gateway/endpoint");
  assert.equal(freeze.gateway.publicEndpoint.productionScheme, "https");
  assert.deepEqual(freeze.gateway.publicEndpoint.forbiddenFallbacks, ["OPL_SUB2API_BASE_URL", "gflabtoken.cn"]);
  assert.deepEqual(freeze.gateway.customerMutationApis, ["create_general_key", "update_general_key", "delete_general_key", "reveal_owned_key"]);
  assert.equal(freeze.gateway.createKeyRequest.expiryField, "expiresInDays");
  assert.equal(freeze.gateway.createKeyRequest.responseExpiryField, "expiresAt");
  assert.equal(freeze.gateway.createKeyRequest.createThenUpdate, false);

  assert.equal(sourceTruth.envelope.typeName, "SourceEnvelope<T>");
  assert.equal(sourceTruth.envelope.serverWriter, "writeSourceEnvelope");
  assert.equal(sourceTruth.envelope.fetchedAtMaySubstituteSourceUpdatedAt, false);
  assert.equal(sourceTruth.sources.gateway.endpoint.route, "GET /api/gateway/endpoint");
  assert.equal(sourceTruth.sources.gateway.keys.createRequest.expiryField, "expiresInDays");
  assert.equal(sourceTruth.sources.gateway.keys.revealRoute, "POST /api/gateway/keys/{keyId}/reveal");
  assert.equal(sourceTruth.sources.gateway.usage.route, "GET /api/gateway/keys/{keyId}/usage");
  assert.equal(sourceTruth.sources.gateway.usageStats.route, "GET /api/gateway/keys/{keyId}/usage-summary");
  assert.equal(sourceTruth.sources.gateway.accountUsageStats.route, "GET /api/gateway/usage-summary");

  assert.deepEqual(boundary.customerMutationBoundary, { payment: false, topUp: false, keyCreate: true, keyRevoke: true });
  assert.ok(boundary.externalServices.gateway.controlPlaneApi.includes("mutate owned general keys with delegated user credentials"));
  assert.doesNotMatch(dtos, /ProductSourceEnvelope/);
  assert.match(dtos, /type SourceEnvelope<T>/);
  for (const name of [
    "MoneyDTO", "OperationStatusDTO", "SessionDTO", "CurrentAccountDTO", "GatewayWalletDTO", "GatewayBalanceHistoryPageDTO",
    "GatewayEndpointDTO", "GatewayKeyPageDTO", "GatewayKeySummaryDTO", "CreateGatewayKeyRequest",
    "UpdateGatewayKeyRequest", "GatewayKeySecretDTO", "GatewayKeyUsagePageDTO",
    "GatewayUsageSummaryDTO", "GatewayAccountUsageSummaryDTO", "WorkspaceDTO",
    "WorkspaceLaunchRequest", "WorkspaceLaunchOperationDTO", "WorkspaceKeyRotationDTO",
    "WorkspaceRuntimeDTO", "WorkspaceFileEntryDTO", "WorkspaceFilePageDTO",
    "WorkspaceFilesystemUsageDTO", "BillingReceiptPageDTO", "WorkspaceBillingReceiptDTO",
    "AnnouncementPageDTO", "AnnouncementDTO", "AnnouncementReadDTO", "OperatorOverviewDTO", "OperatorUsageCostDTO",
    "OperatorAccountPageDTO", "OperatorAccountDTO", "InviteAccountRequest",
    "OperatorAccountCommandDTO", "WalletAdjustmentRequest", "WalletAdjustmentOperationDTO",
    "OperatorWorkspacePageDTO", "OperatorWorkspaceDTO", "WorkspaceRuntimeCredentialDTO",
    "WorkspaceAutoRenewRequest", "WorkspaceAutoRenewCommandDTO", "OperatorReconciliationPageDTO",
    "BillingReviewResolutionRequest", "OperatorHealthDTO", "OperatorAnnouncementPageDTO",
    "AnnouncementDraftRequest", "AnnouncementScheduleRequest"
  ]) {
    assert.match(dtos, new RegExp(`export (?:interface|type) ${name}\\b`), `missing ${name}`);
  }
  assert.match(dtos, /interface CreateGatewayKeyRequest[\s\S]*expiresInDays/);
  const rotationDTO = dtos.match(/export interface WorkspaceKeyRotationDTO[\s\S]*?\n}/)?.[0] ?? "";
  assert.match(rotationDTO, /workspaceApiKeyId:\s*string;/);
  assert.doesNotMatch(rotationDTO, /\n\s+keyId:\s*string;/);
});

test("Pilot V2 contracts hard cut Workspace purchase, access, and Runtime facts", async () => {
  const [freeze, billing, business, product, evidence] = await Promise.all([
    json("packages/contracts/opl-cloud-launch-freeze-contract.json"),
    json("packages/contracts/opl-cloud-billing-ledger-contract.json"),
    json("packages/contracts/opl-cloud-business-object-contract.json"),
    json("packages/contracts/opl-cloud-product-contract.json"),
    json("packages/contracts/opl-cloud-evidence-ledger-contract.json")
  ]);

  assert.equal(freeze.workspaceLaunch.customerDebitCardinality, 1);
  assert.deepEqual(freeze.workspaceLaunch.fulfillmentResources, ["compute", "storage", "attachment", "gateway_secret", "runtime"]);
	assert.deepEqual(freeze.gateway.workspaceKeyLifecycle, {
		launchConvergence: "zero_create_one_active_reuse_other_fail_closed",
		rotationApi: "POST /api/workspaces/{workspaceId}/workspace-key/rotate",
		mutationCredential: "session_delegated_user_bearer",
		workspacePersistence: "workspace_api_key_id_only",
		operationPersistence: "control_plane_runtime_operations_non_secret_phases",
		phases: ["replacement_check", "replacement_create", "secret_write", "retire_old", "promote_replacement", "workspace_commit", "runtime_apply", "delete_old", "receipt", "complete"],
		receiptType: "workspace.gateway_key_rotated.v1",
		currentImplementation: "code_complete_local_focused_tests_only"
	});
  assert.equal(billing.chargePolicy.customerObject, "workspace");
  assert.equal(billing.chargePolicy.debitCardinalityPerPeriod, 1);
  assert.equal(billing.ledgerEvidencePolicy.workspaceReceiptTypes.purchased, "billing.workspace_purchased.v1");
  assert.equal(billing.entitlementPolicy.resourceCompatibility.customerChargeOwner, false);
  assert.ok(evidence.receiptTypes.includes("billing.workspace_purchased.v1"));
  assert.ok(evidence.receiptTypes.includes("workspace.gateway_key_rotated.v1"));
  assert.ok(evidence.receiptTypes.includes("gateway.wallet_adjustment.v1"));

  const workspace = business.objectKinds.find((entry: { kind: string }) => entry.kind === "Workspace");
  assert.ok(workspace.requiredFields.includes("workspaceApiKeyId"));
  assert.deepEqual(workspace.accessQuestions, ["url", "username", "passwordRevealCopy", "workspaceKeyRevealCopy"]);
  assert.equal(workspace.workspaceKeyRevealRoute, "POST /api/gateway/keys/{keyId}/reveal");
	assert.equal(workspace.workspaceKeyRotationRoute, "POST /api/workspaces/{workspaceId}/workspace-key/rotate");
	assert.equal(workspace.workspaceKeyPersistence, "workspace_api_key_id_only");
	assert.deepEqual(workspace.workspaceKeyRotationDTOFields, ["operationId", "workspaceId", "status", "workspaceApiKeyId", "fingerprint", "updatedAt", "receiptId"]);
	assert.equal(evidence.workspaceGatewayKeyRotationReceipt.implementation, "validator_and_control_plane_exact_readback_code_complete_local_only");
  assert.deepEqual(product.workspaceRuntimeFacts, {
    fileMetadataAuthority: "workspace_runtime_projects_mount",
    filesystemUsageAuthority: "workspace_runtime_statfs",
    persistence: "none"
  });
});

test("Pilot V2 contracts hard cut operator resources, wallet adjustments, and announcements", async () => {
  const [management, sourceTruth, business, boundary, evidence, billing] = await Promise.all([
    json("packages/contracts/opl-cloud-management-contract.json"),
    json("packages/contracts/opl-cloud-console-source-truth-contract.json"),
    json("packages/contracts/opl-cloud-business-object-contract.json"),
    json("packages/contracts/opl-cloud-service-boundary-contract.json"),
    json("packages/contracts/opl-cloud-evidence-ledger-contract.json"),
    json("packages/contracts/opl-cloud-billing-ledger-contract.json")
  ]);

  assert.deepEqual(management.api.operatorAccounts, {
    list: "GET /api/operator/accounts",
    invite: "POST /api/operator/accounts/invitations",
    disable: "POST /api/operator/accounts/{accountId}/disable",
    delete: false
  });
  assert.deepEqual(management.api.operatorReads, {
    overview: "GET /api/operator/overview",
    accounts: "GET /api/operator/accounts",
    workspaces: "GET /api/operator/workspaces",
    workspaceDetail: "GET /api/operator/workspaces/{workspaceId}",
    reconciliation: "GET /api/operator/reconciliation",
    health: "GET /api/operator/health"
  });
  assert.deepEqual(management.operatorProjection.sub2apiReads, {
    users: "GET /api/v1/admin/users",
    usersUsage: "POST /api/v1/admin/dashboard/users-usage",
    apiKeysUsage: "POST /api/v1/admin/dashboard/api-keys-usage",
    batchSizeMax: 50
  });
  assert.equal(management.operatorProjection.perAccountUserOrUsageNPlusOne, false);
  assert.equal(management.operatorProjection.persistence, "none_request_join_only");
  assert.equal(management.operatorProjection.readReplica, false);
  assert.equal(management.operatorProjection.partialFailure, "affected_nested_source_unavailable_without_zero_data");
  assert.deepEqual(management.walletAdjustments.kinds, ["recharge", "debit", "business_refund"]);
  assert.equal(management.walletAdjustments.balanceAuthority, "sub2api");
  assert.deepEqual(management.walletAdjustments.routes, {
    create: "POST /api/operator/accounts/{accountId}/wallet-adjustments",
    read: "GET /api/operator/wallet-adjustments/{operationId}"
  });
  assert.equal(management.walletAdjustments.unknownResult, "manual_review_without_automatic_replay");
  assert.equal(management.walletAdjustments.implementation, "code_complete_local_focused_tests");
  assert.deepEqual(evidence.gatewayWalletAdjustmentReceipt.commonRequiredRefs, ["operationId", "kind", "amountUsdMicros", "balanceHistoryRef", "actor"]);
  assert.deepEqual(evidence.gatewayWalletAdjustmentReceipt.businessRefundAdditionalRequiredRefs, ["relatedOperationId"]);
  assert.equal(evidence.gatewayWalletAdjustmentReceipt.implementation, "validator_and_control_plane_exact_readback_code_complete_local_only");
  assert.equal(billing.walletAdjustmentEvidence.balanceAuthority, "sub2api");
  assert.equal(billing.walletAdjustmentEvidence.controlPlaneState, "runtime_operation_non_authoritative");
  assert.equal(billing.walletAdjustmentEvidence.ledgerState, "append_only_reference_non_authoritative");
  assert.equal(billing.walletAdjustmentEvidence.localBalancePersistence, false);
  assert.equal(management.announcements.owner, "control_plane_postgresql");
  assert.deepEqual(management.announcements.tables, ["control_plane_announcements", "control_plane_announcement_reads"]);
  assert.equal(boundary.services.controlPlane.owns.includes("announcements"), true);

  const resource = sourceTruth.sources.operator.resources;
  assert.deepEqual(resource.requiredFields, [
    "ownerAccount", "ownerUser", "workspace", "resourceType", "packageOrSpec", "providerId", "zone",
    "status", "createdAt", "expiresAt", "lastReadAt", "operationRef", "receiptRef"
  ]);
  assert.equal(resource.fabricAndLedgerPersistenceInControlPlane, false);
  assert.equal(sourceTruth.sources.identity.operatorAccounts.pagination, "one_bounded_sub2api_user_page_then_control_plane_page");
  assert.equal(sourceTruth.sources.identity.operatorAccounts.failure, "affected_nested_source_unavailable_without_zero_data");
  assert.deepEqual(sourceTruth.sources.operator.routes, {
    overview: "GET /api/operator/overview",
    workspaces: "GET /api/operator/workspaces",
    workspaceDetail: "GET /api/operator/workspaces/{workspaceId}",
    reconciliation: "GET /api/operator/reconciliation",
    health: "GET /api/operator/health"
  });
  assert.equal(boundary.services.controlPlane.operatorProjection.persistence, "none_request_join_only");
  assert.deepEqual(boundary.services.controlPlane.operatorProjection.authorities, ["control_plane", "sub2api", "fabric", "ledger", "runtime"]);
  assert.equal(boundary.externalServices.gateway.currentImplementation, "paginated_users_batch_user_and_key_usage_code_complete_local_only");
  assert.equal(business.objectKinds.some((entry: { kind: string }) => entry.kind === "Announcement"), true);
});

test("Pilot V2 binds delegated Gateway credentials to process-local Console sessions", async () => {
  const [management, boundary, deployment] = await Promise.all([
    json("packages/contracts/opl-cloud-management-contract.json"),
    json("packages/contracts/opl-cloud-service-boundary-contract.json"),
    json("packages/contracts/opl-cloud-deployment-contract.json")
  ]);

  assert.deepEqual(management.identitySecurity.delegatedGatewayCredential, {
    authority: "sub2api_login_access_token",
    storage: "control_plane_process_memory_only",
    index: "hashed_session_lookup_key",
    lifetime: "bounded_by_console_session",
    missingOrExpired: "401_reauthentication_required_and_cookie_clear",
    forbidden: ["browser", "postgresql", "ledger", "logs"]
  });
  assert.equal(boundary.services.controlPlane.sessionDelegatedGatewayCredential.persistence, "process_memory_only");
  assert.equal(boundary.services.controlPlane.sessionDelegatedGatewayCredential.lookupKey, "hashed_console_session_key");
  assert.deepEqual(deployment.controlPlaneSessionCredentialVault, {
    replicas: 1,
    strategyType: "Recreate",
    persistence: "none",
    restartBehavior: "reauthentication_required",
    horizontalScaling: "blocked_pending_secure_shared_vault"
  });
});

test("Pilot V2 current human truth preserves public entry points and evidence levels", async () => {
  const [invariants, architecture, status, consoleProduct, runbook] = await Promise.all([
    text("docs/invariants.md"),
    text("docs/architecture.md"),
    text("docs/status.md"),
    text("docs/product/console-workspace-v1.md"),
    text("docs/runtime/production-runbook.md")
  ]);

  for (const document of [invariants, architecture, status, consoleProduct]) {
    assert.match(document, /OPL_GATEWAY_PUBLIC_BASE_URL/);
    assert.match(document, /code-complete/i);
    assert.match(document, /pilot-ready/i);
    assert.match(document, /production-proven/i);
  }
  assert.match(consoleProduct, /Home.*Login.*Logo/is);
  assert.match(consoleProduct, /URL.*用户名.*密码.*Workspace Key/is);
  assert.match(runbook, /OPL_POSTGRES_TESTS=1/);
  assert.match(runbook, /OPL_CAPACITY_TESTS=1/);
  assert.match(runbook, /Action=skip/);
});
