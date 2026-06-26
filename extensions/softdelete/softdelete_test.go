package softdelete_test

import (
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/extensions/internal/exttest"
	"github.com/duxweb/oro/extensions/softdelete"
)

type article struct {
	oro.Model
	softdelete.SoftDeleteFields
	Title string
}

func (article) Define(s *oro.SchemaBuilder) {
	s.Table("oro_softdelete_articles")
	s.Field("Title").String()
}

func TestSoftDeleteExtensionFields(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			db, ctx := exttest.Open(t, testCase, exttest.OpenOptions{
				Models:     []oro.Definer{article{}},
				Tables:     []string{"oro_softdelete_articles"},
				Prefix:     "softdelete_matrix_",
				Extensions: []oro.Extension{softdelete.Extension()},
			})

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
		})
	}
}
