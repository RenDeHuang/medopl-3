CREATE UNIQUE INDEX IF NOT EXISTS fabric_operations_runtime_claim_idx
ON fabric_operations(action, idempotency_key)
WHERE action = 'create_workspace_runtime' AND idempotency_key <> '' AND id LIKE 'fop_runtime_claim_%';
