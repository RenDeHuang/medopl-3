CREATE TABLE IF NOT EXISTS machine_ownerships (
    id TEXT PRIMARY KEY,
    resource_id TEXT NOT NULL UNIQUE,
    account_id TEXT NOT NULL,
    workspace_id TEXT NOT NULL DEFAULT '',
    package_id TEXT NOT NULL,
    node_pool_id TEXT NOT NULL,
    machine_id TEXT NOT NULL UNIQUE,
    instance_id TEXT UNIQUE,
    node_name TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    provider_request_id TEXT NOT NULL DEFAULT '',
    claimed_at TIMESTAMPTZ NOT NULL,
    released_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS machine_ownerships_pool_status_idx ON machine_ownerships(node_pool_id, status);
