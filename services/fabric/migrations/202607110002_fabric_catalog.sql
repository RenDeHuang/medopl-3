CREATE TABLE IF NOT EXISTS fabric_connectors (id TEXT PRIMARY KEY, connector_id TEXT NOT NULL, version TEXT NOT NULL, version_identity TEXT NOT NULL UNIQUE, digest TEXT NOT NULL, name TEXT NOT NULL, status TEXT NOT NULL, read_only BOOLEAN NOT NULL DEFAULT true, provider TEXT NOT NULL, resource_metadata TEXT NOT NULL, runtime_metadata TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE (connector_id, version));
CREATE INDEX IF NOT EXISTS fabric_connectors_status_idx ON fabric_connectors(status);

CREATE TABLE IF NOT EXISTS fabric_environment_templates (id TEXT PRIMARY KEY, template_id TEXT NOT NULL, version TEXT NOT NULL, version_identity TEXT NOT NULL UNIQUE, digest TEXT NOT NULL, name TEXT NOT NULL, status TEXT NOT NULL, resource_metadata TEXT NOT NULL, runtime_metadata TEXT NOT NULL, created_at TIMESTAMPTZ NOT NULL DEFAULT now(), UNIQUE (template_id, version));
CREATE INDEX IF NOT EXISTS fabric_environment_templates_status_idx ON fabric_environment_templates(status);

CREATE OR REPLACE FUNCTION fabric_connector_identity_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.connector_id IS DISTINCT FROM OLD.connector_id OR NEW.version IS DISTINCT FROM OLD.version OR NEW.version_identity IS DISTINCT FROM OLD.version_identity OR NEW.digest IS DISTINCT FROM OLD.digest OR NEW.name IS DISTINCT FROM OLD.name OR NEW.status IS DISTINCT FROM OLD.status OR NEW.read_only IS DISTINCT FROM OLD.read_only OR NEW.provider IS DISTINCT FROM OLD.provider OR NEW.resource_metadata IS DISTINCT FROM OLD.resource_metadata OR NEW.runtime_metadata IS DISTINCT FROM OLD.runtime_metadata OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'connector version is immutable' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS fabric_connector_identity_immutable ON fabric_connectors;
CREATE TRIGGER fabric_connector_identity_immutable BEFORE UPDATE ON fabric_connectors FOR EACH ROW EXECUTE FUNCTION fabric_connector_identity_immutable();

CREATE OR REPLACE FUNCTION fabric_environment_template_identity_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF NEW.id IS DISTINCT FROM OLD.id OR NEW.template_id IS DISTINCT FROM OLD.template_id OR NEW.version IS DISTINCT FROM OLD.version OR NEW.version_identity IS DISTINCT FROM OLD.version_identity OR NEW.digest IS DISTINCT FROM OLD.digest OR NEW.name IS DISTINCT FROM OLD.name OR NEW.status IS DISTINCT FROM OLD.status OR NEW.resource_metadata IS DISTINCT FROM OLD.resource_metadata OR NEW.runtime_metadata IS DISTINCT FROM OLD.runtime_metadata OR NEW.created_at IS DISTINCT FROM OLD.created_at THEN
    RAISE EXCEPTION 'environment template version is immutable' USING ERRCODE = '23514';
  END IF;
  RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS fabric_environment_template_identity_immutable ON fabric_environment_templates;
CREATE TRIGGER fabric_environment_template_identity_immutable BEFORE UPDATE ON fabric_environment_templates FOR EACH ROW EXECUTE FUNCTION fabric_environment_template_identity_immutable();
