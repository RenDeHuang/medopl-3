import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const contractUrl = new URL("../../packages/contracts/opl-cloud-fabric-resource-catalog-contract.json", import.meta.url);

test("Fabric catalog contract publishes versioned connector and environment APIs", async () => {
  const contract = JSON.parse(await readFile(contractUrl, "utf8"));
  assert.deepEqual(contract.versionedCatalog.identity, ["id", "version", "digest"]);
  assert.deepEqual(contract.versionedCatalog.statuses, ["approved", "disabled"]);
  assert.deepEqual(contract.versionedCatalog.connectors.endpoints, [
    "GET /fabric/catalog/connectors",
    "GET /fabric/catalog/connectors/{id}/versions/{version}"
  ]);
  assert.deepEqual(contract.versionedCatalog.environmentTemplates.endpoints, [
    "GET /fabric/catalog/environment-templates",
    "GET /fabric/catalog/environment-templates/{id}/versions/{version}"
  ]);
  assert.equal(contract.pubMed.endpoint, "GET /fabric/catalog/connectors/pubmed/versions/{version}/query");
  assert.equal(contract.pubMed.credentials, "none");
  assert.deepEqual(contract.seededEnvironmentTemplates.map((entry) => entry.id), [
    "python-minimal",
    "r-minimal",
    "quarto-minimal",
    "latex-minimal"
  ]);
  assert.equal(contract.seededEnvironmentTemplates.some((entry) => /cuda/i.test(JSON.stringify(entry))), false);
});
