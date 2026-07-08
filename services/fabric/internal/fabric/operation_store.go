package fabric

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"

	_ "github.com/lib/pq"
)

type OperationStore interface {
	Append(ctx context.Context, operation FabricOperation) error
	List(ctx context.Context) ([]FabricOperation, error)
}

type MemoryOperationStore struct {
	mu        sync.Mutex
	operation []FabricOperation
}

func NewMemoryOperationStore() *MemoryOperationStore {
	return &MemoryOperationStore{}
}

func (s *MemoryOperationStore) Append(_ context.Context, operation FabricOperation) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operation = append(s.operation, operation)
	return nil
}

func (s *MemoryOperationStore) List(_ context.Context) ([]FabricOperation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	operations := make([]FabricOperation, len(s.operation))
	copy(operations, s.operation)
	return operations, nil
}

type PostgresOperationStore struct {
	db *sql.DB
}

const postgresOperationSchema = `
CREATE TABLE IF NOT EXISTS fabric_operations (
  id TEXT PRIMARY KEY,
  operation_id TEXT NOT NULL,
  caller_service TEXT NOT NULL,
  action TEXT NOT NULL,
  resource_kind TEXT NOT NULL,
  resource_id TEXT NOT NULL,
  account_id TEXT,
  workspace_id TEXT,
  provider TEXT,
  provider_request_id TEXT,
  idempotency_key TEXT,
  request_hash TEXT,
  redacted_provider_payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL,
  error_code TEXT,
  retryable BOOLEAN NOT NULL DEFAULT false,
  started_at TIMESTAMPTZ NOT NULL,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX IF NOT EXISTS fabric_operations_operation_id_idx ON fabric_operations(operation_id);
CREATE INDEX IF NOT EXISTS fabric_operations_resource_idx ON fabric_operations(resource_kind, resource_id);
CREATE INDEX IF NOT EXISTS fabric_operations_workspace_idx ON fabric_operations(workspace_id);
CREATE INDEX IF NOT EXISTS fabric_operations_created_idx ON fabric_operations(created_at);
`

func PostgresOperationSchemaSQL() string {
	return postgresOperationSchema
}

func NewPostgresOperationStore(databaseURL string) (*PostgresOperationStore, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	store := &PostgresOperationStore{db: db}
	if err := store.Install(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *PostgresOperationStore) Install(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, postgresOperationSchema)
	return err
}

func (s *PostgresOperationStore) Append(ctx context.Context, operation FabricOperation) error {
	payload := operation.RedactedProviderPayload
	if payload == nil {
		payload = map[string]any{}
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	var finishedAt any
	if !operation.FinishedAt.IsZero() {
		finishedAt = operation.FinishedAt
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO fabric_operations(
  id, operation_id, caller_service, action, resource_kind, resource_id, account_id, workspace_id,
  provider, provider_request_id, idempotency_key, request_hash, redacted_provider_payload,
  status, error_code, retryable, started_at, finished_at, created_at
) VALUES (
  $1, $2, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), NULLIF($9, ''), NULLIF($10, ''),
  NULLIF($11, ''), NULLIF($12, ''), $13::jsonb, $14, NULLIF($15, ''), $16, $17, $18, $19
)`, operation.ID, operation.OperationID, operation.CallerService, operation.Action, operation.ResourceKind, operation.ResourceID, operation.AccountID, operation.WorkspaceID, operation.Provider, operation.ProviderRequestID, operation.IdempotencyKey, operation.RequestHash, string(payloadJSON), operation.Status, operation.ErrorCode, operation.Retryable, operation.StartedAt, finishedAt, operation.CreatedAt)
	return err
}

func (s *PostgresOperationStore) List(ctx context.Context) ([]FabricOperation, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, operation_id, caller_service, action, resource_kind, resource_id, COALESCE(account_id, ''),
  COALESCE(workspace_id, ''), COALESCE(provider, ''), COALESCE(provider_request_id, ''),
  COALESCE(idempotency_key, ''), COALESCE(request_hash, ''), redacted_provider_payload,
  status, COALESCE(error_code, ''), retryable, started_at, finished_at, created_at
FROM fabric_operations
ORDER BY created_at, id
`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operations []FabricOperation
	for rows.Next() {
		var operation FabricOperation
		var payload []byte
		var finishedAt sql.NullTime
		if err := rows.Scan(&operation.ID, &operation.OperationID, &operation.CallerService, &operation.Action, &operation.ResourceKind, &operation.ResourceID, &operation.AccountID, &operation.WorkspaceID, &operation.Provider, &operation.ProviderRequestID, &operation.IdempotencyKey, &operation.RequestHash, &payload, &operation.Status, &operation.ErrorCode, &operation.Retryable, &operation.StartedAt, &finishedAt, &operation.CreatedAt); err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			operation.FinishedAt = finishedAt.Time
		}
		if len(payload) > 0 {
			_ = json.Unmarshal(payload, &operation.RedactedProviderPayload)
		}
		operations = append(operations, operation)
	}
	return operations, rows.Err()
}
