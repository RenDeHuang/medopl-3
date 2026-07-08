package ledger

import (
	"context"
	"errors"
	"testing"
)

func TestManualTopUpReplayReturnsExistingReceipt(t *testing.T) {
	store := NewMemoryStore()
	input := ManualTopUpInput{
		AccountID:      "acct-alpha",
		AmountCents:    20000,
		Currency:       "CNY",
		OperatorUserID: "usr-admin",
		IdempotencyKey: "topup-once",
		Reason:         "operator_credit",
	}

	first, err := store.ManualTopUp(context.Background(), input)
	if err != nil {
		t.Fatalf("first topup failed: %v", err)
	}
	second, err := store.ManualTopUp(context.Background(), input)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if !second.Replayed {
		t.Fatalf("expected replayed result")
	}
	if first.LedgerEntry.ID != second.LedgerEntry.ID {
		t.Fatalf("expected same ledger entry on replay")
	}

	wallet, err := store.Wallet(context.Background(), "acct-alpha")
	if err != nil {
		t.Fatalf("wallet failed: %v", err)
	}
	if wallet.BalanceCents != 20000 {
		t.Fatalf("balance = %d, want 20000", wallet.BalanceCents)
	}
}

func TestManualTopUpSameKeyDifferentPayloadConflicts(t *testing.T) {
	store := NewMemoryStore()
	input := ManualTopUpInput{
		AccountID:      "acct-alpha",
		AmountCents:    20000,
		Currency:       "CNY",
		OperatorUserID: "usr-admin",
		IdempotencyKey: "topup-once",
		Reason:         "operator_credit",
	}
	if _, err := store.ManualTopUp(context.Background(), input); err != nil {
		t.Fatalf("topup failed: %v", err)
	}

	input.AmountCents = 30000
	_, err := store.ManualTopUp(context.Background(), input)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}

func TestReleaseHoldReducesFrozenWithoutDebitingBalance(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()
	if _, err := store.ManualTopUp(ctx, ManualTopUpInput{AccountID: "acct-alpha", AmountCents: 2000, Currency: "CNY", OperatorUserID: "usr-admin", IdempotencyKey: "topup-release"}); err != nil {
		t.Fatalf("topup failed: %v", err)
	}
	hold, err := store.CreateHold(ctx, HoldInput{AccountID: "acct-alpha", WorkspaceID: "ws-alpha", ResourceType: "compute", ResourceID: "compute-alpha", AmountCents: 1000, Currency: "CNY", IdempotencyKey: "hold-release"})
	if err != nil {
		t.Fatalf("hold failed: %v", err)
	}

	releaseInput := HoldReleaseInput{
		AccountID:      "acct-alpha",
		WorkspaceID:    "ws-alpha",
		ResourceType:   "compute",
		ResourceID:     "compute-alpha",
		HoldID:         hold.ID,
		AmountCents:    600,
		Currency:       "CNY",
		Reason:         "destroy_compute",
		IdempotencyKey: "release-once",
	}
	released, err := store.ReleaseHold(ctx, releaseInput)
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if released.Status != "released" || released.Wallet.BalanceCents != 2000 || released.Wallet.FrozenCents != 400 || released.Wallet.AvailableCents != 1600 || released.Wallet.TotalSpentCents != 0 {
		t.Fatalf("unexpected release wallet: %#v", released)
	}

	replayed, err := store.ReleaseHold(ctx, releaseInput)
	if err != nil {
		t.Fatalf("release replay failed: %v", err)
	}
	if !replayed.Replayed || replayed.ID != released.ID {
		t.Fatalf("expected same replayed release, got %#v", replayed)
	}

	releaseInput.AmountCents = 700
	_, err = store.ReleaseHold(ctx, releaseInput)
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected idempotency conflict, got %v", err)
	}
}

func TestCreateHoldRequiresResourceIdentity(t *testing.T) {
	store := NewMemoryStore()
	_, err := store.CreateHold(context.Background(), HoldInput{AccountID: "acct-alpha", WorkspaceID: "ws-alpha", AmountCents: 1000, Currency: "CNY", IdempotencyKey: "hold-missing-resource"})
	if !errors.Is(err, ErrInvalidHoldInput) {
		t.Fatalf("expected invalid hold input, got %v", err)
	}
}

func TestLedgerMutationsReturnStableAuditFacts(t *testing.T) {
	store := NewMemoryStore()
	ctx := context.Background()

	topup, err := store.ManualTopUp(ctx, ManualTopUpInput{AccountID: "acct-alpha", AmountCents: 5000, Currency: "CNY", OperatorUserID: "usr-admin", Reason: "manual_adjustment", IdempotencyKey: "audit-topup"})
	if err != nil {
		t.Fatalf("topup failed: %v", err)
	}
	if topup.TopUp.ID == "" || topup.LedgerEntry.ID == "" || topup.WalletTransaction.ID == "" || topup.WalletTransaction.LedgerEntryID != topup.LedgerEntry.ID || topup.LedgerEntry.Source != "manual_topup" {
		t.Fatalf("topup must return linked audit facts, got %#v", topup)
	}

	hold, err := store.CreateHold(ctx, HoldInput{AccountID: "acct-alpha", WorkspaceID: "ws-alpha", ResourceType: "compute", ResourceID: "compute-alpha", AmountCents: 2000, Currency: "CNY", IdempotencyKey: "audit-hold"})
	if err != nil {
		t.Fatalf("hold failed: %v", err)
	}
	if hold.ID == "" || hold.LedgerEntryID == "" || hold.WalletTransactionID == "" || hold.Status != "held" || hold.Wallet.FrozenCents != 2000 {
		t.Fatalf("hold must return linked audit facts, got %#v", hold)
	}

	release, err := store.ReleaseHold(ctx, HoldReleaseInput{AccountID: "acct-alpha", WorkspaceID: "ws-alpha", ResourceType: "compute", ResourceID: "compute-alpha", HoldID: hold.ID, AmountCents: 500, Currency: "CNY", Reason: "destroy_compute", IdempotencyKey: "audit-release"})
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}
	if release.ID == "" || release.LedgerEntryID == "" || release.WalletTransactionID == "" || release.Status != "released" || release.Wallet.BalanceCents != 5000 || release.Wallet.FrozenCents != 1500 || release.Wallet.TotalSpentCents != 0 {
		t.Fatalf("release must return linked audit facts without debiting balance, got %#v", release)
	}

	settlement, err := store.SettleResource(ctx, ResourceSettlementInput{AccountID: "acct-alpha", WorkspaceID: "ws-alpha", ResourceType: "compute", ResourceID: "compute-alpha", AmountCents: 1200, Currency: "CNY", IdempotencyKey: "audit-settlement"})
	if err != nil {
		t.Fatalf("settlement failed: %v", err)
	}
	if settlement.ID == "" || settlement.LedgerEntryID == "" || settlement.WalletTransactionID == "" || settlement.Status != "settled" || settlement.Wallet.BalanceCents != 3800 || settlement.Wallet.FrozenCents != 300 || settlement.Wallet.TotalSpentCents != 1200 {
		t.Fatalf("settlement must return linked audit facts and debit balance, got %#v", settlement)
	}

	evidence, err := store.RecordEvidence(ctx, EvidenceInput{WorkspaceID: "ws-alpha", ProviderRequestID: "provider-request-alpha", RedactedURL: "https://workspace.example.test/w/ws-alpha", TokenVersion: "v1", IdempotencyKey: "audit-evidence"})
	if err != nil {
		t.Fatalf("evidence failed: %v", err)
	}
	if evidence.ID == "" || evidence.ProviderRequestID != "provider-request-alpha" || evidence.RedactedURL == "" {
		t.Fatalf("evidence receipt must return provider provenance, got %#v", evidence)
	}

	reconciliation, err := store.RecordReconciliation(ctx, ReconciliationInput{Report: map[string]any{"id": "recon-alpha", "status": "mismatch"}, IdempotencyKey: "audit-reconciliation"})
	if err != nil {
		t.Fatalf("reconciliation failed: %v", err)
	}
	if reconciliation.ID != "recon-alpha" || reconciliation.Status != "mismatch" || !reconciliation.BlockNewWorkspaces {
		t.Fatalf("reconciliation mismatch must block new workspaces, got %#v", reconciliation)
	}

	replayed, err := store.SettleResource(ctx, ResourceSettlementInput{AccountID: "acct-alpha", WorkspaceID: "ws-alpha", ResourceType: "compute", ResourceID: "compute-alpha", AmountCents: 1200, Currency: "CNY", IdempotencyKey: "audit-settlement"})
	if err != nil {
		t.Fatalf("settlement replay failed: %v", err)
	}
	if !replayed.Replayed || replayed.ID != settlement.ID || replayed.LedgerEntryID != settlement.LedgerEntryID || replayed.WalletTransactionID != settlement.WalletTransactionID {
		t.Fatalf("settlement replay must return the same facts, got %#v", replayed)
	}

	_, err = store.SettleResource(ctx, ResourceSettlementInput{AccountID: "acct-alpha", WorkspaceID: "ws-alpha", ResourceType: "compute", ResourceID: "compute-alpha", AmountCents: 1300, Currency: "CNY", IdempotencyKey: "audit-settlement"})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("expected settlement idempotency conflict, got %v", err)
	}
}
