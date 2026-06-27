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

type relationArticle struct {
	oro.Model
	softdelete.SoftDeleteFields
	Title string
}

func (relationArticle) Define(s *oro.SchemaBuilder) {
	s.Table("sd_articles")
	s.Field("Title").String()
}

func (article relationArticle) Comments() oro.Relation {
	return oro.HasMany(article, "Comments", "relationComment").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type relationComment struct {
	oro.Model
	softdelete.SoftDeleteFields
	ArticleID uint64
	Body      string
}

func (relationComment) Define(s *oro.SchemaBuilder) {
	s.Table("sd_comments")
	s.Field("ArticleID").UnsignedBigInt()
	s.Field("Body").String()
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

func TestSoftDeleteScopesPreloadedRelations(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			db, ctx := exttest.Open(t, testCase, exttest.OpenOptions{
				Models:     []oro.Definer{relationArticle{}, relationComment{}},
				Tables:     []string{"sd_comments", "sd_articles"},
				Prefix:     "sdrel_",
				Extensions: []oro.Extension{softdelete.Extension()},
			})

			created, err := db.Use[relationArticle]().Create(ctx, &relationArticle{Title: "draft"})
			if err != nil {
				t.Fatalf("create article: %v", err)
			}
			visible, err := db.Use[relationComment]().Create(ctx, &relationComment{ArticleID: created.ID, Body: "visible"})
			if err != nil {
				t.Fatalf("create visible comment: %v", err)
			}
			hidden, err := db.Use[relationComment]().Create(ctx, &relationComment{ArticleID: created.ID, Body: "hidden"})
			if err != nil {
				t.Fatalf("create hidden comment: %v", err)
			}
			if _, err := db.Use[relationComment]().Where("ID", hidden.ID).Delete(ctx); err != nil {
				t.Fatalf("delete hidden comment: %v", err)
			}

			loaded, err := db.Use[relationArticle]().With(relationArticle{}.Comments()).Where("ID", created.ID).First(ctx)
			if err != nil {
				t.Fatalf("load article: %v", err)
			}
			comments, err := loaded.Comments().Many[relationComment]()
			if err != nil {
				t.Fatalf("comments: %v", err)
			}
			if len(comments) != 1 || comments[0].ID != visible.ID {
				t.Fatalf("expected only visible comment, got %#v", comments)
			}

			loadedWithDeleted, err := db.Use[relationArticle]().With(relationArticle{}.Comments(), func(q *oro.RelationQuery) {
				q.WithDeleted()
			}).Where("ID", created.ID).First(ctx)
			if err != nil {
				t.Fatalf("load with deleted: %v", err)
			}
			allComments, err := loadedWithDeleted.Comments().Many[relationComment]()
			if err != nil {
				t.Fatalf("with deleted comments: %v", err)
			}
			if len(allComments) != 2 {
				t.Fatalf("expected both comments with deleted, got %#v", allComments)
			}
		})
	}
}
