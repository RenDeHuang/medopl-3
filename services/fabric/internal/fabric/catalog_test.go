package fabric

import (
	"context"
	"strings"
	"testing"
)

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

func TestCatalogVersionIdentityAndDigestRemainImmutableAcrossSeeding(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryOperationStore()
	service := NewServiceWithOperationStore(testProvider{}, store)
	connector, err := service.Connector(ctx, "pubmed", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}

	changed := connector
	changed.Digest = "sha256:" + strings.Repeat("f", 64)
	changed.Status = "disabled"
	if err := store.SeedCatalog(ctx, []Connector{changed}, nil); err != nil {
		t.Fatal(err)
	}
	restarted := NewServiceWithOperationStore(testProvider{}, store)
	got, err := restarted.Connector(ctx, "pubmed", "1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if got.Digest != connector.Digest || got.Status != connector.Status || got.VersionIdentity != "pubmed@1.0.0" {
		t.Fatalf("immutable connector changed: before=%#v after=%#v", connector, got)
	}
}
