package oro_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/softdelete"
	_ "modernc.org/sqlite"
)

type integrationProduct struct {
	oro.Model
	softdelete.SoftDeleteFields
	Code  string
	Price uint
}

func (integrationProduct) Define(s *oro.SchemaBuilder) {
	s.Table("products")
	s.Field("Code").String()
	s.Field("Price").Uint()
}

type integrationProductAggregate struct {
	oro.Model
	Code       string
	Price      uint
	TotalPrice int64
}

func (integrationProductAggregate) Define(s *oro.SchemaBuilder) {
	s.Table("products")
	s.Field("Code").String()
	s.Field("Price").Uint()
	s.Field("TotalPrice").Column("total_price").BigInt().Virtual()
}

type integrationUser struct {
	oro.Model
	Email        string
	PasswordHash string
}

func (integrationUser) Define(s *oro.SchemaBuilder) {
	s.Table("users")
	s.Field("Email").String()
	s.Field("PasswordHash").Column("password_hash").String().Hidden()
}

type integrationStock struct {
	oro.Model
	Code    string
	Stock   uint
	Version uint
}

func (integrationStock) Define(s *oro.SchemaBuilder) {
	s.Table("stocks")
	s.Field("Code").String()
	s.Field("Stock").Uint()
	s.Field("Version").Uint().Default(1).OptimisticLock()
}

type integrationMappedDTO struct {
	Code  string
	Price uint
}

type integrationCustomMapperFactory struct {
	oro.DefaultFactory
}

func (integrationCustomMapperFactory) NewMapper(rt *oro.Runtime) oro.Mapper {
	return integrationCustomMapper{}
}

type integrationCustomMapper struct{}

func (integrationCustomMapper) MapModel(schema *oro.ModelSchema, row oro.Map, dest any) error {
	if product, ok := dest.(*integrationProduct); ok {
		product.Code = "mapped-model"
		product.Price = 777
	}
	return nil
}

func (integrationCustomMapper) MapDTO(row oro.Map, dest any) error {
	if dto, ok := dest.(*integrationMappedDTO); ok {
		dto.Code = "mapped-dto"
		dto.Price = 888
	}
	return nil
}

type integrationOrderView struct {
	ID       uint64
	UserName string
	Total    uint
}

type integrationJSONProduct struct {
	oro.Model
	Code string
	Meta oro.JSONRaw
}

func (integrationJSONProduct) Define(s *oro.SchemaBuilder) {
	s.Table("json_products")
	s.Field("Code").String()
	s.Field("Meta").JSON()
}

type integrationTimeModel struct {
	oro.Model
	softdelete.SoftDeleteFields
	Code     string
	Occurred time.Time
	Optional oro.Null[time.Time]
}

func (integrationTimeModel) Define(s *oro.SchemaBuilder) {
	s.Table("time_models")
	s.Field("Code").String()
	s.Field("Occurred").Timestamp()
	s.Field("Optional").Timestamp().Nullable()
}

type integrationTimeDTO struct {
	Code     string
	Occurred time.Time
	Optional oro.Null[time.Time]
}

type integrationArticle struct {
	oro.Model
	Title string
}

func (integrationArticle) Define(s *oro.SchemaBuilder) {
	s.Table("articles")
	s.Field("Title").String()
}

func (article integrationArticle) Cover() oro.Relation {
	return oro.HasOne(article, "Cover", "integrationImage").
		ForeignKey("ArticleID").
		ReferenceKey("ID").
		JSONName("cover_image")
}

func (article integrationArticle) Comments() oro.Relation {
	return oro.HasMany(article, "Comments", "integrationComment").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

func (article integrationArticle) Tags() oro.Relation {
	return oro.ManyToMany(article, "Tags", "integrationTag").
		Through("article_tags").
		SourceForeignKey("ArticleID").
		TargetForeignKey("TagID")
}

type integrationArticleAggregate struct {
	oro.Model
	Title            string
	CommentsCount    int64
	CommentsExists   bool
	CommentsIDSum    int64
	ApprovedComments int64
}

func (integrationArticleAggregate) Define(s *oro.SchemaBuilder) {
	s.Table("articles")
	s.Field("Title").String()
	s.Field("CommentsCount").Virtual()
	s.Field("CommentsExists").Virtual()
	s.Field("CommentsIDSum").Column("comments_id_sum").Virtual()
	s.Field("ApprovedComments").Column("approved_comments").Virtual()
}

func (article integrationArticleAggregate) Comments() oro.Relation {
	return oro.HasMany(article, "Comments", "integrationComment").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type integrationImage struct {
	oro.Model
	ArticleID uint64
	URL       string
}

func (integrationImage) Define(s *oro.SchemaBuilder) {
	s.Table("images")
	s.Field("ArticleID").UnsignedBigInt()
	s.Field("URL").String()
}

func (image integrationImage) Article() oro.Relation {
	return oro.BelongsTo(image, "Article", "integrationArticle").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type integrationComment struct {
	oro.Model
	ArticleID uint64
	Body      string
	Status    string
}

func (integrationComment) Define(s *oro.SchemaBuilder) {
	s.Table("comments")
	s.Field("ArticleID").UnsignedBigInt()
	s.Field("Body").String()
	s.Field("Status").String()
}

func (comment integrationComment) Article() oro.Relation {
	return oro.BelongsTo(comment, "Article", "integrationArticle").
		ForeignKey("ArticleID").
		ReferenceKey("ID")
}

type integrationTag struct {
	oro.Model
	Name string
}

func (integrationTag) Define(s *oro.SchemaBuilder) {
	s.Table("tags")
	s.Field("Name").String()
}

type integrationDynamicArticle struct {
	oro.Model
	Title       string
	ImagesCount int64
	TagsCount   int64
}

func (integrationDynamicArticle) Define(s *oro.SchemaBuilder) {
	s.Table("dynamic_articles")
	s.Field("Title").String()
	s.Field("ImagesCount").Virtual()
	s.Field("TagsCount").Virtual()
}

func (article integrationDynamicArticle) Images() oro.Relation {
	return oro.DynamicHasMany(article, "Images", "integrationDynamicImage").
		IDField("OwnerID").
		TypeField("OwnerType").
		TypeValue("integrationDynamicArticle")
}

func (article integrationDynamicArticle) Tags() oro.Relation {
	return oro.DynamicManyToMany(article, "Tags", "integrationDynamicTag").
		Through("dynamic_tag_links").
		SourceForeignKey("OwnerID").
		SourceType("OwnerType", "integrationDynamicArticle").
		TargetForeignKey("TagID")
}

type integrationDynamicProduct struct {
	oro.Model
	Code string
}

func (integrationDynamicProduct) Define(s *oro.SchemaBuilder) {
	s.Table("dynamic_products")
	s.Field("Code").String()
}

func (product integrationDynamicProduct) Tags() oro.Relation {
	return oro.DynamicManyToMany(product, "Tags", "integrationDynamicTag").
		Through("dynamic_tag_links").
		SourceForeignKey("OwnerID").
		SourceType("OwnerType", "integrationDynamicProduct").
		TargetForeignKey("TagID")
}

type integrationDynamicImage struct {
	oro.Model
	OwnerID   uint64
	OwnerType string
	URL       string
}

func (integrationDynamicImage) Define(s *oro.SchemaBuilder) {
	s.Table("dynamic_images")
	s.Field("OwnerID").UnsignedBigInt()
	s.Field("OwnerType").String()
	s.Field("URL").String()
}

func (image integrationDynamicImage) Owner() oro.Relation {
	return oro.DynamicBelongsTo(image, "Owner").
		IDField("OwnerID").
		TypeField("OwnerType")
}

type integrationDynamicTag struct {
	oro.Model
	Name string
}

func (integrationDynamicTag) Define(s *oro.SchemaBuilder) {
	s.Table("dynamic_tags")
	s.Field("Name").String()
}

var integrationHookCalls []string

type integrationHookProduct struct {
	oro.Model
	softdelete.SoftDeleteFields
	Code  string
	Price uint
}

func (integrationHookProduct) Define(s *oro.SchemaBuilder) {
	s.Table("hook_products")
	s.Field("Code").String()
	s.Field("Price").Uint()
}

func (product *integrationHookProduct) BeforeCreate(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "before_create")
	if product.Code == "ERR" {
		return errors.New("blocked create")
	}
	product.Code = "hook-" + product.Code
	return nil
}

func (product *integrationHookProduct) AfterCreate(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "after_create")
	return nil
}

func (product *integrationHookProduct) BeforeUpdate(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "before_update")
	if h.Values["Price"] == uint(12) {
		h.Values["Price"] = uint(120)
	}
	return nil
}

func (product *integrationHookProduct) AfterUpdate(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "after_update")
	return nil
}

func (product *integrationHookProduct) BeforeDelete(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "before_delete")
	return nil
}

func (product *integrationHookProduct) AfterDelete(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "after_delete")
	return nil
}

func (product *integrationHookProduct) BeforeRestore(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "before_restore")
	return nil
}

func (product *integrationHookProduct) AfterRestore(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "after_restore")
	return nil
}

func (product *integrationHookProduct) AfterFind(ctx context.Context, h *oro.Hook) error {
	integrationHookCalls = append(integrationHookCalls, "after_find")
	return nil
}

func openSQLiteTestDB(t *testing.T) (*oro.DB, context.Context) {
	t.Helper()

	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})

	_, err = db.Raw(`
		create table products (
			id integer primary key autoincrement,
			code text not null unique,
			price integer not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Register(integrationProduct{}); err != nil {
		t.Fatal(err)
	}

	return db, ctx
}

func openSQLiteTestDBWithDriver(t *testing.T, driver oro.Driver) (*oro.DB, context.Context) {
	t.Helper()

	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: driver},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})

	_, err = db.Raw(`
		create table products (
			id integer primary key autoincrement,
			code text not null unique,
			price integer not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Register(integrationProduct{}); err != nil {
		t.Fatal(err)
	}

	return db, ctx
}

func openSQLiteTimeTestDB(t *testing.T, loc *time.Location) (*oro.DB, context.Context) {
	t.Helper()

	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Location: loc,
		Pool: oro.PoolConfig{
			MaxOpenConns: 1,
		},
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})

	_, err = db.Raw(`
		create table time_models (
			id integer primary key autoincrement,
			code text not null unique,
			occurred datetime,
			optional datetime,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if err := db.Register(integrationTimeModel{}); err != nil {
		t.Fatal(err)
	}

	return db, ctx
}

func TestSQLiteTimeValuesStoreUTCAndReadConfiguredLocation(t *testing.T) {
	displayLocation := time.FixedZone("UTC+08", 8*60*60)
	inputLocation := time.FixedZone("UTC-07", -7*60*60)
	inputTime := time.Date(2026, 6, 30, 9, 15, 30, 123456789, inputLocation)
	optionalTime := inputTime.Add(2 * time.Hour)
	db, ctx := openSQLiteTimeTestDB(t, displayLocation)

	created, err := db.Use[integrationTimeModel]().Create(ctx, &integrationTimeModel{
		Code:     "T001",
		Occurred: inputTime,
		Optional: oro.NullOf(optionalTime),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.CreatedAt.Location() != displayLocation || created.UpdatedAt.Location() != displayLocation {
		t.Fatalf("expected created model timestamps in configured location, got %s %s", created.CreatedAt.Location(), created.UpdatedAt.Location())
	}
	if !created.Occurred.Equal(inputTime) || created.Occurred.Location() != displayLocation {
		t.Fatalf("expected created Occurred to preserve instant in configured location, got %s (%s)", created.Occurred, created.Occurred.Location())
	}
	if !created.Optional.Valid || !created.Optional.Value.Equal(optionalTime) || created.Optional.Value.Location() != displayLocation {
		t.Fatalf("expected optional time in configured location, got %#v", created.Optional)
	}

	stored, err := db.Raw("select occurred, optional, created_at, updated_at from time_models where id = ?", created.ID).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]time.Time{
		"occurred":   inputTime,
		"optional":   optionalTime,
		"created_at": created.CreatedAt,
		"updated_at": created.UpdatedAt,
	} {
		got, ok := stored[key].(time.Time)
		if !ok {
			t.Fatalf("expected %s to scan as time.Time, got %T %#v", key, stored[key], stored[key])
		}
		if !got.Equal(want) || got.Location() != displayLocation {
			t.Fatalf("expected %s instant in configured location, got %s (%s), want instant %s", key, got, got.Location(), want)
		}
	}

	found, err := db.Use[integrationTimeModel]().
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

	dto, err := db.Table("time_models").
		Where("id", created.ID).
		MapTo[integrationTimeDTO]().
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if dto == nil || dto.Occurred.Location() != displayLocation || !dto.Occurred.Equal(inputTime) {
		t.Fatalf("expected MapTo time in configured location, got %#v", dto)
	}

	minTime, err := db.Use[integrationTimeModel]().Min[time.Time](ctx, "Occurred")
	if err != nil {
		t.Fatal(err)
	}
	if !minTime.Valid || minTime.Value.Location() != displayLocation || !minTime.Value.Equal(inputTime) {
		t.Fatalf("expected model Min time in configured location, got %#v", minTime)
	}
	maxTime, err := db.Table("time_models").Max[time.Time](ctx, "occurred")
	if err != nil {
		t.Fatal(err)
	}
	if !maxTime.Valid || maxTime.Value.Location() != displayLocation || !maxTime.Value.Equal(inputTime) {
		t.Fatalf("expected table Max time in configured location, got %#v", maxTime)
	}

	stream, err := db.Raw("select occurred from time_models where id = ?", created.ID).Stream(ctx)
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

	if _, err := db.Use[integrationTimeModel]().Where("ID", created.ID).Delete(ctx); err != nil {
		t.Fatal(err)
	}
	deleted, err := db.Use[integrationTimeModel]().WithDeleted().Find(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if deleted == nil || !deleted.DeletedAt.Valid {
		t.Fatalf("expected soft deleted row with deleted_at, got %#v", deleted)
	}
	if deleted.DeletedAt.Value.Location() != displayLocation {
		t.Fatalf("expected DeletedAt in configured location, got %s", deleted.DeletedAt.Value.Location())
	}

	batchModels := []*integrationTimeModel{
		{Code: "T001B1", Occurred: inputTime},
		{Code: "T001B2", Occurred: inputTime.Add(time.Minute)},
	}
	if _, err := db.Use[integrationTimeModel]().CreateMany(ctx, batchModels); err != nil {
		t.Fatal(err)
	}
	if batchModels[0].CreatedAt.Location() != displayLocation || batchModels[1].UpdatedAt.Location() != displayLocation {
		t.Fatalf("expected batch timestamps in configured location, got %#v", batchModels)
	}
}

func TestSQLiteTimeNullFieldStaysNull(t *testing.T) {
	db, ctx := openSQLiteTimeTestDB(t, time.UTC)

	created, err := db.Use[integrationTimeModel]().Create(ctx, &integrationTimeModel{
		Code:     "T002",
		Occurred: time.Date(2026, 6, 30, 12, 0, 0, 0, time.FixedZone("UTC+08", 8*60*60)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.Optional.Valid {
		t.Fatalf("expected create result optional time to stay null, got %#v", created.Optional)
	}

	found, err := db.Use[integrationTimeModel]().Find(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Optional.Valid {
		t.Fatalf("expected loaded optional time to stay null, got %#v", found)
	}
}

func TestSQLiteTimeExprRangeQueries(t *testing.T) {
	loc := time.FixedZone("UTC+08", 8*60*60)
	db, ctx := openSQLiteTimeTestDB(t, loc)
	day := time.Date(2026, 6, 30, 0, 0, 0, 0, loc)
	start, end := oro.DayBounds(day, loc)

	fixtures := []*integrationTimeModel{
		{Code: "D_PREV", Occurred: start.Add(-time.Second)},
		{Code: "D_START", Occurred: start},
		{Code: "D_MID", Occurred: start.Add(12 * time.Hour)},
		{Code: "D_END", Occurred: end},
		{Code: "M_NEXT", Occurred: time.Date(2026, 7, 1, 12, 0, 0, 0, loc)},
		{Code: "Y_NEXT", Occurred: time.Date(2027, 1, 1, 0, 0, 0, 0, loc)},
	}
	for _, model := range fixtures {
		if _, err := db.Use[integrationTimeModel]().Create(ctx, model); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").OnDate(day)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_MID", "D_START"}) {
		t.Fatalf("unexpected OnDate rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").Between(start, end)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_END", "D_MID", "D_START"}) {
		t.Fatalf("unexpected Between rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").InRange(start, end)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_MID", "D_START"}) {
		t.Fatalf("unexpected InRange rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").After(start)).
		Where(oro.Time("Occurred").Before(end)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_MID"}) {
		t.Fatalf("unexpected After/Before rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").From(start)).
		Where(oro.Time("Occurred").Until(end)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_MID", "D_START"}) {
		t.Fatalf("unexpected From/Until rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").NotBetween(start, end)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_PREV", "M_NEXT", "Y_NEXT"}) {
		t.Fatalf("unexpected NotBetween rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").InMonth(day)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_MID", "D_PREV", "D_START"}) {
		t.Fatalf("unexpected InMonth rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where(oro.Time("Occurred").InYear(day)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_END", "D_MID", "D_PREV", "D_START", "M_NEXT"}) {
		t.Fatalf("unexpected InYear rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		WhereGroup(func(w *oro.WhereBuilder) {
			w.Where(oro.Time("Occurred").OnDate(day)).
				OrWhere("Code", "Y_NEXT")
		}).
		Where("Code", "!=", "D_END").
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_MID", "D_START", "Y_NEXT"}) {
		t.Fatalf("unexpected grouped rows %#v", got)
	}

	rows, err = db.Use[integrationTimeModel]().
		Where("Code", "D_PREV").
		OrWhere(oro.Time("Occurred").OnDate(day)).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got := timeModelCodes(rows); !slices.Equal(got, []string{"D_MID", "D_PREV", "D_START"}) {
		t.Fatalf("unexpected OrWhere rows %#v", got)
	}
}

func timeModelCodes(models []*integrationTimeModel) []string {
	codes := make([]string, 0, len(models))
	for _, model := range models {
		codes = append(codes, model.Code)
	}
	return codes
}

func TestSQLiteTableCreateGetAndRaw(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	row, err := db.Table("products").Create(ctx, oro.Map{
		"code":  "A001",
		"price": uint(100),
	})
	if err != nil {
		t.Fatal(err)
	}
	if row["id"] == nil {
		t.Fatalf("expected inserted id in %#v", row)
	}
	if row["code"] != "A001" {
		t.Fatalf("got code %#v", row["code"])
	}

	found, err := db.Table("products").Where("code", "A001").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found["price"] != int64(100) {
		t.Fatalf("unexpected row %#v", found)
	}

	rawRows, err := db.Raw("select code, price from products where code = ?", "A001").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rawRows) != 1 || rawRows[0]["code"] != "A001" {
		t.Fatalf("unexpected raw rows %#v", rawRows)
	}
}

func TestSQLiteModelCreateAndGet(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	product := &integrationProduct{
		Code:  "A002",
		Price: 200,
	}
	created, err := db.Use[integrationProduct]().Create(ctx, product)
	if err != nil {
		t.Fatal(err)
	}
	if created.ID == 0 {
		t.Fatalf("expected created id, got %#v", created)
	}
	if created.Code != "A002" || created.Price != 200 {
		t.Fatalf("unexpected created model %#v", created)
	}
	if created.CreatedAt.IsZero() || created.UpdatedAt.IsZero() {
		t.Fatalf("expected auto timestamps, got %#v %#v", created.CreatedAt, created.UpdatedAt)
	}

	found, err := db.Use[integrationProduct]().Where("Code", "A002").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.ID != created.ID || found.Code != "A002" {
		t.Fatalf("unexpected found model %#v", found)
	}
	if found.DeletedAt.Valid {
		t.Fatal("expected null deleted_at")
	}

	byID, err := db.Use[integrationProduct]().Find(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if byID == nil || byID.Code != "A002" {
		t.Fatalf("unexpected model from Find %#v", byID)
	}
}

func TestSQLiteModelCreateWithOmitUsesDatabaseDefault(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})

	_, err = db.Raw(`
		create table products (
			id integer primary key autoincrement,
			code text not null,
			price integer not null default 88,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(integrationProduct{}); err != nil {
		t.Fatal(err)
	}

	created, err := db.Use[integrationProduct]().Create(ctx, &integrationProduct{
		Code:  "A003",
		Price: 0,
	}, oro.Omit("Price"))
	if err != nil {
		t.Fatal(err)
	}
	if created.Price != 88 {
		t.Fatalf("expected database default price, got %d", created.Price)
	}
}

func TestSQLiteRawExecRowsAffected(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Table("products").Create(ctx, oro.Map{
		"code":  "A004",
		"price": 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	affected, err := db.Raw("update products set price = ? where code = ?", 20, "A004").Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("expected one affected row, got %d", affected)
	}
}

func TestSQLiteRawMultiStatementRequiresOptIn(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	if _, err := db.Raw("select 1; select 2").Exec(ctx); !errors.Is(err, oro.ErrInvalidQuery) {
		t.Fatalf("expected invalid query for multi statement raw, got %v", err)
	}
	if _, err := db.Raw("select ';';").Get(ctx); err != nil {
		t.Fatalf("expected semicolon in string and trailing terminator to be allowed, got %v", err)
	}

	multiDB, err := oro.Open(oro.Config{
		AllowRawMultiStatement: true,
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := multiDB.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := multiDB.Raw("create table raw_multi_a (id integer); create table raw_multi_b (id integer)").Exec(ctx); err != nil {
		t.Fatalf("expected multi statement raw to run after opt-in, got %v", err)
	}
}

func TestSQLiteTableFirstNotFound(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	row, err := db.Table("products").Where("code", "missing").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row != nil {
		t.Fatalf("expected nil row, got %#v", row)
	}

	model, err := db.Use[integrationProduct]().Where("Code", "missing").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if model != nil {
		t.Fatalf("expected nil model, got %#v", model)
	}
}

func TestSQLiteSelectOrderLimitOffset(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "S001", "price": 100},
		{"code": "S002", "price": 200},
		{"code": "S003", "price": 300},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	rows, err := db.Table("products").
		Select("code", "price").
		OrderByDesc("price").
		Limit(1).
		Offset(1).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["code"] != "S002" {
		t.Fatalf("unexpected rows %#v", rows)
	}
	if _, ok := rows[0]["id"]; ok {
		t.Fatalf("expected selected columns only, got %#v", rows[0])
	}
}

func TestSQLiteModelSelectOrderLimit(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, product := range []*integrationProduct{
		{Code: "M001", Price: 100},
		{Code: "M002", Price: 200},
	} {
		if _, err := db.Use[integrationProduct]().Create(ctx, product); err != nil {
			t.Fatal(err)
		}
	}

	products, err := db.Use[integrationProduct]().
		Select("Code", "Price").
		OrderByDesc("Price").
		Limit(1).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 || products[0].Code != "M002" || products[0].Price != 200 {
		t.Fatalf("unexpected products %#v", products)
	}
	if products[0].ID != 0 {
		t.Fatalf("expected unselected ID to remain zero, got %#v", products[0])
	}
}

func TestSQLiteWhereConditionObjectsAndGroups(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, product := range []*integrationProduct{
		{Code: "WA001", Price: 50},
		{Code: "WB001", Price: 150},
		{Code: "WC001", Price: 250},
		{Code: "WD001", Price: 350},
	} {
		if _, err := db.Use[integrationProduct]().Create(ctx, product); err != nil {
			t.Fatal(err)
		}
	}

	products, err := db.Use[integrationProduct]().
		Where(
			oro.Or(
				oro.Field("Code").Like("WA%"),
				oro.Field("Code").Like("WB%"),
			),
			oro.Field("Price").Between(40, 200),
			oro.Not(oro.Field("Code").Eq("WA000")),
		).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 2 || products[0].Code != "WA001" || products[1].Code != "WB001" {
		t.Fatalf("unexpected condition object products %#v", products)
	}

	products, err = db.Use[integrationProduct]().
		WhereGroup(func(w *oro.WhereBuilder) {
			w.Where("Code", "like", "WA%").
				OrWhere("Code", "like", "WC%")
		}).
		WhereWhen(true, func(w *oro.WhereBuilder) {
			w.Where("Price", ">=", 200)
		}).
		OrderBy("Code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 1 || products[0].Code != "WC001" {
		t.Fatalf("unexpected grouped products %#v", products)
	}

	total, err := db.Use[integrationProduct]().
		WhereRaw("price >= ?", 300).
		OrWhereGroup(func(w *oro.WhereBuilder) {
			w.Where("Code", "WB001")
		}).
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("expected raw/or group count 2, got %d", total)
	}
}

func TestSQLiteModelHiddenFieldsRequireSelectHidden(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Raw(`
		create table users (
			id integer primary key autoincrement,
			email text not null unique,
			password_hash text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(integrationUser{}); err != nil {
		t.Fatal(err)
	}

	created, err := db.Use[integrationUser]().Create(ctx, &integrationUser{
		Email:        "user@example.com",
		PasswordHash: "hashed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.PasswordHash == "" {
		t.Fatalf("expected create returning hidden field, got %#v", created)
	}

	user, err := db.Use[integrationUser]().Where("Email", "user@example.com").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil || user.Email != "user@example.com" {
		t.Fatalf("unexpected user %#v", user)
	}
	if user.PasswordHash != "" {
		t.Fatalf("expected hidden field to stay zero, got %#v", user)
	}
	serializedUser, ok := oro.Serialize(user).(oro.Map)
	if !ok {
		t.Fatalf("unexpected serialized user %#v", serializedUser)
	}
	if _, ok := serializedUser["PasswordHash"]; ok {
		t.Fatalf("expected hidden field omitted, got %#v", serializedUser)
	}
	if _, ok := serializedUser["id"]; !ok {
		t.Fatalf("expected serialized model id, got %#v", serializedUser)
	}
	if _, ok := serializedUser["ID"]; ok {
		t.Fatalf("expected no Go field ID key, got %#v", serializedUser)
	}

	user, err = db.Use[integrationUser]().
		Select("ID", "Email").
		SelectHidden("PasswordHash").
		Where("Email", "user@example.com").
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil || user.ID == 0 || user.PasswordHash != "hashed" {
		t.Fatalf("expected selected hidden field, got %#v", user)
	}

	_, err = db.Use[integrationUser]().Select("PasswordHash").First(ctx)
	if err == nil {
		t.Fatal("expected hidden field select error")
	}
}

func TestSQLiteOptimisticLock(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Raw(`
		create table stocks (
			id integer primary key autoincrement,
			code text not null unique,
			stock integer not null,
			version integer not null default 1,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(integrationStock{}); err != nil {
		t.Fatal(err)
	}

	created, err := db.Use[integrationStock]().Create(ctx, &integrationStock{
		Code:    "L001",
		Stock:   10,
		Version: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	affected, err := db.Use[integrationStock]().
		Where("ID", created.ID).
		Update(ctx, oro.Map{"Stock": 20}, oro.CheckVersion(created.Version))
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("expected one affected row, got %d", affected)
	}

	stock, err := db.Use[integrationStock]().Find(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stock.Stock != 20 || stock.Version != 2 {
		t.Fatalf("expected updated stock and version, got %#v", stock)
	}

	_, err = db.Use[integrationStock]().
		Where("ID", created.ID).
		Update(ctx, oro.Map{"Stock": 30}, oro.CheckVersion(created.Version))
	if !errors.Is(err, oro.ErrStaleData) {
		t.Fatalf("expected stale data error, got %v", err)
	}

	_, err = db.Use[integrationStock]().
		Where("ID", created.ID).
		Update(ctx, oro.Map{"Version": 10}, oro.CheckVersion(stock.Version))
	if !errors.Is(err, oro.ErrInvalidArgument) {
		t.Fatalf("expected invalid version write, got %v", err)
	}
}

func TestSQLiteLimitZeroReturnsEmpty(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "Z001", "price": 1}); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Table("products").Limit(0).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected empty rows, got %#v", rows)
	}
}

func TestSQLiteModelPaginate(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, product := range []*integrationProduct{
		{Code: "P001", Price: 100},
		{Code: "P002", Price: 200},
		{Code: "P003", Price: 300},
	} {
		if _, err := db.Use[integrationProduct]().Create(ctx, product); err != nil {
			t.Fatal(err)
		}
	}

	page, err := db.Use[integrationProduct]().
		OrderBy("ID").
		Paginate(2).
		Page(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || page.Page != 2 || page.Size != 2 || page.Pages != 2 {
		t.Fatalf("unexpected page metadata %#v", page)
	}
	if len(page.Items) != 1 || page.Items[0].Code != "P003" {
		t.Fatalf("unexpected page items %#v", page.Items)
	}
}

func TestSQLiteTablePaginateAndMapTo(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "TP001", "price": 100},
		{"code": "TP002", "price": 200},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	page, err := db.Table("products").
		Select("code", "price").
		OrderBy("id").
		MapTo[integrationProduct]().
		Paginate(1).
		Page(ctx, 2)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Pages != 2 || len(page.Items) != 1 || page.Items[0].Code != "TP002" {
		t.Fatalf("unexpected dto page %#v", page)
	}
}

func TestSQLitePaginateRejectsInvalidInput(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Table("products").Paginate(0).Page(ctx, 1)
	if !errors.Is(err, oro.ErrInvalidArgument) {
		t.Fatalf("expected invalid size error, got %v", err)
	}
	_, err = db.Table("products").Paginate(10).Page(ctx, 0)
	if !errors.Is(err, oro.ErrInvalidArgument) {
		t.Fatalf("expected invalid page error, got %v", err)
	}
	_, err = db.Table("products").Limit(1).Paginate(10).Page(ctx, 1)
	if !errors.Is(err, oro.ErrInvalidQuery) {
		t.Fatalf("expected invalid query error, got %v", err)
	}
}

func TestSQLiteChunkAndEach(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "CH001", "price": 10},
		{"code": "CH002", "price": 20},
		{"code": "CH003", "price": 30},
		{"code": "CH004", "price": 40},
		{"code": "CH005", "price": 50},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	var batches []int
	var codes []string
	err := db.Use[integrationProduct]().
		Where("Price", ">=", 20).
		Chunk(ctx, 2, func(products []*integrationProduct) error {
			batches = append(batches, len(products))
			for _, product := range products {
				codes = append(codes, product.Code)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(batches) != 2 || batches[0] != 2 || batches[1] != 2 {
		t.Fatalf("unexpected batches %#v", batches)
	}
	if len(codes) != 4 || codes[0] != "CH002" || codes[3] != "CH005" {
		t.Fatalf("unexpected chunk codes %#v", codes)
	}

	var tableTotal int64
	err = db.Table("products").
		OrderBy("id").
		Each(ctx, func(row oro.Map) error {
			tableTotal += row["price"].(int64)
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if tableTotal != 150 {
		t.Fatalf("unexpected table total %d", tableTotal)
	}

	type productDTO struct {
		Code  string
		Price uint
	}
	var dtoCodes []string
	err = db.Table("products").
		OrderBy("id").
		MapTo[productDTO]().
		Chunk(ctx, 3, func(products []*productDTO) error {
			for _, product := range products {
				dtoCodes = append(dtoCodes, product.Code)
			}
			return nil
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(dtoCodes) != 5 || dtoCodes[0] != "CH001" || dtoCodes[4] != "CH005" {
		t.Fatalf("unexpected dto codes %#v", dtoCodes)
	}
}

func TestSQLiteChunkValidationAndCallbackError(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "CE001", "price": 10}); err != nil {
		t.Fatal(err)
	}
	if err := db.Use[integrationProduct]().Chunk(ctx, 0, func(products []*integrationProduct) error {
		return nil
	}); !errors.Is(err, oro.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
	if err := db.From(oro.Query(db.Table("products").Select("id")).As("p")).
		Chunk(ctx, 1, func(rows []oro.Map) error {
			return nil
		}); !errors.Is(err, oro.ErrOrderRequired) {
		t.Fatalf("expected order required, got %v", err)
	}

	stopErr := errors.New("stop chunk")
	err := db.Table("products").
		OrderBy("id").
		Chunk(ctx, 1, func(rows []oro.Map) error {
			return stopErr
		})
	if !errors.Is(err, stopErr) {
		t.Fatalf("expected callback error, got %v", err)
	}
}

func TestSQLiteStream(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "ST001", "price": 10},
		{"code": "ST002", "price": 20},
		{"code": "ST003", "price": 30},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	tableStream, err := db.Table("products").OrderBy("id").Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var tableCodes []string
	for tableStream.Next() {
		tableCodes = append(tableCodes, tableStream.Value()["code"].(string))
	}
	if err := tableStream.Err(); err != nil {
		t.Fatal(err)
	}
	if err := tableStream.Close(); err != nil {
		t.Fatal(err)
	}
	if len(tableCodes) != 3 || tableCodes[0] != "ST001" || tableCodes[2] != "ST003" {
		t.Fatalf("unexpected table stream codes %#v", tableCodes)
	}

	modelStream, err := db.Use[integrationProduct]().OrderBy("ID").Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var modelTotal uint
	for modelStream.Next() {
		modelTotal += modelStream.Value().Price
	}
	if err := modelStream.Err(); err != nil {
		t.Fatal(err)
	}
	if modelTotal != 60 {
		t.Fatalf("unexpected model stream total %d", modelTotal)
	}

	type productDTO struct {
		Code  string
		Price uint
	}
	dtoStream, err := db.Raw("select code, price from products order by id").MapTo[productDTO]().Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var dtoCodes []string
	for dtoStream.Next() {
		dtoCodes = append(dtoCodes, dtoStream.Value().Code)
	}
	if err := dtoStream.Err(); err != nil {
		t.Fatal(err)
	}
	if len(dtoCodes) != 3 || dtoCodes[1] != "ST002" {
		t.Fatalf("unexpected dto stream codes %#v", dtoCodes)
	}
}

func TestSQLiteCountExistsAndMapTo(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Table("products").Create(ctx, oro.Map{
		"code":  "A006",
		"price": 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Table("products").Create(ctx, oro.Map{
		"code":  "A007",
		"price": 20,
	})
	if err != nil {
		t.Fatal(err)
	}

	tableTotal, err := db.Table("products").Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tableTotal != 2 {
		t.Fatalf("expected table count 2, got %d", tableTotal)
	}

	modelTotal, err := db.Use[integrationProduct]().Where("Price", ">=", 20).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if modelTotal != 1 {
		t.Fatalf("expected model count 1, got %d", modelTotal)
	}

	exists, err := db.Table("products").Where("code", "A006").Exists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected table exists")
	}

	missing, err := db.Use[integrationProduct]().Where("Code", "missing").Exists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if missing {
		t.Fatal("expected model missing")
	}

	type productView struct {
		ID    uint64
		Code  string
		Price uint
	}

	view, err := db.Table("products").
		MapTo[productView]().
		Where("code", "A006").
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if view == nil || view.Code != "A006" || view.Price != 10 || view.ID == 0 {
		t.Fatalf("unexpected table mapped view %#v", view)
	}

	rawViews, err := db.Raw("select id, code, price from products order by id").
		MapTo[productView]().
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rawViews) != 2 || rawViews[1].Code != "A007" {
		t.Fatalf("unexpected raw mapped views %#v", rawViews)
	}
}

func TestSQLiteCustomMapperDisablesStructDirectScan(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Factory: integrationCustomMapperFactory{},
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := db.Raw(`
		create table products (
			id integer primary key autoincrement,
			code text not null unique,
			price integer not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx); err != nil {
		t.Fatal(err)
	}
	if err := db.Register(integrationProduct{}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "CM001", "price": 10}); err != nil {
		t.Fatal(err)
	}

	model, err := db.Use[integrationProduct]().First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if model == nil || model.Code != "mapped-model" || model.Price != 777 {
		t.Fatalf("expected custom model mapper, got %#v", model)
	}

	tableDTO, err := db.Table("products").MapTo[integrationMappedDTO]().First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tableDTO == nil || tableDTO.Code != "mapped-dto" || tableDTO.Price != 888 {
		t.Fatalf("expected custom table DTO mapper, got %#v", tableDTO)
	}

	rawDTO, err := db.Raw("select code, price from products").MapTo[integrationMappedDTO]().First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rawDTO == nil || rawDTO.Code != "mapped-dto" || rawDTO.Price != 888 {
		t.Fatalf("expected custom raw DTO mapper, got %#v", rawDTO)
	}
}

func TestSQLiteCreateFallbackWithoutReturning(t *testing.T) {
	db, ctx := openSQLiteTestDBWithDriver(t, sqlite.Open(":memory:", sqlite.DisableReturning()))

	tableRow, err := db.Table("products").Create(ctx, oro.Map{
		"code":  "A008",
		"price": 80,
	})
	if err != nil {
		t.Fatal(err)
	}
	if tableRow["id"] == nil || tableRow["code"] != "A008" {
		t.Fatalf("unexpected table fallback row %#v", tableRow)
	}

	model, err := db.Use[integrationProduct]().Create(ctx, &integrationProduct{
		Code:  "A009",
		Price: 90,
	})
	if err != nil {
		t.Fatal(err)
	}
	if model.ID == 0 || model.Code != "A009" || model.Price != 90 {
		t.Fatalf("unexpected model fallback row %#v", model)
	}
}

func TestSQLiteCreateMany(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	tableResult, err := db.Table("products").CreateMany(ctx, []oro.Map{
		{"code": "A010", "price": 10},
		{"code": "A011", "price": 11},
	})
	if err != nil {
		t.Fatal(err)
	}
	tableIDs, err := tableResult.IDs[uint64]()
	if err != nil {
		t.Fatal(err)
	}
	if tableResult.RowsAffected != 2 || len(tableIDs) != 2 || tableIDs[0] == 0 || tableIDs[1] == 0 {
		t.Fatalf("unexpected table create many result %#v ids=%#v", tableResult, tableIDs)
	}

	tableRows, err := db.Table("products").CreateManyResult(ctx, []oro.Map{
		{"code": "A010R", "price": 10},
		{"code": "A011R", "price": 11},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(tableRows) != 2 || tableRows[0]["code"] != "A010R" || tableRows[1]["code"] != "A011R" || tableRows[0]["id"] == nil || tableRows[1]["id"] == nil {
		t.Fatalf("unexpected table result rows %#v", tableRows)
	}

	models := []*integrationProduct{
		{Code: "A012", Price: 12},
		{Code: "A013", Price: 13},
	}
	result, err := db.Use[integrationProduct]().CreateMany(ctx, models)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := result.IDs[uint64]()
	if err != nil {
		t.Fatal(err)
	}
	if result.RowsAffected != 2 || len(ids) != 2 || ids[0] == 0 || ids[1] == 0 || models[0].ID == 0 || models[1].ID == 0 {
		t.Fatalf("unexpected model create many result %#v ids=%#v models=%#v", result, ids, models)
	}
	if models[0].CreatedAt.IsZero() || models[1].UpdatedAt.IsZero() {
		t.Fatalf("expected timestamps in model rows %#v", models)
	}

	products, err := db.Use[integrationProduct]().CreateManyResult(ctx, []*integrationProduct{
		{Code: "A012R", Price: 12},
		{Code: "A013R", Price: 13},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 2 || products[0].ID == 0 || products[1].ID == 0 || products[0].Code != "A012R" || products[1].Code != "A013R" {
		t.Fatalf("unexpected model result rows %#v", products)
	}

	type productView struct {
		ID   uint64
		Code string
	}
	views, err := db.Table("products").
		MapTo[productView]().
		CreateManyResult(ctx, []oro.Map{
			{"code": "A014", "price": 14},
			{"code": "A015", "price": 15},
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(views) != 2 || views[0].ID == 0 || views[1].Code != "A015" {
		t.Fatalf("unexpected mapped create many rows %#v", views)
	}

	chunked, err := db.Use[integrationProduct]().CreateManyResult(ctx, []*integrationProduct{
		{Code: "A016", Price: 16},
		{Code: "A017", Price: 17},
	}, oro.BatchSize(1))
	if err != nil {
		t.Fatal(err)
	}
	if len(chunked) != 2 || chunked[0].Code != "A016" || chunked[1].Code != "A017" {
		t.Fatalf("unexpected chunked create many rows %#v", chunked)
	}
}

func TestSQLiteCreateManyFallbackWithoutReturning(t *testing.T) {
	db, ctx := openSQLiteTestDBWithDriver(t, sqlite.Open(":memory:", sqlite.DisableReturning()))

	tableResult, err := db.Table("products").CreateMany(ctx, []oro.Map{
		{"code": "A018", "price": 18},
		{"code": "A019", "price": 19},
	})
	if err != nil {
		t.Fatal(err)
	}
	tableIDs, err := tableResult.IDs[uint64]()
	if err != nil {
		t.Fatal(err)
	}
	if tableResult.RowsAffected != 2 || len(tableIDs) != 2 || tableIDs[0] == 0 || tableIDs[1] == 0 {
		t.Fatalf("unexpected table fallback result %#v ids=%#v", tableResult, tableIDs)
	}

	products, err := db.Use[integrationProduct]().CreateManyResult(ctx, []*integrationProduct{
		{Code: "A020", Price: 20},
		{Code: "A021", Price: 21},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 2 || products[0].ID == 0 || products[1].ID == 0 || products[0].Code != "A020" || products[1].Code != "A021" {
		t.Fatalf("unexpected model fallback rows %#v", products)
	}

	mixedRows, err := db.Table("products").CreateManyResult(ctx, []oro.Map{
		{"code": "A022", "price": 22},
		{"id": uint64(100), "code": "A023", "price": 23},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mixedRows) != 2 || mixedRows[0]["code"] != "A022" || mixedRows[1]["code"] != "A023" {
		t.Fatalf("unexpected mixed-shape fallback rows %#v", mixedRows)
	}
}

func TestSQLiteTableAndModelUpsert(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	row, err := db.Table("products").Upsert(ctx, oro.Map{
		"code":  "U001",
		"price": 10,
	}, oro.ConflictBy("code").Update("price"))
	if err != nil {
		t.Fatal(err)
	}
	if row["id"] == nil || row["code"] != "U001" || row["price"] != int64(10) {
		t.Fatalf("unexpected inserted upsert row %#v", row)
	}

	row, err = db.Table("products").Upsert(ctx, oro.Map{
		"code":  "U001",
		"price": 20,
	}, oro.ConflictBy("code").Update("price"))
	if err != nil {
		t.Fatal(err)
	}
	if row["code"] != "U001" || row["price"] != int64(20) {
		t.Fatalf("unexpected updated upsert row %#v", row)
	}

	product, err := db.Use[integrationProduct]().Upsert(ctx, &integrationProduct{
		Code:  "U002",
		Price: 30,
	}, oro.ConflictBy("Code").Update("Price"))
	if err != nil {
		t.Fatal(err)
	}
	if product.ID == 0 || product.Code != "U002" || product.Price != 30 {
		t.Fatalf("unexpected model upsert insert %#v", product)
	}

	product, err = db.Use[integrationProduct]().Upsert(ctx, &integrationProduct{
		Code:  "U002",
		Price: 40,
	}, oro.ConflictBy("Code").Update("Price"))
	if err != nil {
		t.Fatal(err)
	}
	if product.ID == 0 || product.Code != "U002" || product.Price != 40 {
		t.Fatalf("unexpected model upsert update %#v", product)
	}
}

func TestSQLiteTableUpsertManyUsesEachInsertedRow(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Table("products").CreateMany(ctx, []oro.Map{
		{"code": "B001", "price": 1},
		{"code": "B002", "price": 2},
	})
	if err != nil {
		t.Fatal(err)
	}

	affected, err := db.Table("products").UpsertMany(ctx, []oro.Map{
		{"code": "B001", "price": 100},
		{"code": "B002", "price": 200},
		{"code": "B003", "price": 300},
	}, oro.ConflictBy("code").Update("price"))
	if err != nil {
		t.Fatal(err)
	}
	if affected != 3 {
		t.Fatalf("got affected %d, want 3", affected)
	}

	rows, err := db.Table("products").Select("code", "price").OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	wantPrices := map[string]int64{"B001": 100, "B002": 200, "B003": 300}
	for _, row := range rows {
		code, _ := row["code"].(string)
		if want, ok := wantPrices[code]; ok && row["price"] != want {
			t.Fatalf("row %s price=%#v want %d rows=%#v", code, row["price"], want, rows)
		}
	}
}

func TestSQLiteTableUpsertManyEmitsMultiRowSQL(t *testing.T) {
	ctx := context.Background()
	var upsertSQL []string
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		LogLevel: oro.LogLevelDebug,
		Logger: oro.LoggerFunc(func(ctx context.Context, event oro.LogEvent) {
			if strings.Contains(event.SQL, " on conflict ") {
				upsertSQL = append(upsertSQL, event.SQL)
			}
		}),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	_, err = db.Raw(`
		create table products (
			id integer primary key autoincrement,
			code text not null unique,
			price integer not null
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}

	affected, err := db.Table("products").UpsertMany(ctx, []oro.Map{
		{"code": "Q001", "price": 1},
		{"code": "Q002", "price": 2},
	}, oro.ConflictBy("code").Update("price"))
	if err != nil {
		t.Fatal(err)
	}
	if affected != 2 {
		t.Fatalf("got affected %d, want 2", affected)
	}
	if len(upsertSQL) != 1 {
		t.Fatalf("got %d upsert statements, want 1: %#v", len(upsertSQL), upsertSQL)
	}
	if !strings.Contains(upsertSQL[0], "values (?, ?), (?, ?)") {
		t.Fatalf("expected multi-row upsert SQL, got %q", upsertSQL[0])
	}
}

func TestSQLiteModelUpsertManyUpdateAllSkipsCreatedAt(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)
	oldCreated := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	oldUpdated := time.Date(2024, 1, 2, 4, 4, 5, 0, time.UTC)
	newCreated := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	newUpdated := time.Date(2025, 1, 2, 4, 4, 5, 0, time.UTC)

	created, err := db.Use[integrationProduct]().Create(ctx, &integrationProduct{
		Model: oro.Model{CreatedAt: oldCreated, UpdatedAt: oldUpdated},
		Code:  "M001",
		Price: 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	affected, err := db.Use[integrationProduct]().UpsertMany(ctx, []*integrationProduct{{
		Model: oro.Model{CreatedAt: newCreated, UpdatedAt: newUpdated},
		Code:  "M001",
		Price: 99,
	}, {
		Model: oro.Model{CreatedAt: newCreated, UpdatedAt: newUpdated},
		Code:  "M002",
		Price: 88,
	}}, oro.ConflictBy("Code").UpdateAll())
	if err != nil {
		t.Fatal(err)
	}
	if affected != 2 {
		t.Fatalf("got affected %d, want 2", affected)
	}

	found, err := db.Use[integrationProduct]().Where("Code", "M001").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if found.ID != created.ID || found.Code != "M001" || found.Price != 99 {
		t.Fatalf("unexpected updated model %#v", found)
	}
	if !found.CreatedAt.Equal(oldCreated) {
		t.Fatalf("created_at changed: got %s want %s", found.CreatedAt, oldCreated)
	}
	if !found.UpdatedAt.Equal(newUpdated) {
		t.Fatalf("updated_at not refreshed: got %s want %s", found.UpdatedAt, newUpdated)
	}
}

func TestSQLiteTableUpsertManyUpdateMapAndDoNothing(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)
	_, err := db.Table("products").CreateMany(ctx, []oro.Map{
		{"code": "E001", "price": 10},
		{"code": "E002", "price": 20},
	})
	if err != nil {
		t.Fatal(err)
	}

	affected, err := db.Table("products").UpsertMany(ctx, []oro.Map{
		{"code": "E001", "price": 100},
		{"code": "E002", "price": 200},
	}, oro.ConflictBy("code").UpdateMap(oro.Map{"price": oro.Increment(1)}))
	if err != nil {
		t.Fatal(err)
	}
	if affected != 2 {
		t.Fatalf("got affected %d, want 2", affected)
	}
	rows, err := db.Table("products").Select("code", "price").Where(oro.Field("code").In("E001", "E002")).OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["price"] != int64(11) || rows[1]["price"] != int64(21) {
		t.Fatalf("unexpected increment rows %#v", rows)
	}

	affected, err = db.Table("products").UpsertMany(ctx, []oro.Map{
		{"code": "E001", "price": 999},
		{"code": "E003", "price": 30},
	}, oro.ConflictBy("code").DoNothing())
	if err != nil {
		t.Fatal(err)
	}
	if affected != 1 {
		t.Fatalf("got do-nothing affected %d, want 1", affected)
	}
	rows, err = db.Table("products").Select("code", "price").Where(oro.Field("code").In("E001", "E003")).OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rows[0]["price"] != int64(11) || rows[1]["price"] != int64(30) {
		t.Fatalf("unexpected do-nothing rows %#v", rows)
	}
}

func TestSQLiteTableUpsertManyRejectsMixedKeys(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)
	_, err := db.Table("products").UpsertMany(ctx, []oro.Map{
		{"code": "S001", "price": 1},
		{"code": "S002"},
	}, oro.ConflictBy("code").UpdateAll())
	if err == nil {
		t.Fatal("expected mixed-key upsert error")
	}
}

func TestSQLiteTableJoinMapTo(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Raw(`
		create table users (
			id integer primary key autoincrement,
			name text not null,
			status text not null
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Raw(`
		create table orders (
			id integer primary key autoincrement,
			user_id integer not null,
			total integer not null
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.Table("users").Create(ctx, oro.Map{"name": "Alice", "status": "active"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Table("orders").Create(ctx, oro.Map{"user_id": user["id"], "total": 120}); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Table("orders").As("o").
		LeftJoin("users", func(j *oro.Join) {
			j.As("u").
				OnColumn("u.id", "o.user_id").
				Where("u.status", "active")
		}).
		Select("o.id", oro.As("u.name", "user_name"), oro.As("o.total", "total")).
		MapTo[integrationOrderView]().
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].UserName != "Alice" || rows[0].Total != 120 {
		t.Fatalf("unexpected joined rows %#v", rows)
	}
}

func TestSQLiteJoinSubquery(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Raw(`
		create table orders (
			id integer primary key autoincrement,
			user_id integer not null,
			total integer not null
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Table("orders").Create(ctx, oro.Map{"user_id": 1, "total": 120}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Table("orders").Create(ctx, oro.Map{"user_id": 1, "total": 80}); err != nil {
		t.Fatal(err)
	}

	totals := db.Table("orders").
		Select("user_id", oro.Raw("sum(total) as total")).
		GroupBy("user_id")

	rows, err := db.Table("orders").As("o").
		LeftJoin(oro.Query(totals).As("t"), func(j *oro.Join) {
			j.OnColumn("t.user_id", "o.user_id")
		}).
		Select(oro.As("t.total", "total")).
		Limit(1).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["total"] != int64(200) {
		t.Fatalf("unexpected subquery join rows %#v", rows)
	}
}

func TestSQLiteWithRelations(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, statement := range []string{
		`create table articles (
			id integer primary key autoincrement,
			title text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table images (
			id integer primary key autoincrement,
			article_id integer,
			url text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table comments (
			id integer primary key autoincrement,
			article_id integer,
			body text not null,
			status text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table tags (
			id integer primary key autoincrement,
			name text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table article_tags (
			article_id integer not null,
			tag_id integer not null,
			sort integer
		)`,
	} {
		if _, err := db.Raw(statement).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Register(integrationArticle{}, integrationImage{}, integrationComment{}, integrationTag{}); err != nil {
		t.Fatal(err)
	}

	article, err := db.Use[integrationArticle]().Create(ctx, &integrationArticle{Title: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[integrationImage]().Create(ctx, &integrationImage{ArticleID: article.ID, URL: "cover.jpg"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[integrationComment]().Create(ctx, &integrationComment{ArticleID: article.ID, Body: "C1", Status: "approved"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[integrationComment]().Create(ctx, &integrationComment{ArticleID: article.ID, Body: "C2", Status: "pending"}); err != nil {
		t.Fatal(err)
	}
	tag, err := db.Use[integrationTag]().Create(ctx, &integrationTag{Name: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Table("article_tags").Create(ctx, oro.Map{"article_id": article.ID, "tag_id": tag.ID, "sort": 1}); err != nil {
		t.Fatal(err)
	}

	articles, err := db.Use[integrationArticle]().
		With(integrationArticle{}.Cover()).
		With(integrationArticle{}.Tags()).
		With("Comments", func(q *oro.RelationQuery) {
			q.Where("Status", "approved")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(articles) != 1 {
		t.Fatalf("unexpected article count %d", len(articles))
	}
	cover, err := articles[0].Cover().One[integrationImage]()
	if err != nil {
		t.Fatal(err)
	}
	if cover == nil || cover.URL != "cover.jpg" {
		t.Fatalf("unexpected cover %#v", cover)
	}
	comments, err := articles[0].Comments().Many[integrationComment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].Body != "C1" {
		t.Fatalf("unexpected comments %#v", comments)
	}
	tags, err := articles[0].Tags().Many[integrationTag]()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "Go" {
		t.Fatalf("unexpected tags %#v", tags)
	}

	article2, err := db.Use[integrationArticle]().Create(ctx, &integrationArticle{Title: "A2"})
	if err != nil {
		t.Fatal(err)
	}
	orphanImage, err := db.Use[integrationImage]().Create(ctx, &integrationImage{URL: "orphan.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(orphanImage.Article()).Set(ctx, article2); err != nil {
		t.Fatal(err)
	}
	orphanImage, err = db.Use[integrationImage]().Find(ctx, orphanImage.ID)
	if err != nil {
		t.Fatal(err)
	}
	if orphanImage.ArticleID != article2.ID {
		t.Fatalf("expected belongs-to set article id %d, got %#v", article2.ID, orphanImage)
	}
	if err := db.Relation(orphanImage.Article()).Unset(ctx); err != nil {
		t.Fatal(err)
	}
	orphanImage, err = db.Use[integrationImage]().Find(ctx, orphanImage.ID)
	if err != nil {
		t.Fatal(err)
	}
	if orphanImage.ArticleID != 0 {
		t.Fatalf("expected belongs-to unset, got %#v", orphanImage)
	}

	newCover, err := db.Use[integrationImage]().Create(ctx, &integrationImage{URL: "new-cover.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Cover()).Replace(ctx, newCover); err != nil {
		t.Fatal(err)
	}
	coverRows, err := db.Table("images").Where("article_id", article.ID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(coverRows) != 1 || coverRows[0]["url"] != "new-cover.jpg" {
		t.Fatalf("unexpected cover rows after replace %#v", coverRows)
	}
	if err := db.Relation(article.Cover()).Unset(ctx); err != nil {
		t.Fatal(err)
	}
	coverRows, err = db.Table("images").Where("article_id", article.ID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(coverRows) != 0 {
		t.Fatalf("expected no cover after unset, got %#v", coverRows)
	}
	if err := db.Relation(article.Cover()).Set(ctx, newCover); err != nil {
		t.Fatal(err)
	}

	comment3, err := db.Use[integrationComment]().Create(ctx, &integrationComment{Body: "C3", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	comment4, err := db.Use[integrationComment]().Create(ctx, &integrationComment{Body: "C4", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Comments()).AddMany(ctx, []*integrationComment{comment3, comment4}); err != nil {
		t.Fatal(err)
	}
	commentRows, err := db.Table("comments").Where("article_id", article.ID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(commentRows) != 4 {
		t.Fatalf("expected four comments after add many, got %#v", commentRows)
	}
	if err := db.Relation(article.Comments()).Remove(ctx, comment3); err != nil {
		t.Fatal(err)
	}
	comment3, err = db.Use[integrationComment]().Find(ctx, comment3.ID)
	if err != nil {
		t.Fatal(err)
	}
	if comment3.ArticleID != 0 {
		t.Fatalf("expected comment removed from relation, got %#v", comment3)
	}
	if err := db.Relation(article.Comments()).RemoveMany(ctx, []*integrationComment{comment4}); err != nil {
		t.Fatal(err)
	}
	comment4, err = db.Use[integrationComment]().Find(ctx, comment4.ID)
	if err != nil {
		t.Fatal(err)
	}
	if comment4.ArticleID != 0 {
		t.Fatalf("expected comment removed by remove many, got %#v", comment4)
	}

	tag2, err := db.Use[integrationTag]().Create(ctx, &integrationTag{Name: "ORM"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).Attach(ctx, tag2, oro.Map{"Sort": 2}); err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).UpdateThrough(ctx, tag2, oro.Map{"Sort": 3}); err != nil {
		t.Fatal(err)
	}
	through, err := db.Table("article_tags").
		Where("article_id", article.ID).
		Where("tag_id", tag2.ID).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if through == nil || through["sort"] != int64(3) {
		t.Fatalf("unexpected through row %#v", through)
	}
	withAttached, err := db.Use[integrationArticle]().
		With(integrationArticle{}.Tags()).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	attachedTags, err := withAttached.Tags().Many[integrationTag]()
	if err != nil {
		t.Fatal(err)
	}
	if len(attachedTags) != 2 {
		t.Fatalf("unexpected attached tags %#v", attachedTags)
	}
	if err := db.Relation(article.Tags()).Detach(ctx, tag2); err != nil {
		t.Fatal(err)
	}
	afterDetach, err := db.Use[integrationArticle]().
		With(integrationArticle{}.Tags()).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	afterDetachTags, err := afterDetach.Tags().Many[integrationTag]()
	if err != nil {
		t.Fatal(err)
	}
	if len(afterDetachTags) != 1 {
		t.Fatalf("unexpected tags after detach %#v", afterDetachTags)
	}

	tag3, err := db.Use[integrationTag]().Create(ctx, &integrationTag{Name: "SQL"})
	if err != nil {
		t.Fatal(err)
	}
	tag4, err := db.Use[integrationTag]().Create(ctx, &integrationTag{Name: "Builder"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).AttachMany(ctx, []oro.RelationItem[*integrationTag]{
		{Model: tag2, Data: oro.Map{"Sort": 4}},
		{Model: tag3, Data: oro.Map{"Sort": 5}},
	}); err != nil {
		t.Fatal(err)
	}
	attachedCount, err := db.Table("article_tags").Where("article_id", article.ID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if attachedCount != 3 {
		t.Fatalf("expected three attached tags, got %d", attachedCount)
	}
	through, err = db.Table("article_tags").
		Where("article_id", article.ID).
		Where("tag_id", tag3.ID).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if through == nil || through["sort"] != int64(5) {
		t.Fatalf("unexpected batch through row %#v", through)
	}
	if err := db.Relation(article.Tags()).DetachMany(ctx, []*integrationTag{tag2, tag3}); err != nil {
		t.Fatal(err)
	}
	attachedCount, err = db.Table("article_tags").Where("article_id", article.ID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if attachedCount != 1 {
		t.Fatalf("expected one tag after detach many, got %d", attachedCount)
	}
	if err := db.Relation(article.Tags()).SyncWithoutDetach(ctx, []*integrationTag{tag3, tag4}); err != nil {
		t.Fatal(err)
	}
	attachedCount, err = db.Table("article_tags").Where("article_id", article.ID).Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if attachedCount != 3 {
		t.Fatalf("expected three tags after sync without detach, got %d", attachedCount)
	}
	if err := db.Relation(article.Tags()).Sync(ctx, []*integrationTag{tag4}); err != nil {
		t.Fatal(err)
	}
	attachedRows, err := db.Table("article_tags").Where("article_id", article.ID).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(attachedRows) != 1 || attachedRows[0]["tag_id"] != int64(tag4.ID) {
		t.Fatalf("unexpected rows after sync %#v", attachedRows)
	}

	image, err := db.Use[integrationImage]().
		With(integrationImage{}.Article()).
		Where("ID", newCover.ID).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := image.Article().One[integrationArticle]()
	if err != nil {
		t.Fatal(err)
	}
	if parent == nil || parent.Title != "A1" {
		t.Fatalf("unexpected belongs-to article %#v", parent)
	}

	nested, err := db.Use[integrationArticle]().
		With(integrationArticle{}.Comments(), func(q *oro.RelationQuery) {
			q.Where("Status", "approved").
				With(integrationComment{}.Article())
		}).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	nestedComments, err := nested.Comments().Many[integrationComment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedComments) != 1 {
		t.Fatalf("unexpected nested comments %#v", nestedComments)
	}
	nestedParent, err := nestedComments[0].Article().One[integrationArticle]()
	if err != nil {
		t.Fatal(err)
	}
	if nestedParent == nil || nestedParent.Title != "A1" {
		t.Fatalf("unexpected nested parent %#v", nestedParent)
	}

	pathLoaded, err := db.Use[integrationArticle]().
		With("Comments.Article").
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	pathComments, err := pathLoaded.Comments().Many[integrationComment]()
	if err != nil {
		t.Fatal(err)
	}
	pathParent, err := pathComments[0].Article().One[integrationArticle]()
	if err != nil {
		t.Fatal(err)
	}
	if pathParent == nil || pathParent.Title != "A1" {
		t.Fatalf("unexpected path parent %#v", pathParent)
	}

	forCover, err := db.Use[integrationImage]().
		For(article.Cover()).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if forCover == nil || forCover.URL != "new-cover.jpg" {
		t.Fatalf("unexpected for cover %#v", forCover)
	}

	forComments, err := db.Use[integrationComment]().
		For(article.Comments()).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(forComments) != 2 {
		t.Fatalf("unexpected for comments %#v", forComments)
	}

	forArticle, err := db.Use[integrationArticle]().
		For(forCover.Article()).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if forArticle == nil || forArticle.Title != "A1" {
		t.Fatalf("unexpected for article %#v", forArticle)
	}

	forTags, err := db.Use[integrationTag]().
		For(article.Tags()).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(forTags) != 1 || forTags[0].Name != "Builder" {
		t.Fatalf("unexpected for tags %#v", forTags)
	}

	hasApprovedComments, err := db.Use[integrationArticle]().
		WhereHas(integrationArticle{}.Comments(), func(q *oro.RelationQuery) {
			q.Where("Status", "approved")
		}).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasApprovedComments) != 1 || hasApprovedComments[0].Title != "A1" {
		t.Fatalf("unexpected where has comments %#v", hasApprovedComments)
	}

	hasPendingByName, err := db.Use[integrationArticle]().
		WhereHas("Comments", func(q *oro.RelationQuery) {
			q.Where("Status", "pending")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasPendingByName) != 1 || hasPendingByName[0].Title != "A1" {
		t.Fatalf("unexpected where has by name %#v", hasPendingByName)
	}

	hasGoTag, err := db.Use[integrationArticle]().
		WhereHas(integrationArticle{}.Tags(), func(q *oro.RelationQuery) {
			q.Where("Name", "Go")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasGoTag) != 0 {
		t.Fatalf("expected no go tag after sync, got %#v", hasGoTag)
	}

	hasBuilderTag, err := db.Use[integrationArticle]().
		WhereHas(integrationArticle{}.Tags(), func(q *oro.RelationQuery) {
			q.Where("Name", "Builder")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasBuilderTag) != 1 || hasBuilderTag[0].Title != "A1" {
		t.Fatalf("unexpected where has tags %#v", hasBuilderTag)
	}

	withoutApprovedComments, err := db.Use[integrationArticle]().
		WhereDoesntHave(integrationArticle{}.Comments(), func(q *oro.RelationQuery) {
			q.Where("Status", "approved")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(withoutApprovedComments) != 1 || withoutApprovedComments[0].Title != "A2" {
		t.Fatalf("unexpected where doesnt have comments %#v", withoutApprovedComments)
	}

	withTwoComments, err := db.Use[integrationArticle]().
		WhereHas(integrationArticle{}.Comments(), func(q *oro.RelationQuery) {
			q.Count(">=", 2)
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(withTwoComments) != 1 || withTwoComments[0].Title != "A1" {
		t.Fatalf("unexpected where has count %#v", withTwoComments)
	}

	imagesWithArticle, err := db.Use[integrationImage]().
		WhereHas(integrationImage{}.Article(), func(q *oro.RelationQuery) {
			q.Where("Title", "A1")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(imagesWithArticle) != 1 || imagesWithArticle[0].URL != "new-cover.jpg" {
		t.Fatalf("unexpected belongs-to where has %#v", imagesWithArticle)
	}

	nestedWhereHas, err := db.Use[integrationArticle]().
		WhereHas(integrationArticle{}.Comments(), func(q *oro.RelationQuery) {
			q.WhereHas(integrationComment{}.Article(), func(parent *oro.RelationQuery) {
				parent.Where("Title", "A1")
			})
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(nestedWhereHas) != 1 || nestedWhereHas[0].Title != "A1" {
		t.Fatalf("unexpected nested where has %#v", nestedWhereHas)
	}

	if err := db.Register(integrationArticleAggregate{}); err != nil {
		t.Fatal(err)
	}
	aggregated, err := db.Use[integrationArticleAggregate]().
		WithCount(integrationArticleAggregate{}.Comments()).
		WithExists(integrationArticleAggregate{}.Comments(), func(q *oro.RelationQuery) {
			q.Where("Status", "approved")
		}).
		WithSum(integrationArticleAggregate{}.Comments(), "ID").
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(aggregated) != 2 {
		t.Fatalf("unexpected aggregate articles %#v", aggregated)
	}
	if aggregated[0].CommentsCount != 2 || !aggregated[0].CommentsExists || aggregated[0].CommentsIDSum == 0 {
		t.Fatalf("unexpected relation aggregates %#v", aggregated[0])
	}
	if aggregated[1].CommentsCount != 0 || aggregated[1].CommentsExists || aggregated[1].CommentsIDSum != 0 {
		t.Fatalf("unexpected empty relation aggregates %#v", aggregated[1])
	}

	customAggregated, err := db.Use[integrationArticleAggregate]().
		Select(
			"ID",
			"Title",
			oro.CountOf(integrationArticleAggregate{}.Comments()).
				As("approved_comments").
				Filter(func(q *oro.RelationQuery) {
					q.Where("Status", "approved")
				}),
		).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(customAggregated) != 2 || customAggregated[0].ApprovedComments != 1 || customAggregated[1].ApprovedComments != 0 {
		t.Fatalf("unexpected custom relation aggregate %#v", customAggregated)
	}

	serialized, ok := oro.Serialize(nested).(oro.Map)
	if !ok {
		t.Fatalf("unexpected serialized article %#v", serialized)
	}
	if _, ok := serialized["cover_image"]; ok {
		t.Fatalf("unexpected unloaded cover in serialized article %#v", serialized)
	}
	commentsValue, ok := serialized["comments"].([]any)
	if !ok || len(commentsValue) != 1 {
		t.Fatalf("unexpected serialized comments %#v", serialized["comments"])
	}
	commentMap, ok := commentsValue[0].(oro.Map)
	if !ok {
		t.Fatalf("unexpected serialized comment %#v", commentsValue[0])
	}
	if _, ok := commentMap["article"]; !ok {
		t.Fatalf("expected nested article relation, got %#v", commentMap)
	}

	serializedWithCover, ok := oro.Serialize(withAttached).(oro.Map)
	if !ok {
		t.Fatalf("unexpected serialized article with cover %#v", serializedWithCover)
	}
	if _, ok := serializedWithCover["tags"].([]any); !ok {
		t.Fatalf("expected serialized many-to-many tags, got %#v", serializedWithCover)
	}
}

func TestSQLiteDynamicRelations(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, statement := range []string{
		`create table dynamic_articles (
			id integer primary key autoincrement,
			title text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table dynamic_products (
			id integer primary key autoincrement,
			code text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table dynamic_images (
			id integer primary key autoincrement,
			owner_id integer,
			owner_type text,
			url text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table dynamic_tags (
			id integer primary key autoincrement,
			name text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table dynamic_tag_links (
			owner_id integer not null,
			owner_type text not null,
			tag_id integer not null,
			sort integer
		)`,
	} {
		if _, err := db.Raw(statement).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Register(
		integrationDynamicArticle{},
		integrationDynamicProduct{},
		integrationDynamicImage{},
		integrationDynamicTag{},
	); err != nil {
		t.Fatal(err)
	}

	article, err := db.Use[integrationDynamicArticle]().Create(ctx, &integrationDynamicArticle{Title: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	article2, err := db.Use[integrationDynamicArticle]().Create(ctx, &integrationDynamicArticle{Title: "A2"})
	if err != nil {
		t.Fatal(err)
	}
	product, err := db.Use[integrationDynamicProduct]().Create(ctx, &integrationDynamicProduct{Code: "P1"})
	if err != nil {
		t.Fatal(err)
	}
	image1, err := db.Use[integrationDynamicImage]().Create(ctx, &integrationDynamicImage{URL: "a1.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	image2, err := db.Use[integrationDynamicImage]().Create(ctx, &integrationDynamicImage{URL: "p1.jpg"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Images()).Add(ctx, image1); err != nil {
		t.Fatal(err)
	}
	image1, err = db.Use[integrationDynamicImage]().Find(ctx, image1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[integrationDynamicImage]().
		Where("ID", image2.ID).
		Update(ctx, oro.Map{
			"OwnerID":   product.ID,
			"OwnerType": "integrationDynamicProduct",
		}); err != nil {
		t.Fatal(err)
	}

	loaded, err := db.Use[integrationDynamicArticle]().
		With(integrationDynamicArticle{}.Images()).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("unexpected dynamic articles %#v", loaded)
	}
	images, err := loaded[0].Images().Many[integrationDynamicImage]()
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 1 || images[0].URL != "a1.jpg" {
		t.Fatalf("unexpected dynamic images %#v", images)
	}
	emptyImages, err := loaded[1].Images().Many[integrationDynamicImage]()
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyImages) != 0 {
		t.Fatalf("expected no dynamic images, got %#v", emptyImages)
	}
	if loaded[1].ID != article2.ID {
		t.Fatalf("unexpected second dynamic article %#v", loaded[1])
	}

	owner, err := db.Use[integrationDynamicArticle]().
		For(image1.Owner()).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if owner == nil || owner.ID != article.ID {
		t.Fatalf("unexpected dynamic owner %#v", owner)
	}
	imageWithOwner, err := db.Use[integrationDynamicImage]().
		With(integrationDynamicImage{}.Owner()).
		Where("ID", image1.ID).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	preloadedOwner, err := imageWithOwner.Owner().One[integrationDynamicArticle]()
	if err != nil {
		t.Fatal(err)
	}
	if preloadedOwner == nil || preloadedOwner.ID != article.ID {
		t.Fatalf("unexpected preloaded dynamic owner %#v", preloadedOwner)
	}
	wrongOwner, err := db.Use[integrationDynamicArticle]().
		For(image2.Owner()).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if wrongOwner != nil {
		t.Fatalf("expected no owner for mismatched dynamic type, got %#v", wrongOwner)
	}

	tagGo, err := db.Use[integrationDynamicTag]().Create(ctx, &integrationDynamicTag{Name: "Go"})
	if err != nil {
		t.Fatal(err)
	}
	tagSQL, err := db.Use[integrationDynamicTag]().Create(ctx, &integrationDynamicTag{Name: "SQL"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(article.Tags()).Attach(ctx, tagGo, oro.Map{"Sort": 1}); err != nil {
		t.Fatal(err)
	}
	if err := db.Relation(product.Tags()).Attach(ctx, tagSQL, oro.Map{"Sort": 2}); err != nil {
		t.Fatal(err)
	}

	withTags, err := db.Use[integrationDynamicArticle]().
		With(integrationDynamicArticle{}.Tags()).
		Where("ID", article.ID).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := withTags.Tags().Many[integrationDynamicTag]()
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 || tags[0].Name != "Go" {
		t.Fatalf("unexpected dynamic tags %#v", tags)
	}

	forTags, err := db.Use[integrationDynamicTag]().
		For(article.Tags()).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(forTags) != 1 || forTags[0].Name != "Go" {
		t.Fatalf("unexpected for dynamic tags %#v", forTags)
	}

	hasImage, err := db.Use[integrationDynamicArticle]().
		WhereHas(integrationDynamicArticle{}.Images(), func(q *oro.RelationQuery) {
			q.Where("URL", "a1.jpg")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasImage) != 1 || hasImage[0].ID != article.ID {
		t.Fatalf("unexpected where has dynamic image %#v", hasImage)
	}

	hasGoTag, err := db.Use[integrationDynamicArticle]().
		WhereHas(integrationDynamicArticle{}.Tags(), func(q *oro.RelationQuery) {
			q.Where("Name", "Go")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasGoTag) != 1 || hasGoTag[0].ID != article.ID {
		t.Fatalf("unexpected where has dynamic tag %#v", hasGoTag)
	}

	aggregated, err := db.Use[integrationDynamicArticle]().
		WithCount(integrationDynamicArticle{}.Images()).
		WithCount(integrationDynamicArticle{}.Tags()).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(aggregated) != 2 {
		t.Fatalf("unexpected aggregated dynamic articles %#v", aggregated)
	}
	if aggregated[0].ImagesCount != 1 || aggregated[0].TagsCount != 1 {
		t.Fatalf("unexpected dynamic aggregate counts %#v", aggregated[0])
	}
	if aggregated[1].ImagesCount != 0 || aggregated[1].TagsCount != 0 {
		t.Fatalf("unexpected empty dynamic aggregate counts %#v", aggregated[1])
	}

	if err := db.Relation(article.Images()).Remove(ctx, image1); err != nil {
		t.Fatal(err)
	}
	image1, err = db.Use[integrationDynamicImage]().Find(ctx, image1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if image1.OwnerID != 0 || image1.OwnerType != "" {
		t.Fatalf("expected dynamic image relation removed, got %#v", image1)
	}

	if err := db.Relation(article.Tags()).Detach(ctx, tagGo); err != nil {
		t.Fatal(err)
	}
	left, err := db.Table("dynamic_tag_links").
		Where("owner_id", article.ID).
		Where("owner_type", "integrationDynamicArticle").
		Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("expected dynamic tag detached, got %d", left)
	}

}

func TestSQLiteJoinRaw(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Raw(`
		create table users (
			id integer primary key autoincrement,
			name text not null,
			status text not null
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	user, err := db.Table("users").Create(ctx, oro.Map{"name": "Bob", "status": "active"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "JR001", "price": user["id"]}); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Table("products").As("p").
		JoinRaw("left join users u on u.id = p.price and u.status = ?", "active").
		Select(oro.As("u.name", "name")).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["name"] != "Bob" {
		t.Fatalf("unexpected raw join rows %#v", rows)
	}
}

func TestSQLiteFromSubquery(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "SQ001", "price": 120}); err != nil {
		t.Fatal(err)
	}

	expensive := db.Table("products").
		Select("code", "price").
		Where("price", ">=", 100)

	rows, err := db.From(oro.Query(expensive).As("p")).
		Select("p.code", "p.price").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["code"] != "SQ001" {
		t.Fatalf("unexpected from subquery rows %#v", rows)
	}
}

func TestSQLiteSelectWhereAndHavingSubquery(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "SUB001", "price": 10},
		{"code": "SUB002", "price": 10},
		{"code": "SUB003", "price": 30},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	samePriceCount := db.Table("products").As("p2").
		Select(oro.Count("*")).
		WhereColumn("p2.price", "p.price")

	rows, err := db.Table("products").As("p").
		Select("p.code", oro.Query(samePriceCount).As("same_price_count")).
		Where("p.code", "SUB001").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["same_price_count"] != int64(2) {
		t.Fatalf("unexpected select subquery rows %#v", rows)
	}

	priceSubquery := db.Table("products").
		Select("price").
		Where("code", "SUB003")

	matches, err := db.Use[integrationProduct]().
		WhereIn("Price", oro.Query(priceSubquery)).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || matches[0].Code != "SUB003" {
		t.Fatalf("unexpected where in subquery matches %#v", matches)
	}

	existsRows, err := db.Table("products").As("p").
		WhereExists(oro.Query(
			db.Table("products").As("p2").
				Select("id").
				WhereColumn("p2.price", "p.price").
				WhereColumn("p2.code", "!=", "p.code"),
		)).
		OrderBy("p.code").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(existsRows) != 2 {
		t.Fatalf("unexpected exists subquery rows %#v", existsRows)
	}

	scalarMatches, err := db.Use[integrationProduct]().
		Where("Price", ">", oro.Query(db.Use[integrationProduct]().Select(oro.Avg("Price")))).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(scalarMatches) != 1 || scalarMatches[0].Code != "SUB003" {
		t.Fatalf("unexpected scalar subquery matches %#v", scalarMatches)
	}

	groupRows, err := db.Table("products").
		Select("price", oro.Count("*").As("total")).
		GroupBy("price").
		Having("price", ">", oro.Query(db.Table("products").Select(oro.Avg("price")))).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(groupRows) != 1 || groupRows[0]["price"] != int64(30) || groupRows[0]["total"] != int64(1) {
		t.Fatalf("unexpected having subquery rows %#v", groupRows)
	}
}

func TestSQLiteGroupByHaving(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "G001", "price": 10},
		{"code": "G002", "price": 10},
		{"code": "G003", "price": 30},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	type report struct {
		Price uint
		Total int64
	}

	reports, err := db.Table("products").
		Select("price", oro.Raw("count(*) as total")).
		GroupBy("price").
		HavingRaw("count(*) > ?", 1).
		MapTo[report]().
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(reports) != 1 || reports[0].Price != 10 || reports[0].Total != 2 {
		t.Fatalf("unexpected reports %#v", reports)
	}
}

func TestSQLiteCountWithGroupByCountsGroups(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "GC001", "price": 10},
		{"code": "GC002", "price": 10},
		{"code": "GC003", "price": 30},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	tableCount, err := db.Table("products").GroupBy("price").Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tableCount != 2 {
		t.Fatalf("table grouped count = %d, want 2", tableCount)
	}

	modelCount, err := db.Use[integrationProduct]().GroupBy("Price").Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if modelCount != 2 {
		t.Fatalf("model grouped count = %d, want 2", modelCount)
	}
}

func TestSQLiteAggregates(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, row := range []oro.Map{
		{"code": "AG001", "price": 100},
		{"code": "AG002", "price": 200},
		{"code": "AG003", "price": 300},
	} {
		if _, err := db.Table("products").Create(ctx, row); err != nil {
			t.Fatal(err)
		}
	}

	sum, err := db.Table("products").Sum(ctx, "price")
	if err != nil {
		t.Fatal(err)
	}
	if sum != "600" && sum != "600.0" {
		t.Fatalf("unexpected sum %q", sum)
	}

	avg, err := db.Use[integrationProduct]().Avg(ctx, "Price")
	if err != nil {
		t.Fatal(err)
	}
	if avg != "200" && avg != "200.0" {
		t.Fatalf("unexpected avg %q", avg)
	}

	minPrice, err := db.Table("products").Min[int64](ctx, "price")
	if err != nil {
		t.Fatal(err)
	}
	if !minPrice.Valid || minPrice.Value != 100 {
		t.Fatalf("unexpected min %#v", minPrice)
	}

	maxPrice, err := db.Use[integrationProduct]().Max[uint](ctx, "Price")
	if err != nil {
		t.Fatal(err)
	}
	if !maxPrice.Valid || maxPrice.Value != 300 {
		t.Fatalf("unexpected max %#v", maxPrice)
	}

	_, err = db.Table("products").Limit(1).Sum(ctx, "price")
	if !errors.Is(err, oro.ErrInvalidQuery) {
		t.Fatalf("expected invalid query error, got %v", err)
	}
}

func TestSQLiteAggregateExpressions(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	if _, err := db.Table("products").Create(ctx, oro.Map{"code": "AE001", "price": 100}); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Table("products").
		Select(oro.Count("*").As("total"), oro.Sum("price").As("sum_price")).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["total"] != int64(1) {
		t.Fatalf("unexpected aggregate expression rows %#v", rows)
	}
	if rows[0]["sum_price"] != int64(100) {
		t.Fatalf("unexpected sum expression rows %#v", rows)
	}

	if err := db.Register(integrationProductAggregate{}); err != nil {
		t.Fatal(err)
	}
	modelRows, err := db.Use[integrationProductAggregate]().
		Select(oro.Sum("Price").As("total_price")).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(modelRows) != 1 || modelRows[0].TotalPrice != 100 {
		t.Fatalf("unexpected model aggregate rows %#v", modelRows)
	}
}

func TestSQLiteJSONConditions(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	_, err := db.Raw(`
		create table json_products (
			id integer primary key autoincrement,
			code text not null unique,
			meta text,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(integrationJSONProduct{}); err != nil {
		t.Fatal(err)
	}

	if _, err := db.Table("json_products").Create(ctx, oro.Map{
		"code": "J001",
		"meta": `{"vip":true,"profile":{"country":"CN"}}`,
	}); err != nil {
		t.Fatal(err)
	}

	row, err := db.Table("json_products").
		Where(oro.JSON("meta").Path("vip").Eq(true)).
		Where(oro.JSON("meta").Path("profile", "country").Exists()).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row == nil || row["code"] != "J001" {
		t.Fatalf("unexpected json row %#v", row)
	}

	product, err := db.Use[integrationJSONProduct]().
		Where(oro.JSON("Meta").Path("vip").Eq(true)).
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if product == nil || product.Code != "J001" {
		t.Fatalf("unexpected json product %#v", product)
	}

	_, err = db.Table("json_products").
		Where(oro.JSON("meta").Path("profile").Contains(oro.Map{"country": "CN"})).
		First(ctx)
	if !errors.Is(err, oro.ErrUnsupported) {
		t.Fatalf("expected unsupported contains, got %v", err)
	}
}

func TestSQLiteIncrementDecrementAndRawUpdate(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	row, err := db.Table("products").Create(ctx, oro.Map{
		"code":  "E001",
		"price": 10,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := db.Table("products").
		Where("id", row["id"]).
		Update(ctx, oro.Map{"price": oro.Increment(5)})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one updated row, got %d", updated)
	}

	updated, err = db.Use[integrationProduct]().
		Where("Code", "E001").
		Update(ctx, oro.Map{"Price": oro.Decrement(3)})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one model updated row, got %d", updated)
	}

	updated, err = db.Table("products").
		Where("code", "E001").
		Update(ctx, oro.Map{"price": oro.Raw("price * ?", 2)})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one raw updated row, got %d", updated)
	}

	found, err := db.Table("products").Where("code", "E001").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if found["price"] != int64(24) {
		t.Fatalf("unexpected expression update result %#v", found)
	}
}

func TestSQLiteUpdateAndDelete(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	row, err := db.Table("products").Create(ctx, oro.Map{
		"code":  "A016",
		"price": 16,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := db.Table("products").
		Where("id", row["id"]).
		Update(ctx, oro.Map{"price": 160})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one updated row, got %d", updated)
	}

	found, err := db.Table("products").Where("id", row["id"]).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if found["price"] != int64(160) {
		t.Fatalf("unexpected updated row %#v", found)
	}

	deleted, err := db.Table("products").Where("id", row["id"]).Delete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected one deleted row, got %d", deleted)
	}

	missing, err := db.Table("products").Where("id", row["id"]).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("expected deleted row missing, got %#v", missing)
	}
}

func TestSQLiteUpdateAndDeleteRequireWhere(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	if _, err := db.Table("products").Update(ctx, oro.Map{"price": 1}); err == nil {
		t.Fatal("expected unsafe table update error")
	}
	if _, err := db.Table("products").Delete(ctx); err == nil {
		t.Fatal("expected unsafe table delete error")
	}
	if _, err := db.Use[integrationProduct]().Update(ctx, oro.Map{"Price": 1}); err == nil {
		t.Fatal("expected unsafe model update error")
	}
	if _, err := db.Use[integrationProduct]().Delete(ctx); err == nil {
		t.Fatal("expected unsafe model delete error")
	}
}

func TestSQLiteModelUpdateSoftDeleteRestoreAndForceDelete(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	product, err := db.Use[integrationProduct]().Create(ctx, &integrationProduct{
		Code:  "A017",
		Price: 17,
	})
	if err != nil {
		t.Fatal(err)
	}

	updated, err := db.Use[integrationProduct]().
		Where("ID", product.ID).
		Update(ctx, oro.Map{"Price": 170})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one updated row, got %d", updated)
	}

	found, err := db.Use[integrationProduct]().Find(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Price != 170 || !found.UpdatedAt.After(product.UpdatedAt) {
		t.Fatalf("unexpected updated model %#v", found)
	}

	deleted, err := db.Use[integrationProduct]().
		Where("ID", product.ID).
		Delete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected one soft deleted row, got %d", deleted)
	}

	hidden, err := db.Use[integrationProduct]().Find(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if hidden != nil {
		t.Fatalf("expected soft deleted row hidden, got %#v", hidden)
	}

	withDeleted, err := db.Use[integrationProduct]().
		WithDeleted().
		Find(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if withDeleted == nil || !withDeleted.DeletedAt.Valid {
		t.Fatalf("expected soft deleted row visible with deleted, got %#v", withDeleted)
	}

	onlyDeletedTotal, err := db.Use[integrationProduct]().OnlyDeleted().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if onlyDeletedTotal != 1 {
		t.Fatalf("expected one only deleted row, got %d", onlyDeletedTotal)
	}

	restored, err := db.Use[integrationProduct]().
		WithDeleted().
		Where("ID", product.ID).
		Restore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if restored != 1 {
		t.Fatalf("expected one restored row, got %d", restored)
	}

	active, err := db.Use[integrationProduct]().Find(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if active == nil || active.DeletedAt.Valid {
		t.Fatalf("expected restored active row, got %#v", active)
	}

	forceDeleted, err := db.Use[integrationProduct]().
		Where("ID", product.ID).
		ForceDelete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if forceDeleted != 1 {
		t.Fatalf("expected one force deleted row, got %d", forceDeleted)
	}

	gone, err := db.Use[integrationProduct]().WithDeleted().Find(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if gone != nil {
		t.Fatalf("expected force deleted row gone, got %#v", gone)
	}
}

func TestSQLiteModelHooksAndEvents(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)
	integrationHookCalls = nil

	_, err := db.Raw(`
		create table hook_products (
			id integer primary key autoincrement,
			code text not null,
			price integer not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)
	`).Exec(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(integrationHookProduct{}); err != nil {
		t.Fatal(err)
	}

	var events []oro.EventName
	off := db.On(oro.AfterCreate, func(ctx context.Context, event *oro.Event) error {
		if event.ModelName == "integrationHookProduct" {
			events = append(events, event.Name)
		}
		return nil
	})
	defer off()
	db.On(oro.AfterUpdate, func(ctx context.Context, event *oro.Event) error {
		if event.ModelName == "integrationHookProduct" && event.RowsAffected == 1 {
			events = append(events, event.Name)
		}
		return nil
	})
	db.On(oro.AfterFind, func(ctx context.Context, event *oro.Event) error {
		if event.ModelName == "integrationHookProduct" {
			events = append(events, event.Name)
		}
		return nil
	})

	product, err := db.Use[integrationHookProduct]().Create(ctx, &integrationHookProduct{
		Code:  "A",
		Price: 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if product.Code != "hook-A" || product.ID == 0 {
		t.Fatalf("unexpected created product %#v", product)
	}
	if len(integrationHookCalls) != 2 || integrationHookCalls[0] != "before_create" || integrationHookCalls[1] != "after_create" {
		t.Fatalf("unexpected create hook calls %#v", integrationHookCalls)
	}
	if len(events) != 1 || events[0] != oro.AfterCreate {
		t.Fatalf("unexpected after create events %#v", events)
	}

	updated, err := db.Use[integrationHookProduct]().
		Where("ID", product.ID).
		Update(ctx, oro.Map{"Price": uint(12)})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one updated row, got %d", updated)
	}
	found, err := db.Use[integrationHookProduct]().Find(ctx, product.ID)
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found.Price != 120 {
		t.Fatalf("expected hook-mutated price, got %#v", found)
	}
	if len(events) < 3 || events[1] != oro.AfterUpdate || events[2] != oro.AfterFind {
		t.Fatalf("unexpected update/find events %#v", events)
	}

	deleted, err := db.Use[integrationHookProduct]().
		Where("ID", product.ID).
		Delete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected one soft delete, got %d", deleted)
	}
	restored, err := db.Use[integrationHookProduct]().
		WithDeleted().
		Where("ID", product.ID).
		Restore(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if restored != 1 {
		t.Fatalf("expected one restore, got %d", restored)
	}
	if !slices.Contains(integrationHookCalls, "before_delete") || !slices.Contains(integrationHookCalls, "after_restore") {
		t.Fatalf("expected delete and restore hooks, got %#v", integrationHookCalls)
	}

	skipped, err := db.Use[integrationHookProduct]().
		SkipHooks().
		SkipEvents().
		Create(ctx, &integrationHookProduct{Code: "B", Price: 20})
	if err != nil {
		t.Fatal(err)
	}
	if skipped.Code != "B" {
		t.Fatalf("expected skipped hooks to keep code, got %#v", skipped)
	}

	_, err = db.Use[integrationHookProduct]().Create(ctx, &integrationHookProduct{Code: "ERR", Price: 1})
	if !errors.Is(err, oro.ErrHook) {
		t.Fatalf("expected hook error, got %v", err)
	}

	var sqlEvents int
	db.On(oro.AfterSQL, func(ctx context.Context, event *oro.Event) error {
		if event.SQL != "" {
			sqlEvents++
		}
		return nil
	})
	if _, err := db.Table("hook_products").Count(ctx); err != nil {
		t.Fatal(err)
	}
	if sqlEvents == 0 {
		t.Fatal("expected SQL event")
	}

	offEventErr := db.On(oro.AfterCreate, func(ctx context.Context, event *oro.Event) error {
		if event.ModelName == "integrationHookProduct" {
			return errors.New("event blocked")
		}
		return nil
	})
	_, err = db.Use[integrationHookProduct]().
		SkipHooks().
		Create(ctx, &integrationHookProduct{Code: "E", Price: 1})
	if !errors.Is(err, oro.ErrEvent) {
		t.Fatalf("expected event error, got %v", err)
	}
	offEventErr()

	offPanicErr := db.On(oro.AfterCreate, func(ctx context.Context, event *oro.Event) error {
		if event.ModelName == "integrationHookProduct" {
			panic("event panic")
		}
		return nil
	})
	_, err = db.Use[integrationHookProduct]().
		SkipHooks().
		Create(ctx, &integrationHookProduct{Code: "P", Price: 1})
	if !errors.Is(err, oro.ErrEvent) {
		t.Fatalf("expected panic to be converted to event error, got %v", err)
	}
	offPanicErr()

	var txEvents []oro.EventName
	db.On(oro.AfterCommit, func(ctx context.Context, event *oro.Event) error {
		txEvents = append(txEvents, event.Name)
		return nil
	})
	db.On(oro.AfterRollback, func(ctx context.Context, event *oro.Event) error {
		txEvents = append(txEvents, event.Name)
		return nil
	})
	if err := db.Transaction(ctx, func(tx *oro.DB) error {
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Transaction(ctx, func(tx *oro.DB) error {
		return errors.New("rollback")
	}); err == nil {
		t.Fatal("expected rollback transaction error")
	}
	if !slices.Contains(txEvents, oro.AfterCommit) || !slices.Contains(txEvents, oro.AfterRollback) {
		t.Fatalf("expected transaction events, got %#v", txEvents)
	}
}

func TestSQLiteScansTime(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)
	now := time.Date(2026, 6, 23, 10, 30, 0, 0, time.UTC)

	row, err := db.Table("products").Create(ctx, oro.Map{
		"code":       "A005",
		"price":      10,
		"created_at": now,
		"updated_at": now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := row["created_at"].(time.Time); !ok {
		t.Fatalf("expected time.Time, got %T %#v", row["created_at"], row["created_at"])
	}
}

func TestSQLiteEagerLoadWhereHas(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	for _, statement := range []string{
		`create table articles (
			id integer primary key autoincrement,
			title text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
		`create table comments (
			id integer primary key autoincrement,
			article_id integer,
			body text not null,
			status text not null,
			created_at datetime,
			updated_at datetime,
			deleted_at datetime
		)`,
	} {
		if _, err := db.Raw(statement).Exec(ctx); err != nil {
			t.Fatal(err)
		}
	}
	// All models referenced by integrationArticle's relations must be registered
	// so relation schemas resolve, even though this test only queries articles
	// and comments.
	if err := db.Register(integrationArticle{}, integrationComment{}, integrationImage{}, integrationTag{}); err != nil {
		t.Fatal(err)
	}

	a1, err := db.Use[integrationArticle]().Create(ctx, &integrationArticle{Title: "A1"})
	if err != nil {
		t.Fatal(err)
	}
	a2, err := db.Use[integrationArticle]().Create(ctx, &integrationArticle{Title: "A2"})
	if err != nil {
		t.Fatal(err)
	}
	for _, comment := range []*integrationComment{
		{ArticleID: a1.ID, Body: "c1", Status: "approved"},
		{ArticleID: a1.ID, Body: "c2", Status: "pending"},
		{ArticleID: a2.ID, Body: "c3", Status: "approved"},
	} {
		if _, err := db.Use[integrationComment]().Create(ctx, comment); err != nil {
			t.Fatal(err)
		}
	}

	// Regression: WhereHas inside an eager-load .With() callback must resolve
	// the deferred relation-filter payload. Previously this failed with
	// "where: oro: unknown field" because the eager-load path converted
	// conditions without resolving relation filters first.
	articles, err := db.Use[integrationArticle]().
		With(integrationArticle{}.Comments(), func(q *oro.RelationQuery) {
			q.Where("Status", "approved").
				WhereHas(integrationComment{}.Article(), func(parent *oro.RelationQuery) {
					parent.Where("Title", "A1")
				})
		}).
		OrderBy("ID").
		Get(ctx)
	if err != nil {
		t.Fatalf("eager WhereHas: %v", err)
	}
	if len(articles) != 2 {
		t.Fatalf("expected 2 articles, got %d", len(articles))
	}

	a1Comments, err := articles[0].Comments().Many[integrationComment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(a1Comments) != 1 || a1Comments[0].Body != "c1" {
		t.Fatalf("A1: expected [c1], got %#v", a1Comments)
	}

	a2Comments, err := articles[1].Comments().Many[integrationComment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(a2Comments) != 0 {
		t.Fatalf("A2: expected no comments, got %#v", a2Comments)
	}
}

func TestSQLiteTableQuerySoftDeleteScope(t *testing.T) {
	db, ctx := openSQLiteTestDB(t)

	// openSQLiteTestDB registers integrationProduct (soft-delete) and creates
	// the products table. Give each row a distinct price so grouped counts are
	// meaningful, then soft-delete one via the model API.
	prices := map[string]uint{"A": 10, "B": 20, "C": 30}
	for code, price := range prices {
		if _, err := db.Use[integrationProduct]().Create(ctx, &integrationProduct{Code: code, Price: price}); err != nil {
			t.Fatal(err)
		}
	}
	deleted, err := db.Use[integrationProduct]().Where("Code", "C").Delete(ctx)
	if err != nil || deleted != 1 {
		t.Fatalf("soft delete: n=%d err=%v", deleted, err)
	}

	// #2: a low-level table query on the same table must exclude soft-deleted rows.
	rows, err := db.Table("products").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("table Get: expected 2 live rows, got %d", len(rows))
	}

	count, err := db.Table("products").Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("table Count: expected 2, got %d", count)
	}

	// #3: grouped count via a table query must be scoped, so the soft-deleted
	// row's distinct price group is not counted (2 live groups, not 3).
	groupCount, err := db.Table("products").Select("price").GroupBy("price").Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if groupCount != 2 {
		t.Fatalf("grouped table Count: expected 2 groups, got %d", groupCount)
	}
}
