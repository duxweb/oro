package oro

import (
	"errors"
	"testing"
)

func openRegistryTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(Config{
		Connections: map[string]ConnectionConfig{
			"default": {Driver: fakeDriver{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

type relationArticle struct {
	Model
	Title string
}

func (relationArticle) Define(s *SchemaBuilder) {
	s.Table("relation_articles")
	s.Field("Title").String()
}

func (article relationArticle) Cover() Relation {
	return HasOne(article, "Cover", "oro.relationImage").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

func (article relationArticle) Comments() Relation {
	return HasMany(article, "Comments", "relationComment").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

func (article relationArticle) Tags() Relation {
	return ManyToMany(article, "Tags", "relationTag").
		Through("relation_article_tags").
		SourceForeignKey("ArticleID").
		TargetForeignKey("TagID")
}

func (article relationArticle) Images() Relation {
	return DynamicHasMany(article, "Images", "relationDynamicImage").
		IDField("OwnerID").
		TypeField("OwnerType").
		TypeValue("relationArticle")
}

type relationImage struct {
	Model
	ArticleID uint64
	URL       string
}

func (relationImage) Define(s *SchemaBuilder) {
	s.Table("relation_images")
	s.Field("ArticleID").UnsignedBigInt()
	s.Field("URL").String()
}

func (image relationImage) Article() Relation {
	return BelongsTo(image, "Article", "relationArticle").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type relationComment struct {
	Model
	ArticleID uint64
	Body      string
}

func (relationComment) Define(s *SchemaBuilder) {
	s.Table("relation_comments")
	s.Field("ArticleID").UnsignedBigInt()
	s.Field("Body").String()
}

type relationTag struct {
	Model
	Name string
}

func (relationTag) Define(s *SchemaBuilder) {
	s.Table("relation_tags")
	s.Field("Name").String()
}

type relationDynamicImage struct {
	Model
	OwnerID   uint64
	OwnerType string
	URL       string
}

func (relationDynamicImage) Define(s *SchemaBuilder) {
	s.Table("relation_dynamic_images")
	s.Field("OwnerID").UnsignedBigInt()
	s.Field("OwnerType").String()
	s.Field("URL").String()
}

func (image relationDynamicImage) Owner() Relation {
	return DynamicBelongsTo(image, "Owner").
		IDField("OwnerID").
		TypeField("OwnerType")
}

func TestRelationRegisterScansAndValidatesRelations(t *testing.T) {
	db := openRegistryTestDB(t)

	err := db.Register(
		relationArticle{},
		relationImage{},
		relationComment{},
		relationTag{},
		relationDynamicImage{},
	)
	if err != nil {
		t.Fatal(err)
	}

	schema, ok := db.runtime.Registry.Get(relationArticle{})
	if !ok {
		t.Fatal("expected relationArticle schema")
	}
	if len(schema.Relations) != 4 {
		t.Fatalf("expected 4 relations, got %#v", schema.Relations)
	}

	cover := relationByName(schema, "Cover")
	if cover.Kind != RelationHasOne || cover.TargetModel != "oro.relationImage" || cover.ForeignKey != "ArticleID" || cover.ReferenceKey != "ID" {
		t.Fatalf("unexpected cover relation %#v", cover)
	}
	tags := relationByName(schema, "Tags")
	if tags.Kind != RelationManyToMany || tags.Through != "relation_article_tags" || tags.SourceForeignKey != "ArticleID" || tags.TargetForeignKey != "TagID" {
		t.Fatalf("unexpected tags relation %#v", tags)
	}
	images := relationByName(schema, "Images")
	if images.Kind != RelationDynamicHasMany || images.IDField != "OwnerID" || images.TypeField != "OwnerType" || images.TypeValue != "relationArticle" {
		t.Fatalf("unexpected images relation %#v", images)
	}

	imageSchema, ok := db.runtime.Registry.Get(relationImage{})
	if !ok {
		t.Fatal("expected relationImage schema")
	}
	article := relationByName(imageSchema, "Article")
	if article.Kind != RelationBelongsTo || article.ForeignKey != "ArticleID" || article.ReferenceKey != "ID" {
		t.Fatalf("unexpected belongs to relation %#v", article)
	}
}

func TestRelationGenericAccessReturnsNotLoaded(t *testing.T) {
	article := relationArticle{}
	_, err := article.Cover().One[relationImage]()
	if !errors.Is(err, ErrRelationNotLoaded) {
		t.Fatalf("expected relation not loaded, got %v", err)
	}
	_, err = article.Comments().Many[relationComment]()
	if !errors.Is(err, ErrRelationNotLoaded) {
		t.Fatalf("expected relation not loaded, got %v", err)
	}
}

func TestRelationRegisterRejectsUnknownTarget(t *testing.T) {
	db := openRegistryTestDB(t)
	err := db.Register(relationUnknownTarget{})
	if !errors.Is(err, ErrUnknownRelation) {
		t.Fatalf("expected unknown relation, got %v", err)
	}
}

type relationUnknownTarget struct {
	Model
	Title string
}

func (relationUnknownTarget) Define(s *SchemaBuilder) {
	s.Table("relation_unknown_targets")
	s.Field("Title").String()
}

func (model relationUnknownTarget) Missing() Relation {
	return HasOne(model, "Missing", "missing.Model").
		ForeignKey("OwnerID").
		ReferenceKey("ID")
}

func TestRelationRegisterRejectsUnknownField(t *testing.T) {
	db := openRegistryTestDB(t)
	err := db.Register(relationBadFieldSource{}, relationBadFieldTarget{})
	if !errors.Is(err, ErrUnknownField) {
		t.Fatalf("expected unknown field, got %v", err)
	}
}

type relationBadFieldSource struct {
	Model
}

func (relationBadFieldSource) Define(s *SchemaBuilder) {
	s.Table("relation_bad_field_sources")
}

func (model relationBadFieldSource) Target() Relation {
	return HasOne(model, "Target", "relationBadFieldTarget").
		ForeignKey("MissingID").
		ReferenceKey("ID")
}

type relationBadFieldTarget struct {
	Model
	SourceID uint64
}

func (relationBadFieldTarget) Define(s *SchemaBuilder) {
	s.Table("relation_bad_field_targets")
	s.Field("SourceID").UnsignedBigInt()
}

func relationByName(schema *ModelSchema, name string) RelationSchema {
	for _, relation := range schema.Relations {
		if relation.Name == name {
			return relation
		}
	}
	return RelationSchema{}
}
