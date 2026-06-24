package main

import (
	"context"
	"log"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
)

type Article struct {
	oro.Model
	Title string
}

func (Article) Define(s *oro.SchemaBuilder) {
	s.Table("articles")
	s.Field("Title").String()
}

func (article Article) Cover() oro.Relation {
	return oro.HasOne(article, "Cover", "Image").ForeignKey("ArticleID").ReferenceKey("ID")
}

func (article Article) Comments() oro.Relation {
	return oro.HasMany(article, "Comments", "Comment").ForeignKey("ArticleID").ReferenceKey("ID")
}

type Image struct {
	oro.Model
	ArticleID uint64
	URL       string
}

func (Image) Define(s *oro.SchemaBuilder) {
	s.Table("images")
	s.Field("ArticleID").UnsignedBigInt().Index()
	s.Field("URL").String()
}

func (image Image) Article() oro.Relation {
	return oro.BelongsTo(image, "Article", "Article").ForeignKey("ArticleID").ReferenceKey("ID")
}

type Comment struct {
	oro.Model
	ArticleID uint64
	Body      string
}

func (Comment) Define(s *oro.SchemaBuilder) {
	s.Table("comments")
	s.Field("ArticleID").UnsignedBigInt().Index()
	s.Field("Body").String()
}

func main() {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{Connections: map[string]oro.ConnectionConfig{
		"default": {Driver: sqlite.Open(":memory:")},
	}})
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close(ctx)

	if err := db.Register(Article{}, Image{}, Comment{}); err != nil {
		log.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		log.Fatal(err)
	}

	article, _ := db.Use[Article]().Create(ctx, &Article{Title: "Generics ORM"})
	_, _ = db.Use[Image]().Create(ctx, &Image{ArticleID: article.ID, URL: "cover.jpg"})
	_, _ = db.Use[Comment]().Create(ctx, &Comment{ArticleID: article.ID, Body: "Nice"})

	loaded, err := db.Use[Article]().With(Article{}.Cover()).With(Article{}.Comments()).First(ctx)
	if err != nil {
		log.Fatal(err)
	}
	cover, _ := loaded.Cover().One[Image]()
	comments, _ := loaded.Comments().Many[Comment]()

	log.Printf("article=%s cover=%s comments=%d", loaded.Title, cover.URL, len(comments))
}
