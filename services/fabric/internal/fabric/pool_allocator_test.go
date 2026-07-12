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
