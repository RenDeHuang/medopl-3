package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/index"
)

type WorkspaceBackup struct{ ent.Schema }

func (WorkspaceBackup) Fields() []ent.Field { return workspaceBackupFields() }

func (WorkspaceBackup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("workspace_id", "created_at"),
		index.Fields("idempotency_key").Unique(),
	}
}
