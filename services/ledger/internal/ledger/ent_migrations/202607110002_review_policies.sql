CREATE TABLE IF NOT EXISTS review_policies (
    id TEXT PRIMARY KEY,
    organization_id TEXT NOT NULL DEFAULT '',
    workspace_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    task_id TEXT NOT NULL,
    job_id TEXT NOT NULL,
    version TEXT NOT NULL,
    required_reviewers_json TEXT NOT NULL,
    status TEXT NOT NULL,
    supersedes_policy_id TEXT NOT NULL DEFAULT '',
    idempotency_key TEXT NOT NULL UNIQUE,
    request_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (status IN ('active', 'superseded'))
);

CREATE INDEX IF NOT EXISTS review_policies_scope_created ON review_policies (organization_id, workspace_id, project_id, task_id, job_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS review_policies_active_scope ON review_policies (organization_id, workspace_id, project_id, task_id, job_id) WHERE status = 'active';
