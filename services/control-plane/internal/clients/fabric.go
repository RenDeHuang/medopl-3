package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
)

type FabricClient interface {
	CreateComputeAllocation(ctx context.Context, input ComputeAllocationInput, idempotencyKey string) (ComputeAllocation, error)
	CreateStorageVolume(ctx context.Context, input StorageVolumeInput, idempotencyKey string) (StorageVolume, error)
	CreateWorkspaceRuntime(ctx context.Context, input WorkspaceRuntimeInput, idempotencyKey string) (WorkspaceRuntime, error)
}

type ComputeAllocationInput struct {
	AccountID   string `json:"accountId"`
	WorkspaceID string `json:"workspaceId"`
	PackageID   string `json:"packageId"`
}

type ComputeAllocation struct {
	ID                string `json:"id"`
	ProviderRequestID string `json:"providerRequestId"`
}

type StorageVolumeInput struct {
	AccountID   string `json:"accountId"`
	WorkspaceID string `json:"workspaceId"`
	SizeGB      int    `json:"sizeGb"`
}

type StorageVolume struct {
	ID                string `json:"id"`
	ProviderRequestID string `json:"providerRequestId"`
}

type WorkspaceRuntimeInput struct {
	WorkspaceID string `json:"workspaceId"`
	ComputeID   string `json:"computeId"`
	VolumeID    string `json:"volumeId"`
	ImageID     string `json:"imageId"`
}

type WorkspaceRuntime struct {
	ID          string `json:"id"`
	WorkspaceID string `json:"workspaceId"`
	URL         string `json:"url"`
}

type fabricHTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewFabricHTTPClient(baseURL string, client *http.Client) FabricClient {
	if client == nil {
		client = http.DefaultClient
	}
	return &fabricHTTPClient{baseURL: baseURL, client: client}
}

func (c *fabricHTTPClient) CreateComputeAllocation(ctx context.Context, input ComputeAllocationInput, idempotencyKey string) (ComputeAllocation, error) {
	var result ComputeAllocation
	err := c.post(ctx, "/fabric/compute-allocations", input, idempotencyKey, &result)
	return result, err
}

func (c *fabricHTTPClient) CreateStorageVolume(ctx context.Context, input StorageVolumeInput, idempotencyKey string) (StorageVolume, error) {
	var result StorageVolume
	err := c.post(ctx, "/fabric/storage-volumes", input, idempotencyKey, &result)
	return result, err
}

func (c *fabricHTTPClient) CreateWorkspaceRuntime(ctx context.Context, input WorkspaceRuntimeInput, idempotencyKey string) (WorkspaceRuntime, error) {
	var result WorkspaceRuntime
	err := c.post(ctx, "/fabric/workspace-runtimes", input, idempotencyKey, &result)
	return result, err
}

func (c *fabricHTTPClient) post(ctx context.Context, path string, input any, idempotencyKey string, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", idempotencyKey)
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	return json.NewDecoder(res.Body).Decode(output)
}
