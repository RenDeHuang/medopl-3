package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type ReviewPolicy struct{ ent.Schema }

func (ReviewPolicy) Fields() []ent.Field {
	return []ent.Field{
		idField(),
		field.String("organization_id").Default(""),
		field.String("workspace_id").NotEmpty(),
		field.String("project_id").NotEmpty(),
		field.String("task_id").NotEmpty(),
		field.String("job_id").NotEmpty(),
		field.String("version").NotEmpty(),
		field.String("required_reviewers_json").NotEmpty(),
		field.String("status").NotEmpty(),
		field.String("supersedes_policy_id").Default(""),
		field.String("idempotency_key").NotEmpty().Unique(),
		field.String("request_hash").NotEmpty(),
		createdAtField(),
	}
}

func (ReviewPolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("organization_id", "workspace_id", "project_id", "task_id", "job_id", "created_at"),
		index.Fields("status"),
	}
}
