ALTER TABLE control_plane_workspaces
  ADD COLUMN IF NOT EXISTS purchase_receipt_id TEXT NOT NULL DEFAULT '';
