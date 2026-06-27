package audit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/extensions/audit"
	"github.com/duxweb/oro/extensions/internal/exttest"
	"github.com/duxweb/oro/extensions/softdelete"
)

type article struct {
	oro.Model
	softdelete.SoftDeleteFields
	audit.AuditFields
	Title string
}

type secretArticle struct {
	oro.Model
	Title        string
	PasswordHash string
}

func (secretArticle) Define(s *oro.SchemaBuilder) {
	s.Table("oro_audit_secret_articles")
	s.Field("Title").String()
	s.Field("PasswordHash").Column("password_hash").String().Hidden()
}

func (article) Define(s *oro.SchemaBuilder) {
	s.Table("oro_audit_articles")
	s.Field("Title").String()
}

func TestAuditExtensionOmitsHiddenValues(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			ctx := audit.WithActor(context.Background(), 7)
			db, _ := exttest.Open(t, testCase, exttest.OpenOptions{
				Models: []oro.Definer{secretArticle{}, audit.Log{}},
				Tables: []string{"oro_audit_logs", "oro_audit_secret_articles"},
				Prefix: "audit_hidden_matrix_",
				Extensions: []oro.Extension{
					audit.Extension(audit.WithDefaultLogModel()),
				},
			})

			if _, err := db.Use[secretArticle]().Create(ctx, &secretArticle{Title: "draft", PasswordHash: "secret-hash"}); err != nil {
				t.Fatalf("create: %v", err)
			}
			log, err := db.Use[audit.Log]().OrderBy("ID").First(ctx)
			if err != nil {
				t.Fatalf("log: %v", err)
			}
			values := oro.Map{}
			if err := json.Unmarshal(log.Values, &values); err != nil {
				t.Fatalf("decode values: %v", err)
			}
			if _, ok := values["password_hash"]; ok {
				t.Fatalf("hidden password_hash leaked into audit log: %#v", values)
			}
			if _, ok := values["PasswordHash"]; ok {
				t.Fatalf("hidden PasswordHash leaked into audit log: %#v", values)
			}
			if values["title"] != "draft" {
				t.Fatalf("expected visible title, got %#v", values)
			}
		})
	}
}

func TestAuditExtensionFillsActorFieldsAndWritesLogs(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			ctx := audit.WithActor(context.Background(), 42)
			db, _ := exttest.Open(t, testCase, exttest.OpenOptions{
				Models: []oro.Definer{article{}, audit.Log{}},
				Tables: []string{"oro_audit_logs", "oro_audit_articles"},
				Prefix: "audit_matrix_",
				Extensions: []oro.Extension{
					audit.Extension(audit.WithDefaultLogModel()),
				},
			})

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
		})
	}
}
