package fabric

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

type recordingPoolProvider struct {
	testProvider
	mu         sync.Mutex
	maxDesired int64
}

type failingPoolProvider struct {
	testProvider
	syncCalls int
}

type scaleDownFailureProvider struct{ testProvider }

func (*scaleDownFailureProvider) ReconcileComputePool(_ context.Context, input ComputePoolDemand) (ComputePoolState, error) {
	if input.DesiredReplicas == 0 {
		return ComputePoolState{}, fmt.Errorf("scale down failed")
	}
	return ComputePoolState{DesiredReplicas: input.DesiredReplicas}, nil
}

type lingeringMachineProvider struct{ testProvider }

func (*lingeringMachineProvider) ReconcileComputePool(_ context.Context, input ComputePoolDemand) (ComputePoolState, error) {
	return ComputePoolState{DesiredReplicas: input.DesiredReplicas, CurrentReplicas: 1, Machines: []ProviderMachine{{MachineID: "machine-lingering"}}}, nil
}

type failedCreateCleanupProvider struct {
	testProvider
	cleanupComplete bool
	demands         []int64
}

type transientProviderError struct {
	testProvider
	calls int
}

type evidencePoolProvider struct{ testProvider }

func (*evidencePoolProvider) ReconcileComputePool(_ context.Context, input ComputePoolDemand) (ComputePoolState, error) {
	return ComputePoolState{
		PoolID:            input.PoolID,
		NodePoolID:        "np-basic",
		DesiredReplicas:   input.DesiredReplicas,
		CurrentReplicas:   0,
		ProviderRequestID: "req-describe-machines",
	}, nil
}

func (p *transientProviderError) ReconcileComputePool(_ context.Context, input ComputePoolDemand) (ComputePoolState, error) {
	p.calls++
	if input.DesiredReplicas > 0 && p.calls == 1 {
		return ComputePoolState{}, fmt.Errorf("tencent_scale_node_pool_failed:quota")
	}
	return ComputePoolState{DesiredReplicas: input.DesiredReplicas}, nil
}

func TestPoolReconcileWindowAllowsNativeMachineProvisioning(t *testing.T) {
	if time.Duration(poolReconcileAttempts)*poolReconcileDelay < 15*time.Minute {
		t.Fatalf("pool reconcile window = %s, want at least 15m", time.Duration(poolReconcileAttempts)*poolReconcileDelay)
	}
}

func TestPoolAllocatorPersistsPoolEvidence(t *testing.T) {
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(&evidencePoolProvider{}, store)
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning"}
	operation := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "request-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), operation, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource

	_, _, _ = service.reconcileComputePoolOnce(context.Background(), "basic", false)

	operations, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var latest ComputeAllocation
	if !decodeOperationResource(operations[len(operations)-1], &latest) {
		t.Fatalf("latest operation missing resource: %#v", operations[len(operations)-1])
	}
	if latest.ProviderData["poolReconcileAttempt"] != "1" || latest.ProviderData["desiredReplicas"] != "1" || latest.ProviderData["currentReplicas"] != "0" || latest.ProviderData["describeMachinesRequestId"] != "req-describe-machines" {
		t.Fatalf("pool evidence not persisted: %#v", latest.ProviderData)
	}
}

func (p *failedCreateCleanupProvider) ReconcileComputePool(_ context.Context, input ComputePoolDemand) (ComputePoolState, error) {
	p.demands = append(p.demands, input.DesiredReplicas)
	if input.DesiredReplicas > 0 {
		return ComputePoolState{DesiredReplicas: input.DesiredReplicas}, nil
	}
	if p.cleanupComplete {
		return ComputePoolState{DesiredReplicas: 0}, nil
	}
	return ComputePoolState{DesiredReplicas: 0, CurrentReplicas: 1, Machines: []ProviderMachine{{MachineID: "machine-lingering"}}}, nil
}

type tagFailurePoolProvider struct {
	testProvider
	deleteCalls int
	deleteErr   error
}

func (*tagFailurePoolProvider) TagComputeMachine(context.Context, ProviderMachine, MachineOwnership) error {
	return fmt.Errorf("node label failed")
}

func (p *tagFailurePoolProvider) DeleteComputeMachine(context.Context, ProviderMachine) error {
	p.deleteCalls++
	return p.deleteErr
}

func TestPoolAllocatorDoesNotReleaseHoldStateWhileMachineIsQuarantined(t *testing.T) {
	provider := &tagFailurePoolProvider{deleteErr: fmt.Errorf("tencent delete failed")}
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(provider, store)
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning"}
	operation := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "request-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), operation, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource
	oldAttempts, oldDelay := poolReconcileAttempts, poolReconcileDelay
	poolReconcileAttempts, poolReconcileDelay = 1, 0
	t.Cleanup(func() { poolReconcileAttempts, poolReconcileDelay = oldAttempts, oldDelay })

	service.reconcileComputePool("basic", false)
	got, _ := service.GetComputeAllocation(context.Background(), resource.ID)
	if got.Status != "quarantined" || got.MachineName == "" || got.InstanceID == "" || got.NodePoolID == "" {
		t.Fatalf("resource with undeleted machine = %#v", got)
	}
}

func TestPoolAllocatorDeletesPartiallyTaggedMachineBeforeReleasingClaim(t *testing.T) {
	provider := &tagFailurePoolProvider{}
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(provider, store)
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning"}
	operation := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "request-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), operation, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource

	_, _, _ = service.reconcileComputePoolOnce(context.Background(), "basic", false)
	ownership, err := store.MachineOwnership(context.Background(), resource.ID)
	if err != nil || ownership.Status != "released" || provider.deleteCalls != 1 {
		t.Fatalf("partial claim cleanup ownership=%#v err=%v deletes=%d", ownership, err, provider.deleteCalls)
	}
}

func (p *failingPoolProvider) ReconcileComputePool(context.Context, ComputePoolDemand) (ComputePoolState, error) {
	return ComputePoolState{}, fmt.Errorf("tencent pool unavailable")
}

func (p *failingPoolProvider) SyncComputeAllocation(_ context.Context, allocation ComputeAllocation) (ComputeAllocation, error) {
	p.syncCalls++
	return allocation, nil
}

func TestPoolAllocatorExhaustionKeepsHoldUntilCleanupIsConfirmed(t *testing.T) {
	provider := &failingPoolProvider{}
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(provider, store)
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning", ProviderRequestID: "local-request"}
	operation := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "request-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), operation, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource
	oldAttempts, oldDelay := poolReconcileAttempts, poolReconcileDelay
	poolReconcileAttempts, poolReconcileDelay = 1, 0
	t.Cleanup(func() { poolReconcileAttempts, poolReconcileDelay = oldAttempts, oldDelay })

	service.reconcileComputePool("basic", false)
	pendingCleanup, ok := service.GetComputeAllocation(context.Background(), resource.ID)
	if !ok || pendingCleanup.Status != "provisioning" {
		t.Fatalf("cleanup-pending resource = %#v ok=%v", pendingCleanup, ok)
	}
	synced, err := service.SyncComputeAllocation(context.Background(), resource.ID)
	if err != nil || synced.Status != "provisioning" || provider.syncCalls != 1 {
		t.Fatalf("sync cleanup-pending resource = %#v err=%v provider calls=%d", synced, err, provider.syncCalls)
	}
}

func TestPendingComputeOperationStopsWhenDestroyStarts(t *testing.T) {
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(testProvider{}, store)
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning"}
	create := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "create-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), create, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	destroy := newOperation("destroy_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "destroy-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), destroy, "started", resource, nil); err != nil {
		t.Fatal(err)
	}

	pending, err := service.pendingComputeOperations(context.Background(), "basic")
	if err != nil || len(pending) != 0 {
		t.Fatalf("destroying resource remained pending: %#v err=%v", pending, err)
	}
}

func TestDestroyComputeAllocationRecordsPoolScaleDownFailure(t *testing.T) {
	service := NewServiceWithOperationStore(&scaleDownFailureProvider{}, NewMemoryOperationStore())
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning", Provider: "tencent-tke", ProviderRequestID: "compute-alpha-request"}
	service.computes[resource.ID] = resource
	oldAttempts, oldDelay := poolReconcileAttempts, poolReconcileDelay
	poolReconcileAttempts, poolReconcileDelay = 1, 0
	t.Cleanup(func() { poolReconcileAttempts, poolReconcileDelay = oldAttempts, oldDelay })

	allocation, err := service.DestroyComputeAllocation(context.Background(), resource.ID)
	if err != nil || allocation.Status != "destroying" {
		t.Fatalf("destroy response = %#v err=%v", allocation, err)
	}
	waitForOperation(t, service, "destroy_compute_allocation", "compute_allocation", resource.ID, "failed")
}

func TestDestroyFinalizesCancelingCreateAfterPoolCleanup(t *testing.T) {
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(testProvider{}, store)
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning", Provider: "tencent-tke", ProviderRequestID: "compute-alpha-request"}
	create := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "create-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), create, "canceling", resource, fmt.Errorf("compute_machine_unavailable")); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource

	if _, err := service.DestroyComputeAllocation(context.Background(), resource.ID); err != nil {
		t.Fatal(err)
	}
	waitForOperation(t, service, "destroy_compute_allocation", "compute_allocation", resource.ID, "succeeded")
	operations, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	latestCreate := FabricOperation{}
	for _, operation := range operations {
		if operation.Action == "create_compute_allocation" {
			latestCreate = operation
		}
	}
	if latestCreate.Status != "failed" || latestCreate.ErrorCode != "compute_create_canceled" {
		t.Fatalf("canceling create was not finalized: %#v", latestCreate)
	}
}

func TestPoolReconcileIsIncompleteWhileExtraMachineRemains(t *testing.T) {
	service := NewServiceWithOperationStore(&lingeringMachineProvider{}, NewMemoryOperationStore())
	complete, _, err := service.reconcileComputePoolOnce(context.Background(), "basic", false)
	if err != nil || complete {
		t.Fatalf("complete=%v err=%v, want incomplete until machine deletion", complete, err)
	}
}

func TestFailedCreateKeepsProvisioningStateUntilPoolCleanupCompletes(t *testing.T) {
	provider := &failedCreateCleanupProvider{}
	service := NewServiceWithOperationStore(provider, NewMemoryOperationStore())
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning"}
	operation := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "request-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), operation, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource
	oldAttempts, oldDelay := poolReconcileAttempts, poolReconcileDelay
	poolReconcileAttempts, poolReconcileDelay = 1, 0
	t.Cleanup(func() { poolReconcileAttempts, poolReconcileDelay = oldAttempts, oldDelay })

	_ = service.reconcileComputePool("basic", false)
	got, _ := service.GetComputeAllocation(context.Background(), resource.ID)
	if got.Status != "provisioning" {
		t.Fatalf("resource became releasable before cleanup: %#v demands=%v", got, provider.demands)
	}
}

func TestFailedCreatePublishesFailureAfterPoolCleanupCompletes(t *testing.T) {
	provider := &failedCreateCleanupProvider{cleanupComplete: true}
	service := NewServiceWithOperationStore(provider, NewMemoryOperationStore())
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning"}
	operation := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "request-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), operation, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource
	oldAttempts, oldDelay := poolReconcileAttempts, poolReconcileDelay
	poolReconcileAttempts, poolReconcileDelay = 1, 0
	t.Cleanup(func() { poolReconcileAttempts, poolReconcileDelay = oldAttempts, oldDelay })

	_ = service.reconcileComputePool("basic", false)
	got, _ := service.GetComputeAllocation(context.Background(), resource.ID)
	if got.Status != "failed" || len(provider.demands) != 2 || provider.demands[0] != 1 || provider.demands[1] != 0 {
		t.Fatalf("failure cleanup lifecycle resource=%#v demands=%v", got, provider.demands)
	}
}

func TestFailedCreatePreservesProviderErrorAcrossEmptyPolls(t *testing.T) {
	provider := &transientProviderError{}
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(provider, store)
	resource := ComputeAllocation{ID: "compute-alpha", AccountID: "acct-alpha", PackageID: "basic", Status: "provisioning"}
	operation := newOperation("create_compute_allocation", "compute_allocation", resource.ID, resource.AccountID, "", "request-alpha", hashInput(resource), time.Now().UTC())
	if err := service.recordOperation(context.Background(), operation, "started", resource, nil); err != nil {
		t.Fatal(err)
	}
	service.computes[resource.ID] = resource
	oldAttempts, oldDelay := poolReconcileAttempts, poolReconcileDelay
	poolReconcileAttempts, poolReconcileDelay = 2, 0
	t.Cleanup(func() { poolReconcileAttempts, poolReconcileDelay = oldAttempts, oldDelay })

	_ = service.reconcileComputePool("basic", false)
	operations, err := store.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	latest := operations[len(operations)-1]
	if latest.Status != "failed" || latest.ErrorCode != "tencent_scale_node_pool_failed:quota" {
		t.Fatalf("provider error was lost: %#v", latest)
	}
}

func (p *recordingPoolProvider) ReconcileComputePool(ctx context.Context, input ComputePoolDemand) (ComputePoolState, error) {
	p.mu.Lock()
	if input.DesiredReplicas > p.maxDesired {
		p.maxDesired = input.DesiredReplicas
	}
	p.mu.Unlock()
	return p.testProvider.ReconcileComputePool(ctx, input)
}

func TestPoolAllocatorAssignsDifferentMachinesToConcurrentResources(t *testing.T) {
	provider := &recordingPoolProvider{}
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(provider, store)
	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := 0; i < 100; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := service.CreateComputeAllocation(context.Background(), ComputeAllocationInput{ID: fmt.Sprintf("resource-%03d", i), AccountID: fmt.Sprintf("acct-%03d", i), PackageID: "basic", IdempotencyKey: fmt.Sprintf("request-%03d", i)})
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		ownerships, err := store.ListMachineOwnerships(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if len(ownerships) == 100 {
			seen := map[string]bool{}
			for _, ownership := range ownerships {
				if ownership.Status != "active" || seen[ownership.MachineID] {
					t.Fatalf("invalid ownership: %#v", ownership)
				}
				seen[ownership.MachineID] = true
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("ownership count = %d, want 100", len(ownerships))
		}
		time.Sleep(10 * time.Millisecond)
	}
	provider.mu.Lock()
	maxDesired := provider.maxDesired
	provider.mu.Unlock()
	if maxDesired != 100 {
		t.Fatalf("max desired replicas = %d, want 100", maxDesired)
	}
}
