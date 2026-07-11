package fabric

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	fabricent "opl-cloud/services/fabric/ent"
	"opl-cloud/services/fabric/ent/connector"
	"opl-cloud/services/fabric/ent/environmenttemplate"
)

type CatalogStore interface {
	SeedCatalog(ctx context.Context, connectors []Connector, templates []EnvironmentTemplate) error
	ListConnectors(ctx context.Context) ([]Connector, error)
	Connector(ctx context.Context, id, version string) (Connector, error)
	ListEnvironmentTemplates(ctx context.Context) ([]EnvironmentTemplate, error)
	EnvironmentTemplate(ctx context.Context, id, version string) (EnvironmentTemplate, error)
}

func defaultCatalogRecords() ([]Connector, []EnvironmentTemplate) {
	createdAt := time.Date(2026, 7, 11, 0, 0, 0, 0, time.UTC)
	pubmed := Connector{
		ID: "pubmed", Version: "1.0.0", VersionIdentity: "pubmed@1.0.0", Name: "PubMed", Status: "approved", ReadOnly: true, Provider: "ncbi",
		Resources: ConnectorResourceMetadata{MaxQueryLength: 500, MaxPageSize: 100},
		Runtime:   ConnectorRuntimeMetadata{Protocol: "ncbi-eutils", BaseURL: "https://eutils.ncbi.nlm.nih.gov/entrez/eutils", TimeoutSeconds: 10}, CreatedAt: createdAt,
	}
	pubmed.Digest = catalogRecordDigest(pubmed.VersionIdentity, pubmed.Name, pubmed.Status, pubmed.Resources, pubmed.Runtime)
	templates := []EnvironmentTemplate{
		minimalEnvironment("python-minimal", "Python Minimal", "python", "3.12.4", "python:3.12.4-slim", createdAt),
		minimalEnvironment("r-minimal", "R Minimal", "r", "4.4.1", "r-base:4.4.1", createdAt),
		minimalEnvironment("quarto-minimal", "Quarto Minimal", "quarto", "1.5.57", "ghcr.io/quarto-dev/quarto:1.5.57", createdAt),
		minimalEnvironment("latex-minimal", "LaTeX Minimal", "latex", "texlive-2024", "texlive/texlive:TL2024-historic", createdAt),
	}
	return []Connector{pubmed}, templates
}

func minimalEnvironment(id, name, runtimeName, runtimeVersion, image string, createdAt time.Time) EnvironmentTemplate {
	template := EnvironmentTemplate{
		ID: id, Version: "1.0.0", VersionIdentity: id + "@1.0.0", Name: name, Status: "approved",
		Resources: EnvironmentResourceMetadata{CPU: 1, MemoryMB: 1024, GPU: 0},
		Runtime:   EnvironmentRuntimeMetadata{Name: runtimeName, Version: runtimeVersion, Image: image}, CreatedAt: createdAt,
	}
	template.Digest = catalogRecordDigest(template.VersionIdentity, template.Name, template.Status, template.Resources, template.Runtime)
	return template
}

func catalogRecordDigest(parts ...any) string {
	data, _ := json.Marshal(parts)
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}

func catalogKey(id, version string) string { return id + "@" + version }

func (s *MemoryOperationStore) SeedCatalog(_ context.Context, connectors []Connector, templates []EnvironmentTemplate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, record := range connectors {
		key := catalogKey(record.ID, record.Version)
		if _, exists := s.connectors[key]; !exists {
			s.connectors[key] = record
		}
	}
	for _, record := range templates {
		key := catalogKey(record.ID, record.Version)
		if _, exists := s.environmentTemplates[key]; !exists {
			s.environmentTemplates[key] = record
		}
	}
	return nil
}

func (s *MemoryOperationStore) ListConnectors(_ context.Context) ([]Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]Connector, 0, len(s.connectors))
	for _, row := range s.connectors {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].VersionIdentity < rows[j].VersionIdentity })
	return rows, nil
}

func (s *MemoryOperationStore) Connector(_ context.Context, id, version string) (Connector, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.connectors[catalogKey(id, version)]
	if !ok {
		return Connector{}, ErrCatalogRecordNotFound
	}
	return row, nil
}

func (s *MemoryOperationStore) ListEnvironmentTemplates(_ context.Context) ([]EnvironmentTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows := make([]EnvironmentTemplate, 0, len(s.environmentTemplates))
	for _, row := range s.environmentTemplates {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].VersionIdentity < rows[j].VersionIdentity })
	return rows, nil
}

func (s *MemoryOperationStore) EnvironmentTemplate(_ context.Context, id, version string) (EnvironmentTemplate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row, ok := s.environmentTemplates[catalogKey(id, version)]
	if !ok {
		return EnvironmentTemplate{}, ErrCatalogRecordNotFound
	}
	return row, nil
}

func (s *PostgresOperationStore) SeedCatalog(ctx context.Context, connectors []Connector, templates []EnvironmentTemplate) error {
	for _, record := range connectors {
		_, err := s.client.Connector.Query().Where(connector.ConnectorID(record.ID), connector.Version(record.Version)).Only(ctx)
		if err == nil {
			continue
		}
		if !fabricent.IsNotFound(err) {
			return err
		}
		resources, _ := json.Marshal(record.Resources)
		runtime, _ := json.Marshal(record.Runtime)
		if err := s.client.Connector.Create().SetID(record.VersionIdentity).SetConnectorID(record.ID).SetVersion(record.Version).SetVersionIdentity(record.VersionIdentity).SetDigest(record.Digest).SetName(record.Name).SetStatus(record.Status).SetReadOnly(record.ReadOnly).SetProvider(record.Provider).SetResourceMetadata(string(resources)).SetRuntimeMetadata(string(runtime)).SetCreatedAt(record.CreatedAt).Exec(ctx); err != nil {
			if _, lookupErr := s.client.Connector.Query().Where(connector.ConnectorID(record.ID), connector.Version(record.Version)).Only(ctx); lookupErr != nil {
				return err
			}
		}
	}
	for _, record := range templates {
		_, err := s.client.EnvironmentTemplate.Query().Where(environmenttemplate.TemplateID(record.ID), environmenttemplate.Version(record.Version)).Only(ctx)
		if err == nil {
			continue
		}
		if !fabricent.IsNotFound(err) {
			return err
		}
		resources, _ := json.Marshal(record.Resources)
		runtime, _ := json.Marshal(record.Runtime)
		if err := s.client.EnvironmentTemplate.Create().SetID(record.VersionIdentity).SetTemplateID(record.ID).SetVersion(record.Version).SetVersionIdentity(record.VersionIdentity).SetDigest(record.Digest).SetName(record.Name).SetStatus(record.Status).SetResourceMetadata(string(resources)).SetRuntimeMetadata(string(runtime)).SetCreatedAt(record.CreatedAt).Exec(ctx); err != nil {
			if _, lookupErr := s.client.EnvironmentTemplate.Query().Where(environmenttemplate.TemplateID(record.ID), environmenttemplate.Version(record.Version)).Only(ctx); lookupErr != nil {
				return err
			}
		}
	}
	return nil
}

func (s *PostgresOperationStore) ListConnectors(ctx context.Context) ([]Connector, error) {
	rows, err := s.client.Connector.Query().Order(fabricent.Asc(connector.FieldVersionIdentity)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]Connector, 0, len(rows))
	for _, row := range rows {
		result = append(result, connectorFromEnt(row))
	}
	return result, nil
}

func (s *PostgresOperationStore) Connector(ctx context.Context, id, version string) (Connector, error) {
	row, err := s.client.Connector.Query().Where(connector.ConnectorID(id), connector.Version(version)).Only(ctx)
	if fabricent.IsNotFound(err) {
		return Connector{}, ErrCatalogRecordNotFound
	}
	if err != nil {
		return Connector{}, err
	}
	return connectorFromEnt(row), nil
}

func connectorFromEnt(row *fabricent.Connector) Connector {
	result := Connector{ID: row.ConnectorID, Version: row.Version, VersionIdentity: row.VersionIdentity, Digest: row.Digest, Name: row.Name, Status: row.Status, ReadOnly: row.ReadOnly, Provider: row.Provider, CreatedAt: row.CreatedAt}
	_ = json.Unmarshal([]byte(row.ResourceMetadata), &result.Resources)
	_ = json.Unmarshal([]byte(row.RuntimeMetadata), &result.Runtime)
	return result
}

func (s *PostgresOperationStore) ListEnvironmentTemplates(ctx context.Context) ([]EnvironmentTemplate, error) {
	rows, err := s.client.EnvironmentTemplate.Query().Order(fabricent.Asc(environmenttemplate.FieldVersionIdentity)).All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]EnvironmentTemplate, 0, len(rows))
	for _, row := range rows {
		result = append(result, environmentTemplateFromEnt(row))
	}
	return result, nil
}

func (s *PostgresOperationStore) EnvironmentTemplate(ctx context.Context, id, version string) (EnvironmentTemplate, error) {
	row, err := s.client.EnvironmentTemplate.Query().Where(environmenttemplate.TemplateID(id), environmenttemplate.Version(version)).Only(ctx)
	if fabricent.IsNotFound(err) {
		return EnvironmentTemplate{}, ErrCatalogRecordNotFound
	}
	if err != nil {
		return EnvironmentTemplate{}, err
	}
	return environmentTemplateFromEnt(row), nil
}

func environmentTemplateFromEnt(row *fabricent.EnvironmentTemplate) EnvironmentTemplate {
	result := EnvironmentTemplate{ID: row.TemplateID, Version: row.Version, VersionIdentity: row.VersionIdentity, Digest: row.Digest, Name: row.Name, Status: row.Status, CreatedAt: row.CreatedAt}
	_ = json.Unmarshal([]byte(row.ResourceMetadata), &result.Resources)
	_ = json.Unmarshal([]byte(row.RuntimeMetadata), &result.Runtime)
	return result
}

func (s *Service) ListConnectors(ctx context.Context) ([]Connector, error) {
	if s.catalogInitErr != nil {
		return nil, s.catalogInitErr
	}
	return s.catalog.ListConnectors(ctx)
}

func (s *Service) Connector(ctx context.Context, id, version string) (Connector, error) {
	if s.catalogInitErr != nil {
		return Connector{}, s.catalogInitErr
	}
	return s.catalog.Connector(ctx, id, version)
}

func (s *Service) ListEnvironmentTemplates(ctx context.Context) ([]EnvironmentTemplate, error) {
	if s.catalogInitErr != nil {
		return nil, s.catalogInitErr
	}
	return s.catalog.ListEnvironmentTemplates(ctx)
}

func (s *Service) EnvironmentTemplate(ctx context.Context, id, version string) (EnvironmentTemplate, error) {
	if s.catalogInitErr != nil {
		return EnvironmentTemplate{}, s.catalogInitErr
	}
	return s.catalog.EnvironmentTemplate(ctx, id, version)
}
