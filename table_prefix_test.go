package oro_test

import (
	"context"
	"slices"
	"testing"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	_ "modernc.org/sqlite"
)

type prefixProduct struct {
	oro.Model
	Code  string
	Price uint
	Meta  oro.JSONRaw
}

func (prefixProduct) Define(s *oro.SchemaBuilder) {
	s.Table("prefix_products")
	s.Field("Code").String().Unique()
	s.Field("Price").Uint()
	s.Field("Meta").JSON().Nullable()
}

func (product prefixProduct) Comments() oro.Relation {
	return oro.HasMany(product, "Comments", "prefixComment").
		ForeignKey("ProductID").
		ReferenceKey("ID")
}

func (product prefixProduct) Tags() oro.Relation {
	return oro.ManyToMany(product, "Tags", "prefixTag").
		Through("prefix_product_tags").
		SourceForeignKey("ProductID").
		TargetForeignKey("TagID")
}

type prefixComment struct {
	oro.Model
	ProductID uint64
	Body      string
	Status    string
}

func (prefixComment) Define(s *oro.SchemaBuilder) {
	s.Table("prefix_comments")
	s.Field("ProductID").UnsignedBigInt()
	s.Field("Body").String()
	s.Field("Status").String()
}

type prefixTag struct {
	oro.Model
	Name string
}

func (prefixTag) Define(s *oro.SchemaBuilder) {
	s.Table("prefix_tags")
	s.Field("Name").String()
}

type prefixProductTag struct {
	ProductID uint64
	TagID     uint64
}

func (prefixProductTag) Define(s *oro.SchemaBuilder) {
	s.Table("prefix_product_tags")
	s.Field("ProductID").UnsignedBigInt()
	s.Field("TagID").UnsignedBigInt()
}

func TestSQLiteTablePrefixAppliesToSyncCRUDStreamAndRelations(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		TablePrefix: "tenant_",
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

	if err := db.Register(prefixProduct{}, prefixComment{}, prefixTag{}, prefixProductTag{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	for _, table := range []string{"tenant_prefix_products", "tenant_prefix_comments", "tenant_prefix_tags", "tenant_oro_schema"} {
		row, err := db.Raw("select name from sqlite_master where type = 'table' and name = ?", table).First(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if row == nil {
			t.Fatalf("expected prefixed table %s", table)
		}
	}
	if row, err := db.Raw("select name from sqlite_master where type = 'table' and name = ?", "prefix_products").First(ctx); err != nil {
		t.Fatal(err)
	} else if row != nil {
		t.Fatalf("expected no unprefixed table, got %#v", row)
	}

	product, err := db.Use[prefixProduct]().Create(ctx, &prefixProduct{Code: "P001", Price: 100})
	if err != nil {
		t.Fatal(err)
	}
	if product.ID == 0 {
		t.Fatalf("expected created id, got %#v", product)
	}
	tableRow, err := db.Table("prefix_products").Create(ctx, oro.Map{"code": "P002", "price": 200})
	if err != nil {
		t.Fatal(err)
	}
	if tableRow["id"] == nil {
		t.Fatalf("expected table create id, got %#v", tableRow)
	}
	if _, err := db.Table("tenant_prefix_products").Create(ctx, oro.Map{"code": "P003", "price": 300}); err != nil {
		t.Fatal(err)
	}
	if db.TableName("prefix_products") != "tenant_prefix_products" {
		t.Fatalf("unexpected table name %q", db.TableName("prefix_products"))
	}
	if db.TableName("tenant_prefix_products") != "tenant_prefix_products" {
		t.Fatalf("unexpected existing prefixed table name %q", db.TableName("tenant_prefix_products"))
	}
	if db.TableName("shop.prefix_products") != "shop.tenant_prefix_products" {
		t.Fatalf("unexpected qualified table name %q", db.TableName("shop.prefix_products"))
	}
	rawCount, err := db.Raw("select count(*) as total from " + db.TableName("prefix_products")).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if rawCount["total"] != int64(3) {
		t.Fatalf("unexpected raw count %#v", rawCount)
	}

	qualifiedRows, err := db.Table("prefix_products").
		Where("prefix_products.code", "P001").
		OrderBy("prefix_products.id").
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(qualifiedRows) != 1 || qualifiedRows[0]["code"] != "P001" {
		t.Fatalf("unexpected qualified rows %#v", qualifiedRows)
	}
	if _, err := db.Table("prefix_products").Where("code", "P002").Update(ctx, oro.Map{"meta": oro.JSONRaw(`{"color":"blue"}`)}); err != nil {
		t.Fatal(err)
	}
	jsonRows, err := db.Table("prefix_products").
		Where(oro.JSON("prefix_products.meta").Path("color").Eq("blue")).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(jsonRows) != 1 || jsonRows[0]["code"] != "P002" {
		t.Fatalf("unexpected json rows %#v", jsonRows)
	}

	stream, err := db.Table("prefix_products").OrderBy("id").Stream(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Close()
	var codes []string
	for stream.Next() {
		codes = append(codes, stream.Value()["code"].(string))
	}
	if err := stream.Err(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(codes, []string{"P001", "P002", "P003"}) {
		t.Fatalf("unexpected stream codes %#v", codes)
	}

	comment, err := db.Use[prefixComment]().Create(ctx, &prefixComment{ProductID: product.ID, Body: "ok", Status: "approved"})
	if err != nil {
		t.Fatal(err)
	}
	tag, err := db.Use[prefixTag]().Create(ctx, &prefixTag{Name: "orm"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Table("prefix_product_tags").Create(ctx, oro.Map{
		"product_id": product.ID,
		"tag_id":     tag.ID,
	}); err != nil {
		t.Fatal(err)
	}

	withComments, err := db.Use[prefixProduct]().
		With(prefixProduct{}.Comments()).
		Where("Code", "P001").
		First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	comments, err := withComments.Comments().Many[prefixComment]()
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].ID != comment.ID {
		t.Fatalf("unexpected comments %#v", comments)
	}

	hasComment, err := db.Use[prefixProduct]().
		WhereHas(prefixProduct{}.Comments(), func(q *oro.RelationQuery) {
			q.Where("Status", "approved")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasComment) != 1 || hasComment[0].Code != "P001" {
		t.Fatalf("unexpected where has comments %#v", hasComment)
	}

	hasTag, err := db.Use[prefixProduct]().
		WhereHas(prefixProduct{}.Tags(), func(q *oro.RelationQuery) {
			q.Where("Name", "orm")
		}).
		Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(hasTag) != 1 || hasTag[0].Code != "P001" {
		t.Fatalf("unexpected where has tags %#v", hasTag)
	}
}

func TestTableNameResolverPrefixesQualifiedPhysicalName(t *testing.T) {
	db, err := oro.Open(oro.Config{
		TablePrefix: "tenant_",
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(context.Background()); err != nil {
			t.Fatal(err)
		}
	})

	rows, err := db.Table("tenant.products").Select("tenant.products.id").Get(context.Background())
	if err == nil || rows != nil {
		t.Fatalf("expected sqlite missing table error for tenant-qualified physical table, got rows=%#v err=%v", rows, err)
	}
}
