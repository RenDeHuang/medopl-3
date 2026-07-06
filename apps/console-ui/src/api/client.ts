export interface WorkspaceProjection {
  id: string;
  accountId: string;
  ownerId: string;
  name: string;
  packageId: string;
  url: string;
  status: string;
  holdId: string;
  computeId: string;
  volumeId: string;
  runtimeId: string;
  evidenceId: string;
}

export interface CreateWorkspaceInput {
  accountId: string;
  ownerId: string;
  name: string;
  packageId: string;
}

async function requestJson<T>(path: string, init: RequestInit = {}): Promise<T> {
  const response = await fetch(path, init);
  const payload = await response.json();
  if (!response.ok) {
    throw new Error(payload?.error || "request_failed");
  }
  return payload as T;
}

export function getOverview(): Promise<{ service: string; workspaces: number }> {
  return requestJson("/api/overview");
}

export function createWorkspace(input: CreateWorkspaceInput, idempotencyKey: string): Promise<WorkspaceProjection> {
  return requestJson("/api/workspaces", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Idempotency-Key": idempotencyKey
    },
    body: JSON.stringify(input)
  });
}
