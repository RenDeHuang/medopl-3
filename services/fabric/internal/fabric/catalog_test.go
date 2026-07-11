package fabric

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

var errCatalogSeed = errors.New("catalog_seed_failed")

type failingCatalogSeedStore struct{ *MemoryOperationStore }

func (s *failingCatalogSeedStore) SeedCatalog(context.Context, []Connector, []EnvironmentTemplate) error {
	return errCatalogSeed
}

func TestCatalogSeedFailureMakesServiceUnready(t *testing.T) {
	service := NewServiceWithOperationStore(testProvider{}, &failingCatalogSeedStore{NewMemoryOperationStore()})
	if _, err := service.Readiness(context.Background()); !errors.Is(err, errCatalogSeed) {
		t.Fatalf("readiness error = %v, want catalog seed failure", err)
	}
}

func TestCatalogSeedsVersionedPubMedAndMinimalCPUEnvironments(t *testing.T) {
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(testProvider{}, store)

	connectors, err := service.ListConnectors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(connectors) != 1 || connectors[0].ID != "pubmed" || connectors[0].Version == "" || connectors[0].Digest == "" || connectors[0].Status != "approved" || !connectors[0].ReadOnly {
		t.Fatalf("connectors = %#v", connectors)
	}
	if !strings.HasPrefix(connectors[0].Digest, "sha256:") {
		t.Fatalf("connector digest = %q", connectors[0].Digest)
	}

	templates, err := service.ListEnvironmentTemplates(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(templates) != 4 {
		t.Fatalf("templates = %#v", templates)
	}
	want := map[string]bool{"python-minimal": false, "r-minimal": false, "quarto-minimal": false, "latex-minimal": false}
	for _, template := range templates {
		if template.Version == "" || template.Digest == "" || template.Status != "approved" || template.Runtime.Version == "" {
			t.Fatalf("template lacks immutable/runtime metadata: %#v", template)
		}
		if strings.Contains(strings.ToLower(template.Runtime.Name+" "+template.Runtime.Image), "cuda") || template.Resources.GPU != 0 {
			t.Fatalf("CUDA/GPU template leaked into minimal catalog: %#v", template)
		}
		if _, ok := want[template.ID]; !ok {
			t.Fatalf("unexpected template %q", template.ID)
		}
		want[template.ID] = true
	}
	for id, found := range want {
		if !found {
			t.Fatalf("missing template %q", id)
		}
	}
}

func TestCatalogVersionsRejectAllImmutableContentChanges(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(testProvider{}, store)
	originalConnector, err := service.Connector(ctx, "pubmed", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	connectorChanges := map[string]func(*Connector){
		"identity":  func(row *Connector) { row.VersionIdentity = "changed@1.0.0" },
		"digest":    func(row *Connector) { row.Digest = "sha256:" + strings.Repeat("f", 64) },
		"name":      func(row *Connector) { row.Name = "Changed" },
		"status":    func(row *Connector) { row.Status = "disabled" },
		"readOnly":  func(row *Connector) { row.ReadOnly = !row.ReadOnly },
		"provider":  func(row *Connector) { row.Provider = "changed" },
		"resources": func(row *Connector) { row.Resources.MaxPageSize++ },
		"runtime":   func(row *Connector) { row.Runtime.Protocol = "changed" },
		"createdAt": func(row *Connector) { row.CreatedAt = row.CreatedAt.Add(time.Second) },
	}
	for name, change := range connectorChanges {
		changed := originalConnector
		change(&changed)
		if err := store.SeedCatalog(ctx, []Connector{changed}, nil); !errors.Is(err, ErrCatalogVersionConflict) {
			t.Fatalf("connector %s change error = %v", name, err)
		}
	}

	originalTemplate, err := service.EnvironmentTemplate(ctx, "python-minimal", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	templateChanges := map[string]func(*EnvironmentTemplate){
		"identity":  func(row *EnvironmentTemplate) { row.VersionIdentity = "changed@1.0.0" },
		"digest":    func(row *EnvironmentTemplate) { row.Digest = "sha256:" + strings.Repeat("f", 64) },
		"name":      func(row *EnvironmentTemplate) { row.Name = "Changed" },
		"status":    func(row *EnvironmentTemplate) { row.Status = "disabled" },
		"resources": func(row *EnvironmentTemplate) { row.Resources.MemoryMB++ },
		"runtime":   func(row *EnvironmentTemplate) { row.Runtime.Image = "changed" },
		"createdAt": func(row *EnvironmentTemplate) { row.CreatedAt = row.CreatedAt.Add(time.Second) },
	}
	for name, change := range templateChanges {
		changed := originalTemplate
		change(&changed)
		if err := store.SeedCatalog(ctx, nil, []EnvironmentTemplate{changed}); !errors.Is(err, ErrCatalogVersionConflict) {
			t.Fatalf("template %s change error = %v", name, err)
		}
	}
}
