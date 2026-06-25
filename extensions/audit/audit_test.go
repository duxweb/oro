package audit_test

import (
	"context"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/audit"
	"github.com/duxweb/oro/extensions/softdelete"
	_ "modernc.org/sqlite"
)

type article struct {
	oro.Model
	softdelete.SoftDeleteFields
	audit.AuditFields
	Title string
}

func (article) Define(s *oro.SchemaBuilder) {
	s.Table("articles")
	s.Field("Title").String()
}

func TestAuditExtensionFillsActorFieldsAndWritesLogs(t *testing.T) {
	ctx := audit.WithActor(context.Background(), 42)
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		Extensions: []oro.Extension{
			audit.Extension(audit.WithDefaultLogModel()),
		},
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close(ctx)

	if err := db.Register(article{}, audit.Log{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}

	created, err := db.Use[article]().Create(ctx, &article{Title: "draft"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !created.CreatedBy.Valid || created.CreatedBy.Value != 42 {
		t.Fatalf("expected created_by to be set, got %#v", created.CreatedBy)
	}
	if !created.UpdatedBy.Valid || created.UpdatedBy.Value != 42 {
		t.Fatalf("expected updated_by to be set, got %#v", created.UpdatedBy)
	}

	if _, err := db.Use[article]().Where("ID", created.ID).Update(ctx, oro.Map{"Title": "published"}); err != nil {
		t.Fatalf("update: %v", err)
	}
	updated, err := db.Use[article]().Where("ID", created.ID).First(ctx)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if !updated.UpdatedBy.Valid || updated.UpdatedBy.Value != 42 {
		t.Fatalf("expected updated_by after update, got %#v", updated.UpdatedBy)
	}

	if _, err := db.Use[article]().Where("ID", created.ID).Delete(ctx); err != nil {
		t.Fatalf("delete: %v", err)
	}
	deleted, err := db.Use[article]().OnlyDeleted().Where("ID", created.ID).First(ctx)
	if err != nil {
		t.Fatalf("only deleted: %v", err)
	}
	if !deleted.DeletedBy.Valid || deleted.DeletedBy.Value != 42 {
		t.Fatalf("expected deleted_by after soft delete, got %#v", deleted.DeletedBy)
	}

	logs, err := db.Use[audit.Log]().OrderBy("ID").Get(ctx)
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	if len(logs) != 3 {
		t.Fatalf("expected 3 audit logs, got %d", len(logs))
	}
	if logs[0].Operation != "create" || logs[1].Operation != "update" || logs[2].Operation != "delete" {
		t.Fatalf("unexpected audit operations: %#v", logs)
	}
	if !logs[0].ActorID.Valid || logs[0].ActorID.Value != 42 {
		t.Fatalf("expected log actor_id")
	}
}
