package softdelete_test

import (
	"context"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/softdelete"
	_ "modernc.org/sqlite"
)

type article struct {
	oro.Model
	softdelete.SoftDeleteFields
	Title string
}

func (article) Define(s *oro.SchemaBuilder) {
	s.Table("articles")
	s.Field("Title").String()
}

func TestSoftDeleteExtensionFields(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		Extensions: []oro.Extension{softdelete.Extension()},
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close(ctx)

	if err := db.Register(article{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}

	created, err := db.Use[article]().Create(ctx, &article{Title: "draft"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.Use[article]().Where("ID", created.ID).Delete(ctx); err != nil {
		t.Fatalf("delete: %v", err)
	}

	row, err := db.Use[article]().Where("ID", created.ID).First(ctx)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if row != nil {
		t.Fatalf("expected default query to hide soft deleted row")
	}
	deleted, err := db.Use[article]().OnlyDeleted().Where("ID", created.ID).First(ctx)
	if err != nil {
		t.Fatalf("only deleted: %v", err)
	}
	if deleted == nil || !deleted.DeletedAt.Valid {
		t.Fatalf("expected soft deleted row with deleted_at")
	}
	if _, err := db.Use[article]().WithDeleted().Where("ID", created.ID).Restore(ctx); err != nil {
		t.Fatalf("restore: %v", err)
	}
	restored, err := db.Use[article]().Where("ID", created.ID).First(ctx)
	if err != nil {
		t.Fatalf("restored first: %v", err)
	}
	if restored == nil || restored.DeletedAt.Valid {
		t.Fatalf("expected restored row")
	}
}
