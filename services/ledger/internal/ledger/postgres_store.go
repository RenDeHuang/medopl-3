package ledger

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const postgresSchema = `
CREATE TABLE IF NOT EXISTS wallets (
  account_id TEXT PRIMARY KEY,
  balance_cents BIGINT NOT NULL DEFAULT 0,
  currency TEXT NOT NULL DEFAULT 'CNY',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ledger_entries (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES wallets(account_id),
  amount_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  direction TEXT NOT NULL,
  source TEXT NOT NULL,
  operator_user_id TEXT NOT NULL,
  reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS wallet_transactions (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES wallets(account_id),
  ledger_entry_id TEXT NOT NULL REFERENCES ledger_entries(id),
  amount_cents BIGINT NOT NULL,
  balance_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS manual_topups (
  id TEXT PRIMARY KEY,
  account_id TEXT NOT NULL REFERENCES wallets(account_id),
  amount_cents BIGINT NOT NULL,
  currency TEXT NOT NULL,
  operator_user_id TEXT NOT NULL,
  ledger_entry_id TEXT NOT NULL REFERENCES ledger_entries(id),
  wallet_transaction_id TEXT NOT NULL REFERENCES wallet_transactions(id),
  idempotency_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  reason TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  service TEXT NOT NULL,
  idempotency_key TEXT NOT NULL,
  request_hash TEXT NOT NULL,
  response_ref TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (service, idempotency_key)
);

CREATE UNIQUE INDEX IF NOT EXISTS manual_topups_idempotency_key_idx
  ON manual_topups(idempotency_key);
`

type PostgresStore struct {
	db  *sql.DB
	now func() time.Time
}

func PostgresSchemaSQL() string {
	return postgresSchema
}

func NewPostgresStore(db *sql.DB) *PostgresStore {
	return &PostgresStore{db: db, now: func() time.Time { return time.Now().UTC() }}
}

func (s *PostgresStore) Install(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, PostgresSchemaSQL())
	return err
}

func (s *PostgresStore) ManualTopUp(ctx context.Context, input ManualTopUpInput) (ManualTopUpResult, error) {
	requestHash, err := hashManualTopUp(input)
	if err != nil {
		return ManualTopUpResult{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ManualTopUpResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	existing, existingHash, err := s.manualTopUpByIdempotencyKey(ctx, tx, input.IdempotencyKey)
	if err == nil {
		if existingHash != requestHash {
			return ManualTopUpResult{}, ErrIdempotencyConflict
		}
		existing.Replayed = true
		return existing, tx.Commit()
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return ManualTopUpResult{}, err
	}

	now := s.now()
	if _, err := tx.ExecContext(ctx, `
INSERT INTO idempotency_keys(service, idempotency_key, request_hash)
VALUES ('ledger.manual_topups', $1, $2)
`, input.IdempotencyKey, requestHash); err != nil {
		return ManualTopUpResult{}, err
	}

	wallet := Wallet{}
	if err := tx.QueryRowContext(ctx, `
INSERT INTO wallets(account_id, balance_cents, currency, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id) DO UPDATE SET
  balance_cents = wallets.balance_cents + EXCLUDED.balance_cents,
  currency = EXCLUDED.currency,
  updated_at = EXCLUDED.updated_at
RETURNING account_id, balance_cents, currency, updated_at
`, input.AccountID, input.AmountCents, input.Currency, now).Scan(
		&wallet.AccountID,
		&wallet.BalanceCents,
		&wallet.Currency,
		&wallet.UpdatedAt,
	); err != nil {
		return ManualTopUpResult{}, err
	}

	entry := LedgerEntry{
		ID:             postgresID("le", now),
		AccountID:      input.AccountID,
		AmountCents:    input.AmountCents,
		Currency:       input.Currency,
		Direction:      "credit",
		Source:         "manual_topup",
		OperatorUserID: input.OperatorUserID,
		Reason:         input.Reason,
		CreatedAt:      now,
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO ledger_entries(id, account_id, amount_cents, currency, direction, source, operator_user_id, reason, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
`, entry.ID, entry.AccountID, entry.AmountCents, entry.Currency, entry.Direction, entry.Source, entry.OperatorUserID, entry.Reason, entry.CreatedAt); err != nil {
		return ManualTopUpResult{}, err
	}

	walletTx := WalletTransaction{
		ID:            postgresID("wtx", now.Add(time.Nanosecond)),
		AccountID:     input.AccountID,
		LedgerEntryID: entry.ID,
		AmountCents:   input.AmountCents,
		BalanceCents:  wallet.BalanceCents,
		Currency:      input.Currency,
		CreatedAt:     now,
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO wallet_transactions(id, account_id, ledger_entry_id, amount_cents, balance_cents, currency, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
`, walletTx.ID, walletTx.AccountID, walletTx.LedgerEntryID, walletTx.AmountCents, walletTx.BalanceCents, walletTx.Currency, walletTx.CreatedAt); err != nil {
		return ManualTopUpResult{}, err
	}

	topup := ManualTopUp{
		ID:             postgresID("mtu", now.Add(2*time.Nanosecond)),
		AccountID:      input.AccountID,
		AmountCents:    input.AmountCents,
		Currency:       input.Currency,
		OperatorUserID: input.OperatorUserID,
		LedgerEntryID:  entry.ID,
		Reason:         input.Reason,
		CreatedAt:      now,
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO manual_topups(id, account_id, amount_cents, currency, operator_user_id, ledger_entry_id, wallet_transaction_id, idempotency_key, request_hash, reason, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
`, topup.ID, topup.AccountID, topup.AmountCents, topup.Currency, topup.OperatorUserID, topup.LedgerEntryID, walletTx.ID, input.IdempotencyKey, requestHash, topup.Reason, topup.CreatedAt); err != nil {
		return ManualTopUpResult{}, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE idempotency_keys
SET response_ref = $1
WHERE service = 'ledger.manual_topups' AND idempotency_key = $2
`, topup.ID, input.IdempotencyKey); err != nil {
		return ManualTopUpResult{}, err
	}

	result := ManualTopUpResult{TopUp: topup, LedgerEntry: entry, WalletTransaction: walletTx, Wallet: wallet}
	return result, tx.Commit()
}

func (s *PostgresStore) Wallet(ctx context.Context, accountID string) (Wallet, error) {
	wallet := Wallet{}
	err := s.db.QueryRowContext(ctx, `
SELECT account_id, balance_cents, currency, updated_at
FROM wallets
WHERE account_id = $1
`, accountID).Scan(&wallet.AccountID, &wallet.BalanceCents, &wallet.Currency, &wallet.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Wallet{AccountID: accountID, Currency: "CNY"}, nil
	}
	return wallet, err
}

func (s *PostgresStore) manualTopUpByIdempotencyKey(ctx context.Context, tx *sql.Tx, key string) (ManualTopUpResult, string, error) {
	result := ManualTopUpResult{}
	var requestHash string
	err := tx.QueryRowContext(ctx, `
SELECT
  mt.id, mt.account_id, mt.amount_cents, mt.currency, mt.operator_user_id, mt.ledger_entry_id, mt.reason, mt.created_at, mt.request_hash,
  le.id, le.account_id, le.amount_cents, le.currency, le.direction, le.source, le.operator_user_id, le.reason, le.created_at,
  wt.id, wt.account_id, wt.ledger_entry_id, wt.amount_cents, wt.balance_cents, wt.currency, wt.created_at,
  w.account_id, w.balance_cents, w.currency, w.updated_at
FROM manual_topups mt
JOIN ledger_entries le ON le.id = mt.ledger_entry_id
JOIN wallet_transactions wt ON wt.id = mt.wallet_transaction_id
JOIN wallets w ON w.account_id = mt.account_id
WHERE mt.idempotency_key = $1
`, key).Scan(
		&result.TopUp.ID,
		&result.TopUp.AccountID,
		&result.TopUp.AmountCents,
		&result.TopUp.Currency,
		&result.TopUp.OperatorUserID,
		&result.TopUp.LedgerEntryID,
		&result.TopUp.Reason,
		&result.TopUp.CreatedAt,
		&requestHash,
		&result.LedgerEntry.ID,
		&result.LedgerEntry.AccountID,
		&result.LedgerEntry.AmountCents,
		&result.LedgerEntry.Currency,
		&result.LedgerEntry.Direction,
		&result.LedgerEntry.Source,
		&result.LedgerEntry.OperatorUserID,
		&result.LedgerEntry.Reason,
		&result.LedgerEntry.CreatedAt,
		&result.WalletTransaction.ID,
		&result.WalletTransaction.AccountID,
		&result.WalletTransaction.LedgerEntryID,
		&result.WalletTransaction.AmountCents,
		&result.WalletTransaction.BalanceCents,
		&result.WalletTransaction.Currency,
		&result.WalletTransaction.CreatedAt,
		&result.Wallet.AccountID,
		&result.Wallet.BalanceCents,
		&result.Wallet.Currency,
		&result.Wallet.UpdatedAt,
	)
	return result, requestHash, err
}

func postgresID(prefix string, t time.Time) string {
	return fmt.Sprintf("%s_%d", prefix, t.UnixNano())
}
