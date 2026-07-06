package controlplane

import (
	"context"
	"fmt"
	"time"

	"opl-cloud/services/control-plane/internal/clients"
	"opl-cloud/services/control-plane/internal/domain"
)

type Service struct {
	ledger clients.LedgerClient
	fabric clients.FabricClient
}

type CreateWorkspaceInput struct {
	AccountID string `json:"accountId"`
	OwnerID   string `json:"ownerId"`
	Name      string `json:"name"`
	PackageID string `json:"packageId"`
}

func NewService(ledger clients.LedgerClient, fabric clients.FabricClient) *Service {
	return &Service{ledger: ledger, fabric: fabric}
}

func (s *Service) CreateWorkspace(ctx context.Context, input CreateWorkspaceInput, idempotencyKey string) (domain.WorkspaceProjection, error) {
	workspaceID := fmt.Sprintf("ws_%d", time.Now().UTC().UnixNano())
	hold, err := s.ledger.CreateHold(ctx, clients.HoldInput{AccountID: input.AccountID, WorkspaceID: workspaceID, AmountCents: 1000, Currency: "CNY"}, idempotencyKey+":hold")
	if err != nil {
		return domain.WorkspaceProjection{}, err
	}
	compute, err := s.fabric.CreateComputeAllocation(ctx, clients.ComputeAllocationInput{AccountID: input.AccountID, WorkspaceID: workspaceID, PackageID: input.PackageID}, idempotencyKey+":compute")
	if err != nil {
		return domain.WorkspaceProjection{}, err
	}
	volume, err := s.fabric.CreateStorageVolume(ctx, clients.StorageVolumeInput{AccountID: input.AccountID, WorkspaceID: workspaceID, SizeGB: 10}, idempotencyKey+":storage")
	if err != nil {
		return domain.WorkspaceProjection{}, err
	}
	runtime, err := s.fabric.CreateWorkspaceRuntime(ctx, clients.WorkspaceRuntimeInput{WorkspaceID: workspaceID, ComputeID: compute.ID, VolumeID: volume.ID, ImageID: "one-person-lab-app"}, idempotencyKey+":runtime")
	if err != nil {
		return domain.WorkspaceProjection{}, err
	}
	evidence, err := s.ledger.RecordEvidence(ctx, clients.EvidenceInput{WorkspaceID: workspaceID, ProviderRequestID: runtime.ID, RedactedURL: runtime.URL, TokenVersion: "v1"}, idempotencyKey+":evidence")
	if err != nil {
		return domain.WorkspaceProjection{}, err
	}

	return domain.WorkspaceProjection{
		ID:         workspaceID,
		AccountID:  input.AccountID,
		OwnerID:    input.OwnerID,
		Name:       input.Name,
		PackageID:  input.PackageID,
		URL:        runtime.URL,
		Status:     "running",
		HoldID:     hold.ID,
		ComputeID:  compute.ID,
		VolumeID:   volume.ID,
		RuntimeID:  runtime.ID,
		EvidenceID: evidence.ID,
	}, nil
}
