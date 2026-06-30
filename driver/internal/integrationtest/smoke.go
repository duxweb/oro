package integrationtest

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	oro "github.com/duxweb/oro"
)

type Product struct {
	oro.Model
	Code  string
	Price uint
	Stock oro.Null[uint]
}

func (Product) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_products")
	s.Field("Code").String().Unique()
	s.Field("Price").Uint()
	s.Field("Stock").Uint().Nullable()
}

type ProductDefault struct {
	oro.Model
	Code  string
	Price uint
	Stock uint
}

func (ProductDefault) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_product_defaults")
	s.Field("Code").String().Unique()
	s.Field("Price").Uint().Default(10)
	s.Field("Stock").Uint().Default(3)
}

type ProductView struct {
	ID    uint64
	Code  string
	Price uint
}

type User struct {
	oro.Model
	Name string
}

func (User) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_users")
	s.Field("Name").String()
}

type Order struct {
	oro.Model
	UserID uint64
	Total  uint
	Status string
}

func (Order) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_orders")
	s.Field("UserID").UnsignedBigInt().Index()
	s.Field("Total").Uint()
	s.Field("Status").String()
}

type OrderView struct {
	ID       uint64
	UserName string
	Total    uint
}

type Article struct {
	oro.Model
	Title string
}

func (Article) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_articles")
	s.Field("Title").String()
}

func (article Article) Cover() oro.Relation {
	return oro.HasOne(article, "Cover", "Image").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

func (article Article) Comments() oro.Relation {
	return oro.HasMany(article, "Comments", "Comment").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

func (article Article) Tags() oro.Relation {
	return oro.ManyToMany(article, "Tags", "Tag").
		Through("oro_integration_article_tags").
		SourceForeignKey("ArticleID").
		TargetForeignKey("TagID")
}

type ArticleAggregate struct {
	oro.Model
	Title         string
	CommentsCount int64
	TagsCount     int64
	CommentsTotal int64
}

func (ArticleAggregate) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_articles")
	s.Field("Title").String()
	s.Field("CommentsCount").Virtual()
	s.Field("TagsCount").Virtual()
	s.Field("CommentsTotal").Column("comments_total").Virtual()
}

func (article ArticleAggregate) Comments() oro.Relation {
	return oro.HasMany(article, "Comments", "Comment").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

func (article ArticleAggregate) Tags() oro.Relation {
	return oro.ManyToMany(article, "Tags", "Tag").
		Through("oro_integration_article_tags").
		SourceForeignKey("ArticleID").
		TargetForeignKey("TagID")
}

type Image struct {
	oro.Model
	ArticleID uint64
	URL       string
}

func (Image) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_images")
	s.Field("ArticleID").UnsignedBigInt().Index()
	s.Field("URL").String()
}

func (image Image) Article() oro.Relation {
	return oro.BelongsTo(image, "Article", "Article").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type Comment struct {
	oro.Model
	ArticleID uint64
	Body      string
	Status    string
}

func (Comment) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_comments")
	s.Field("ArticleID").UnsignedBigInt().Index()
	s.Field("Body").String()
	s.Field("Status").String().Index()
}

func (comment Comment) Article() oro.Relation {
	return oro.BelongsTo(comment, "Article", "Article").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type Tag struct {
	oro.Model
	Name string
}

func (Tag) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_tags")
	s.Field("Name").String().Unique()
}

type ArticleTag struct {
	oro.Model
	ArticleID uint64
	TagID     uint64
	Sort      uint
}

func (ArticleTag) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_article_tags")
	s.Field("ArticleID").UnsignedBigInt().Index()
	s.Field("TagID").UnsignedBigInt().Index()
	s.Field("Sort").Uint().Nullable()
	s.Unique("uk_oro_integration_article_tags_pair", "ArticleID", "TagID")
}

type DynamicArticle struct {
	oro.Model
	Title       string
	ImagesCount int64
	TagsCount   int64
}

func (DynamicArticle) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_dynamic_articles")
	s.Field("Title").String()
	s.Field("ImagesCount").Virtual()
	s.Field("TagsCount").Virtual()
}

func (article DynamicArticle) Images() oro.Relation {
	return oro.DynamicHasMany(article, "Images", "DynamicImage").
		IDField("OwnerID").
		TypeField("OwnerType").
		TypeValue("DynamicArticle")
}

func (article DynamicArticle) Tags() oro.Relation {
	return oro.DynamicManyToMany(article, "Tags", "DynamicTag").
		Through("oro_integration_dynamic_tag_links").
		SourceForeignKey("OwnerID").
		SourceType("OwnerType", "DynamicArticle").
		TargetForeignKey("TagID")
}

type DynamicProduct struct {
	oro.Model
	Code string
}

func (DynamicProduct) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_dynamic_products")
	s.Field("Code").String()
}

func (product DynamicProduct) Tags() oro.Relation {
	return oro.DynamicManyToMany(product, "Tags", "DynamicTag").
		Through("oro_integration_dynamic_tag_links").
		SourceForeignKey("OwnerID").
		SourceType("OwnerType", "DynamicProduct").
		TargetForeignKey("TagID")
}

type DynamicImage struct {
	oro.Model
	OwnerID   uint64
	OwnerType string
	URL       string
}

func (DynamicImage) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_dynamic_images")
	s.Field("OwnerID").UnsignedBigInt().Index()
	s.Field("OwnerType").String().Index()
	s.Field("URL").String()
}

func (image DynamicImage) Owner() oro.Relation {
	return oro.DynamicBelongsTo(image, "Owner").
		IDField("OwnerID").
		TypeField("OwnerType")
}

type DynamicTag struct {
	oro.Model
	Name string
}

func (DynamicTag) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_dynamic_tags")
	s.Field("Name").String()
}

type DynamicTagLink struct {
	oro.Model
	OwnerID   uint64
	OwnerType string
	TagID     uint64
	Sort      uint
}

func (DynamicTagLink) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_dynamic_tag_links")
	s.Field("OwnerID").UnsignedBigInt().Index()
	s.Field("OwnerType").String().Index()
	s.Field("TagID").UnsignedBigInt().Index()
	s.Field("Sort").Uint().Nullable()
}

type JSONProduct struct {
	oro.Model
	Code string
	Meta oro.JSONRaw
}

func (JSONProduct) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_json_products")
	s.Field("Code").String()
	s.Field("Meta").JSON()
}

type TimeItem struct {
	oro.Model
	Code     string
	Occurred time.Time
	Optional oro.Null[time.Time]
}

func (TimeItem) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_time_items")
	s.Field("Code").String().Unique()
	s.Field("Occurred").Timestamp().Index()
	s.Field("Optional").Timestamp().Nullable()
}

type SyncBase struct {
	oro.Model
	Code string
}

func (SyncBase) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_sync_items")
	s.Field("Code").String().Unique()
}

type SyncExpanded struct {
	oro.Model
	Code  string
	Stock uint
}

func (SyncExpanded) Define(s *oro.SchemaBuilder) {
	s.Table("oro_integration_sync_items")
	s.Field("Code").String().Unique()
	s.Field("Stock").Uint().Default(0)
}

type DriverCase struct {
	Name              string
	Driver            oro.Driver
	ConnectionTimeout time.Duration
}

func RunMatrix(t *testing.T, testCase DriverCase) {
	t.Helper()

	ctx := context.Background()

	t.Run("schema_sync", func(t *testing.T) {
		db := openWithModels(t, ctx, testCase)
		runSchemaSync(t, ctx, db)
	})
	t.Run("crud_query_write", func(t *testing.T) {
		db := open(t, ctx, testCase)
		runCRUDQueryWrite(t, ctx, db)
	})
	t.Run("transaction", func(t *testing.T) {
		db := open(t, ctx, testCase)
		runTransaction(t, ctx, db)
	})
	t.Run("join_subquery_aggregate", func(t *testing.T) {
		db := open(t, ctx, testCase)
		runJoinSubqueryAggregate(t, ctx, db)
	})
	t.Run("relations", func(t *testing.T) {
		db := open(t, ctx, testCase)
		runRelations(t, ctx, db)
	})
	t.Run("dynamic_relations", func(t *testing.T) {
		db := open(t, ctx, testCase)
		runDynamicRelations(t, ctx, db)
	})
	t.Run("json", func(t *testing.T) {
		db := open(t, ctx, testCase)
		runJSON(t, ctx, db)
	})
	t.Run("time_handling", func(t *testing.T) {
		displayLocation := time.FixedZone("UTC+08", 8*60*60)
		db := openWithLocation(t, ctx, testCase, displayLocation)
		runTimeHandling(t, ctx, db, displayLocation)
	})
}

func RunSmoke(t *testing.T, testCase DriverCase) {
	t.Helper()
	RunMatrix(t, testCase)
}

func open(t *testing.T, ctx context.Context, testCase DriverCase) *oro.DB {
	t.Helper()
	return openWithModels(t, ctx, testCase, matrixModels()...)
}

func openWithModels(t *testing.T, ctx context.Context, testCase DriverCase, models ...oro.Definer) *oro.DB {
	t.Helper()
	return openWithLocationAndModels(t, ctx, testCase, nil, models...)
}

func openWithLocation(t *testing.T, ctx context.Context, testCase DriverCase, loc *time.Location) *oro.DB {
	t.Helper()
	return openWithLocationAndModels(t, ctx, testCase, loc, matrixModels()...)
}

func openWithLocationAndModels(t *testing.T, ctx context.Context, testCase DriverCase, loc *time.Location, models ...oro.Definer) *oro.DB {
	t.Helper()

	timeout := testCase.ConnectionTimeout
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	db, err := oro.Open(oro.Config{
		Location: loc,
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: testCase.Driver},
		},
		Pool: oro.PoolConfig{
			MaxOpenConns: 4,
			MaxIdleConns: 2,
			PingOnOpen:   true,
		},
		Timeout: oro.TimeoutConfig{
			Connect: timeout,
			Query:   10 * time.Second,
		},
	})
	if err != nil {
		if isUnavailable(err) {
			t.Skipf("%s integration database unavailable: %v", testCase.Name, err)
		}
		t.Fatal(err)
	}
	reset(t, ctx, db)
	if len(models) > 0 {
		if err := db.Register(models...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		reset(t, ctx, db)
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	return db
}

func matrixModels() []oro.Definer {
	return []oro.Definer{
		Product{}, ProductDefault{}, User{}, Order{}, Article{}, Image{}, Comment{}, Tag{}, ArticleTag{}, ArticleAggregate{},
		DynamicArticle{}, DynamicProduct{}, DynamicImage{}, DynamicTag{}, DynamicTagLink{}, JSONProduct{}, TimeItem{},
	}
}

func reset(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	for _, table := range []string{
		"oro_integration_dynamic_tag_links",
		"oro_integration_article_tags",
		"oro_integration_dynamic_images",
		"oro_integration_dynamic_tags",
		"oro_integration_dynamic_products",
		"oro_integration_dynamic_articles",
		"oro_integration_json_products",
		"oro_integration_time_items",
		"oro_integration_comments",
		"oro_integration_images",
		"oro_integration_tags",
		"oro_integration_articles",
		"oro_integration_orders",
		"oro_integration_users",
		"oro_integration_product_defaults",
		"oro_integration_products",
		"oro_integration_sync_items",
		"oro_schema",
	} {
		_, _ = db.Raw("drop table if exists " + table).Exec(ctx)
	}
}

func runSchemaSync(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	if err := db.Register(SyncBase{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	base, err := db.Use[SyncBase]().Create(ctx, &SyncBase{Code: "S001"})
	if err != nil {
		t.Fatal(err)
	}
	if base.ID == 0 {
		t.Fatalf("expected created sync base id, got %#v", base)
	}
	if err := db.Register(SyncExpanded{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	row, err := db.Table("oro_integration_sync_items").Create(ctx, oro.Map{"code": "S002", "stock": 8})
	if err != nil {
		t.Fatal(err)
	}
	if gotUint(row["stock"]) != 8 {
		t.Fatalf("expected synced stock column, got %#v", row)
	}
}

func runCRUDQueryWrite(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := db.Use[Product]().Create(ctx, &Product{Code: "IT001", Price: 100, Stock: oro.NullOf[uint](7)})
	if err != nil {
		t.Fatal(err)
	}
	if created == nil || created.ID == 0 || created.Code != "IT001" || created.Price != 100 || !created.Stock.Valid || created.Stock.Value != 7 {
		t.Fatalf("unexpected created product %#v", created)
	}

	defaulted, err := db.Use[ProductDefault]().Create(ctx, &ProductDefault{Code: "DEF001"}, oro.Omit("Price", "Stock"))
	if err != nil {
		t.Fatal(err)
	}
	if defaulted.Price != 10 || defaulted.Stock != 3 {
		t.Fatalf("expected database defaults, got %#v", defaulted)
	}

	many, err := db.Use[Product]().CreateMany(ctx, []*Product{{Code: "IT002", Price: 200}, {Code: "IT003", Price: 300}})
	if err != nil {
		t.Fatal(err)
	}
	manyIDs, err := many.IDs[uint64]()
	if err != nil {
		t.Fatal(err)
	}
	if many.RowsAffected != 2 || len(manyIDs) != 2 || manyIDs[0] == 0 || manyIDs[1] == 0 {
		t.Fatalf("unexpected create many %#v", many)
	}

	row, err := db.Table("oro_integration_products").Create(ctx, oro.Map{"code": "IT004", "price": 400, "stock": nil})
	if err != nil {
		t.Fatal(err)
	}
	if row["id"] == nil || row["code"] != "IT004" {
		t.Fatalf("unexpected table create %#v", row)
	}
	mapped, err := db.Table("oro_integration_products").MapTo[ProductView]().Create(ctx, oro.Map{"code": "IT005", "price": 500})
	if err != nil {
		t.Fatal(err)
	}
	if mapped == nil || mapped.ID == 0 || mapped.Code != "IT005" || mapped.Price != 500 {
		t.Fatalf("unexpected mapped create %#v", mapped)
	}

	filtered, err := db.Use[Product]().
		Where("Price", ">=", 100).
		Where(oro.Field("Code").In("IT001", "IT002", "IT003", "IT004", "IT005")).
		WhereGroup(func(w *oro.WhereBuilder) {
			w.Where("Code", "IT001").OrWhere("Price", ">=", 300)
		}).
		WhereWhen(true, func(w *oro.WhereBuilder) {
			w.Where("Price", "<=", 500)
		}).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(filtered) != 4 {
		t.Fatalf("expected four filtered products, got %#v", filtered)
	}

	view, err := db.Table("oro_integration_products").
		Select("id", "code", "price").
		Where("code", "IT001").
		MapTo[ProductView]().
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || view.ID != created.ID || view.Code != "IT001" {
		t.Fatalf("unexpected mapped view %#v", view)
	}

	count, err := db.Use[Product]().OrderBy("ID").Where("Price", ">=", 200).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 4 {
		t.Fatalf("expected count 4, got %d", count)
	}
	exists, err := db.Table("oro_integration_products").OrderBy("id").Where("code", "IT001").Exists(ctx)
	if err != nil || !exists {
		t.Fatalf("expected exists, got %v %v", exists, err)
	}

	page, err := db.Use[Product]().OrderBy("ID").Paginate(2).Page(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 5 || page.Page != 2 || page.Size != 2 || len(page.Items) != 2 {
		t.Fatalf("unexpected page %#v", page)
	}

	sum, err := db.Use[Product]().Sum(ctx, "Price")
	if err != nil {
		t.Fatal(err)
	}
	if sum == "" {
		t.Fatalf("expected sum value, got %q", sum)
	}
	max, err := db.Use[Product]().Max[uint](ctx, "Price")
	if err != nil {
		t.Fatal(err)
	}
	if !max.Valid || max.Value != 500 {
		t.Fatalf("unexpected max %#v", max)
	}

	updated, err := db.Use[Product]().Where("Code", "IT001").Update(ctx, oro.Map{"Price": oro.Increment(50)})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one updated row, got %d", updated)
	}
	updatedProduct, err := db.Use[Product]().Where("Code", "IT001").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if updatedProduct.Price != 150 {
		t.Fatalf("unexpected increment result %#v", updatedProduct)
	}

	upserted, err := db.Use[Product]().Upsert(ctx, &Product{Code: "IT001", Price: 180}, oro.ConflictBy("Code").Update("Price"))
	if err != nil {
		t.Fatal(err)
	}
	if upserted.ID != created.ID || upserted.Price != 180 {
		t.Fatalf("unexpected upsert %#v", upserted)
	}

	stream, err := db.Use[Product]().OrderBy("ID").Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	streamCount := 0
	for stream.Next() {
		item := stream.Value()
		if item == nil {
			t.Fatal("unexpected nil stream item")
		}
		streamCount++
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}
	if streamCount != 5 {
		t.Fatalf("unexpected stream count %d", streamCount)
	}

	chunkCount := 0
	if err := db.Use[Product]().OrderBy("ID").Chunk(ctx, 2, func(items []*Product) error {
		chunkCount += len(items)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if chunkCount != 5 {
		t.Fatalf("unexpected chunk count %d", chunkCount)
	}

	deleted, err := db.Use[Product]().Where("Code", "IT005").ForceDelete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected one deleted row, got %d", deleted)
	}
	missing, err := db.Use[Product]().Where("Code", "IT005").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing row, got %#v", missing)
	}
}

func runTransaction(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	err := db.Transaction(ctx, func(tx *oro.DB) error {
		_, err := tx.Table("oro_integration_products").Create(ctx, oro.Map{"code": "TX001", "price": 10})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	exists, err := db.Table("oro_integration_products").Where("code", "TX001").Exists(ctx)
	if err != nil || !exists {
		t.Fatalf("expected committed row, got %v %v", exists, err)
	}
	rollbackErr := errors.New("rollback")
	err = db.Transaction(ctx, func(tx *oro.DB) error {
		if _, err := tx.Table("oro_integration_products").Create(ctx, oro.Map{"code": "TX002", "price": 20}); err != nil {
			return err
		}
		return rollbackErr
	})
	if !errors.Is(err, rollbackErr) {
		t.Fatalf("expected rollback error, got %v", err)
	}
	exists, err = db.Table("oro_integration_products").Where("code", "TX002").Exists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("expected rolled back row to be missing")
	}

	err = db.Transaction(ctx, func(tx *oro.DB) error {
		if _, err := tx.Table("oro_integration_products").Create(ctx, oro.Map{"code": "TX003", "price": 30}); err != nil {
			return err
		}
		_ = tx.Transaction(ctx, func(tx2 *oro.DB) error {
			if _, err := tx2.Table("oro_integration_products").Create(ctx, oro.Map{"code": "TX004", "price": 40}); err != nil {
				return err
			}
			return errors.New("inner rollback")
		})
		_, err := tx.Table("oro_integration_products").Create(ctx, oro.Map{"code": "TX005", "price": 50})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Table("oro_integration_products").Select("code").Where(oro.Field("code").In("TX003", "TX004", "TX005")).OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0]["code"] != "TX003" || rows[1]["code"] != "TX005" {
		t.Fatalf("unexpected nested transaction rows %#v", rows)
	}
}

func runJoinSubqueryAggregate(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	user, err := db.Use[User]().Create(ctx, &User{Name: "Alice"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[User]().Create(ctx, &User{Name: "Bob"}); err != nil {
		t.Fatal(err)
	}
	for _, order := range []*Order{
		{UserID: user.ID, Total: 100, Status: "paid"},
		{UserID: user.ID, Total: 200, Status: "paid"},
	} {
		if _, err := db.Use[Order]().Create(ctx, order); err != nil {
			t.Fatal(err)
		}
	}
	views, err := db.Table("oro_integration_orders").As("o").
		Select("o.id", oro.Raw("u.name as user_name"), "o.total").
		Join("oro_integration_users", func(j *oro.Join) {
			j.As("u").OnColumn("u.id", "o.user_id")
		}).
		Where("u.name", "Alice").
		OrderBy("o.total").
		MapTo[OrderView]().
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].UserName != "Alice" || views[1].Total != 200 {
		t.Fatalf("unexpected join views %#v", views)
	}

	sub := oro.Query(db.Table("oro_integration_orders").Select("user_id").Where("total", ">=", 200))
	users, err := db.Use[User]().WhereIn("ID", sub).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].Name != "Alice" {
		t.Fatalf("unexpected where-in subquery users %#v", users)
	}

	reports, err := db.Table("oro_integration_orders").
		Select("user_id", oro.Count("*").As("total")).
		GroupBy("user_id").
		HavingRaw("count(*) >= ?", 2).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || gotInt64(reports[0]["total"]) != 2 {
		t.Fatalf("unexpected grouped reports %#v", reports)
	}
}

func runRelations(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	article, err := db.Use[Article]().Create(ctx, &Article{Title: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	article2, err := db.Use[Article]().Create(ctx, &Article{Title: "A2"})
	if err != nil {
		t.Fatal(err)
	}
	cover, err := db.Use[Image]().Create(ctx, &Image{ArticleID: article.ID, URL: "cover.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[Comment]().Create(ctx, &Comment{ArticleID: article.ID, Body: "C1", Status: "approved"}); err != nil {
		t.Fatal(err)
	}
	pending, err := db.Use[Comment]().Create(ctx, &Comment{ArticleID: article.ID, Body: "C2", Status: "pending"})
	if err != nil {
		t.Fatal(err)
	}
	tag, err := db.Use[Tag]().Create(ctx, &Tag{Name: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).Attach(ctx, tag, oro.Map{"Sort": 1}); err != nil {
		t.Fatal(err)
	}

	loaded, err := db.Use[Article]().
		With(Article{}.Cover()).
		With(Article{}.Tags()).
		With("Comments", func(q *oro.RelationQuery) { q.Where("Status", "approved") }).
		Where("ID", article.ID).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	loadedCover, err := loaded.Cover().One[Image]()
	if err != nil {
		t.Fatal(err)
	}
	if loadedCover == nil || loadedCover.URL != "cover.jpg" {
		t.Fatalf("unexpected cover %#v", loadedCover)
	}
	comments, err := loaded.Comments().Many[Comment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].Body != "C1" {
		t.Fatalf("unexpected comments %#v", comments)
	}
	tags, err := loaded.Tags().Many[Tag]()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "Go" {
		t.Fatalf("unexpected tags %#v", tags)
	}

	orphan, err := db.Use[Image]().Create(ctx, &Image{URL: "orphan.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(orphan.Article()).Set(ctx, article2); err != nil {
		t.Fatal(err)
	}
	orphan, err = db.Use[Image]().Find(ctx, orphan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if orphan.ArticleID != article2.ID {
		t.Fatalf("unexpected belongs-to set %#v", orphan)
	}
	if err := db.Relation(orphan.Article()).Unset(ctx); err != nil {
		t.Fatal(err)
	}
	orphan, err = db.Use[Image]().Find(ctx, orphan.ID)
	if err != nil {
		t.Fatal(err)
	}
	if orphan.ArticleID != 0 {
		t.Fatalf("unexpected belongs-to unset %#v", orphan)
	}

	comment3, err := db.Use[Comment]().Create(ctx, &Comment{Body: "C3", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Comments()).Add(ctx, comment3); err != nil {
		t.Fatal(err)
	}
	comment3, err = db.Use[Comment]().Find(ctx, comment3.ID)
	if err != nil {
		t.Fatal(err)
	}
	if comment3.ArticleID != article.ID {
		t.Fatalf("unexpected has-many add %#v", comment3)
	}
	if err := db.Relation(article.Comments()).Remove(ctx, comment3); err != nil {
		t.Fatal(err)
	}
	comment3, err = db.Use[Comment]().Find(ctx, comment3.ID)
	if err != nil {
		t.Fatal(err)
	}
	if comment3.ArticleID != 0 {
		t.Fatalf("unexpected has-many remove %#v", comment3)
	}

	tag2, err := db.Use[Tag]().Create(ctx, &Tag{Name: "ORM"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).Attach(ctx, tag2, oro.Map{"Sort": 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).UpdateThrough(ctx, tag2, oro.Map{"Sort": 3}); err != nil {
		t.Fatal(err)
	}
	through, err := db.Table("oro_integration_article_tags").Where("article_id", article.ID).Where("tag_id", tag2.ID).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if through == nil || gotInt64(through["sort"]) != 3 {
		t.Fatalf("unexpected through row %#v", through)
	}
	if err := db.Relation(article.Tags()).Sync(ctx, []*Tag{tag2}); err != nil {
		t.Fatal(err)
	}
	forTags, err := db.Use[Tag]().For(article.Tags()).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(forTags) != 1 || forTags[0].Name != "ORM" {
		t.Fatalf("unexpected for tags %#v", forTags)
	}

	forCover, err := db.Use[Image]().For(article.Cover()).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if forCover == nil || forCover.ID != cover.ID {
		t.Fatalf("unexpected for cover %#v", forCover)
	}
	forArticle, err := db.Use[Article]().For(cover.Article()).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if forArticle == nil || forArticle.ID != article.ID {
		t.Fatalf("unexpected for article %#v", forArticle)
	}

	hasApproved, err := db.Use[Article]().WhereHas(Article{}.Comments(), func(q *oro.RelationQuery) {
		q.Where("Status", "approved")
	}).OrderBy("ID").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasApproved) != 1 || hasApproved[0].ID != article.ID {
		t.Fatalf("unexpected where has %#v", hasApproved)
	}
	withoutPending, err := db.Use[Article]().WhereDoesntHave(Article{}.Comments(), func(q *oro.RelationQuery) {
		q.Where("Status", "pending")
	}).OrderBy("ID").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutPending) != 1 || withoutPending[0].ID != article2.ID {
		t.Fatalf("unexpected where doesnt have %#v", withoutPending)
	}
	nested, err := db.Use[Article]().With("Comments.Article").Where("ID", article.ID).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nestedComments, err := nested.Comments().Many[Comment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedComments) == 0 {
		t.Fatal("expected nested comments")
	}
	nestedParent, err := nestedComments[0].Article().One[Article]()
	if err != nil {
		t.Fatal(err)
	}
	if nestedParent == nil || nestedParent.ID != article.ID {
		t.Fatalf("unexpected nested parent %#v", nestedParent)
	}

	if pending.ID == 0 {
		t.Fatalf("expected pending comment id")
	}
	if err := db.Register(ArticleAggregate{}); err != nil {
		t.Fatal(err)
	}
	aggregated, err := db.Use[ArticleAggregate]().
		WithCount(ArticleAggregate{}.Comments()).
		WithCount(ArticleAggregate{}.Tags()).
		Select(oro.SumOf(ArticleAggregate{}.Comments(), "ID").As("comments_total").Filter(func(q *oro.RelationQuery) {
			q.Where("Status", "approved")
		})).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(aggregated) != 2 || aggregated[0].CommentsCount != 2 || aggregated[0].TagsCount != 1 || aggregated[0].CommentsTotal == 0 {
		t.Fatalf("unexpected relation aggregates %#v", aggregated)
	}

	serialized, ok := oro.Serialize(loaded).(oro.Map)
	if !ok {
		t.Fatalf("unexpected serialized article %#v", serialized)
	}
	if _, ok := serialized["comments"].([]any); !ok {
		t.Fatalf("expected serialized comments, got %#v", serialized)
	}
}

func runDynamicRelations(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	article, err := db.Use[DynamicArticle]().Create(ctx, &DynamicArticle{Title: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := db.Use[DynamicProduct]().Create(ctx, &DynamicProduct{Code: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	image, err := db.Use[DynamicImage]().Create(ctx, &DynamicImage{URL: "a1.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Images()).Add(ctx, image); err != nil {
		t.Fatal(err)
	}
	image, err = db.Use[DynamicImage]().Find(ctx, image.ID)
	if err != nil {
		t.Fatal(err)
	}
	if image.OwnerID != article.ID || image.OwnerType != "DynamicArticle" {
		t.Fatalf("unexpected dynamic add %#v", image)
	}
	owner, err := db.Use[DynamicArticle]().For(image.Owner()).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if owner == nil || owner.ID != article.ID {
		t.Fatalf("unexpected dynamic owner %#v", owner)
	}

	tagGo, err := db.Use[DynamicTag]().Create(ctx, &DynamicTag{Name: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	tagSQL, err := db.Use[DynamicTag]().Create(ctx, &DynamicTag{Name: "SQL"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).Attach(ctx, tagGo, oro.Map{"Sort": 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(product.Tags()).Attach(ctx, tagSQL, oro.Map{"Sort": 2}); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.Use[DynamicArticle]().With(DynamicArticle{}.Images()).With(DynamicArticle{}.Tags()).Where("ID", article.ID).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	images, err := loaded.Images().Many[DynamicImage]()
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || images[0].URL != "a1.jpg" {
		t.Fatalf("unexpected dynamic images %#v", images)
	}
	tags, err := loaded.Tags().Many[DynamicTag]()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "Go" {
		t.Fatalf("unexpected dynamic tags %#v", tags)
	}

	hasTag, err := db.Use[DynamicArticle]().WhereHas(DynamicArticle{}.Tags(), func(q *oro.RelationQuery) {
		q.Where("Name", "Go")
	}).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasTag) != 1 || hasTag[0].ID != article.ID {
		t.Fatalf("unexpected dynamic where has %#v", hasTag)
	}
	aggregated, err := db.Use[DynamicArticle]().WithCount(DynamicArticle{}.Images()).WithCount(DynamicArticle{}.Tags()).Where("ID", article.ID).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if aggregated.ImagesCount != 1 || aggregated.TagsCount != 1 {
		t.Fatalf("unexpected dynamic aggregates %#v", aggregated)
	}
	if err := db.Relation(article.Images()).Remove(ctx, image); err != nil {
		t.Fatal(err)
	}
	image, err = db.Use[DynamicImage]().Find(ctx, image.ID)
	if err != nil {
		t.Fatal(err)
	}
	if image.OwnerID != 0 || image.OwnerType != "" {
		t.Fatalf("unexpected dynamic remove %#v", image)
	}
}

func runJSON(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[JSONProduct]().Create(ctx, &JSONProduct{
		Code: "J001",
		Meta: oro.JSONRaw(`{"profile":{"country":"CN"},"vip":true}`),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[JSONProduct]().Create(ctx, &JSONProduct{
		Code: "J002",
		Meta: oro.JSONRaw(`{"profile":{"country":"US"},"vip":false}`),
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Use[JSONProduct]().Where(oro.JSON("Meta").Path("profile", "country").Eq("CN")).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Code != "J001" {
		t.Fatalf("unexpected json equal rows %#v", rows)
	}
	exists, err := db.Use[JSONProduct]().Where(oro.JSON("Meta").Path("vip").Exists()).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if exists != 2 {
		t.Fatalf("unexpected json exists count %d", exists)
	}
}

type TimeItemDTO struct {
	Code     string
	Occurred time.Time
	Optional oro.Null[time.Time]
}

func runTimeHandling(t *testing.T, ctx context.Context, db *oro.DB, displayLocation *time.Location) {
	t.Helper()
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	inputLocation := time.FixedZone("UTC-07", -7*60*60)
	inputTime := time.Date(2026, 6, 30, 9, 15, 30, 0, inputLocation)
	optionalTime := inputTime.Add(2 * time.Hour)

	created, err := db.Use[TimeItem]().Create(ctx, &TimeItem{
		Code:     "TIME001",
		Occurred: inputTime,
		Optional: oro.NullOf(optionalTime),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedAt.Location() != displayLocation || created.UpdatedAt.Location() != displayLocation {
		t.Fatalf("expected created timestamps in configured location, got %s %s", created.CreatedAt.Location(), created.UpdatedAt.Location())
	}
	if !created.Occurred.Equal(inputTime) || created.Occurred.Location() != displayLocation {
		t.Fatalf("expected created Occurred in configured location, got %s (%s)", created.Occurred, created.Occurred.Location())
	}
	if !created.Optional.Valid || !created.Optional.Value.Equal(optionalTime) || created.Optional.Value.Location() != displayLocation {
		t.Fatalf("expected optional time in configured location, got %#v", created.Optional)
	}

	found, err := db.Use[TimeItem]().
		Where("Occurred", inputTime.In(time.FixedZone("UTC+02", 2*60*60))).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != created.ID {
		t.Fatalf("expected WHERE time argument to match UTC instant, got %#v", found)
	}
	if found.Occurred.Location() != displayLocation || !found.Occurred.Equal(inputTime) {
		t.Fatalf("expected model read in configured location, got %s (%s)", found.Occurred, found.Occurred.Location())
	}

	row, err := db.Table("oro_integration_time_items").Where("id", created.ID).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rowTime, ok := row["occurred"].(time.Time)
	if !ok || rowTime.Location() != displayLocation || !rowTime.Equal(inputTime) {
		t.Fatalf("expected table row time in configured location, got %T %#v", row["occurred"], row["occurred"])
	}

	dto, err := db.Table("oro_integration_time_items").
		Where("id", created.ID).
		MapTo[TimeItemDTO]().
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || dto.Occurred.Location() != displayLocation || !dto.Occurred.Equal(inputTime) {
		t.Fatalf("expected MapTo time in configured location, got %#v", dto)
	}

	minTime, err := db.Use[TimeItem]().Min[time.Time](ctx, "Occurred")
	if err != nil {
		t.Fatal(err)
	}
	if !minTime.Valid || minTime.Value.Location() != displayLocation || !minTime.Value.Equal(inputTime) {
		t.Fatalf("expected model Min time in configured location, got %#v", minTime)
	}
	maxTime, err := db.Table("oro_integration_time_items").Max[time.Time](ctx, "occurred")
	if err != nil {
		t.Fatal(err)
	}
	if !maxTime.Valid || maxTime.Value.Location() != displayLocation || !maxTime.Value.Equal(inputTime) {
		t.Fatalf("expected table Max time in configured location, got %#v", maxTime)
	}

	dateRows, err := db.Use[TimeItem]().
		Where(oro.Time("Occurred").OnDate(inputTime.In(displayLocation))).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(dateRows) != 1 || dateRows[0].ID != created.ID {
		t.Fatalf("expected OnDate to match created row, got %#v", dateRows)
	}

	stream, err := db.Table("oro_integration_time_items").
		Select("occurred").
		Where("id", created.ID).
		Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	if !stream.Next() {
		t.Fatalf("expected stream row, err=%v", stream.Err())
	}
	streamTime, ok := stream.Value()["occurred"].(time.Time)
	if !ok || streamTime.Location() != displayLocation || !streamTime.Equal(inputTime) {
		t.Fatalf("expected stream time in configured location, got %T %#v", stream.Value()["occurred"], stream.Value()["occurred"])
	}
	if err := stream.Close(); err != nil {
		t.Fatal(err)
	}

	batchModels := []*TimeItem{
		{Code: "TIME002", Occurred: inputTime},
		{Code: "TIME003", Occurred: inputTime.Add(time.Minute)},
	}
	if _, err := db.Use[TimeItem]().CreateMany(ctx, batchModels); err != nil {
		t.Fatal(err)
	}
	if batchModels[0].CreatedAt.Location() != displayLocation || batchModels[1].UpdatedAt.Location() != displayLocation {
		t.Fatalf("expected batch timestamps in configured location, got %#v", batchModels)
	}

	resultModels := []*TimeItem{
		{Code: "TIME004", Occurred: inputTime},
		{Code: "TIME005", Occurred: inputTime.Add(2 * time.Minute)},
	}
	createdModels, err := db.Use[TimeItem]().CreateManyResult(ctx, resultModels)
	if err != nil {
		t.Fatal(err)
	}
	if len(createdModels) != 2 || createdModels[0].CreatedAt.Location() != displayLocation || createdModels[1].UpdatedAt.Location() != displayLocation {
		t.Fatalf("expected CreateManyResult timestamps in configured location, got %#v", createdModels)
	}
}

func isUnavailable(err error) bool {
	if err == nil {
		return false
	}
	var ormErr *oro.Error
	if errors.As(err, &ormErr) && ormErr.Cause != nil {
		err = ormErr.Cause
	}
	message := strings.ToLower(fmt.Sprint(err))
	unavailableParts := []string{
		"connection refused",
		"connect: connection refused",
		"no such host",
		"connection reset",
		"server closed",
		"timeout",
		"deadline exceeded",
		"access denied",
		"authentication failed",
		"password authentication failed",
		"unknown database",
		"database \"duxorm\" does not exist",
		"role \"root\" does not exist",
	}
	for _, part := range unavailableParts {
		if strings.Contains(message, part) {
			return true
		}
	}
	return false
}

func gotUint(value any) uint {
	switch typed := value.(type) {
	case uint:
		return typed
	case uint64:
		return uint(typed)
	case int64:
		return uint(typed)
	case int:
		return uint(typed)
	case int32:
		return uint(typed)
	case []byte:
		var result uint
		_, _ = fmt.Sscan(string(typed), &result)
		return result
	case string:
		var result uint
		_, _ = fmt.Sscan(typed, &result)
		return result
	default:
		return 0
	}
}

func gotInt64(value any) int64 {
	switch typed := value.(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case uint:
		return int64(typed)
	case uint64:
		return int64(typed)
	case []byte:
		var result int64
		_, _ = fmt.Sscan(string(typed), &result)
		return result
	case string:
		var result int64
		_, _ = fmt.Sscan(typed, &result)
		return result
	default:
		return 0
	}
}
