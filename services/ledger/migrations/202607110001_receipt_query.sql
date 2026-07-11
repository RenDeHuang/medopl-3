ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS receipt_type TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS payload_json TEXT NOT NULL DEFAULT '{}';
ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS supersedes_receipt_id TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS organization_id TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS project_id TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS task_id TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_receipts ADD COLUMN IF NOT EXISTS job_id TEXT NOT NULL DEFAULT '';
ALTER TABLE evidence_receipts ALTER COLUMN provider_request_id SET DEFAULT '';
ALTER TABLE evidence_receipts ALTER COLUMN redacted_url SET DEFAULT '';
ALTER TABLE evidence_receipts ALTER COLUMN token_version SET DEFAULT '';

UPDATE evidence_receipts
SET organization_id = COALESCE(payload_json::jsonb ->> 'organizationId', ''),
    project_id = COALESCE(payload_json::jsonb ->> 'projectId', ''),
    task_id = COALESCE(payload_json::jsonb ->> 'taskId', ''),
    job_id = COALESCE(payload_json::jsonb ->> 'jobId', '')
WHERE organization_id = '' OR project_id = '' OR task_id = '' OR job_id = '';

CREATE INDEX IF NOT EXISTS evidence_receipts_organization_created ON evidence_receipts (organization_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS evidence_receipts_workspace_created ON evidence_receipts (workspace_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS evidence_receipts_project_created ON evidence_receipts (project_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS evidence_receipts_task_created ON evidence_receipts (task_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS evidence_receipts_job_created ON evidence_receipts (job_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS evidence_receipts_type_created ON evidence_receipts (receipt_type, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS evidence_receipts_status_created ON evidence_receipts (status, created_at DESC, id DESC);
