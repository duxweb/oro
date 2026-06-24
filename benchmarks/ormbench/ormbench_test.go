package ormbench

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	oro "github.com/duxweb/oro"
	oromysql "github.com/duxweb/oro/driver/mysql"
	oropgsql "github.com/duxweb/oro/driver/pgsql"
	orosqlite "github.com/duxweb/oro/driver/sqlite"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/mattn/go-sqlite3"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/schema"
	gormmysql "gorm.io/driver/mysql"
	gormpostgres "gorm.io/driver/postgres"
	gormsqlite "gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"xorm.io/xorm"
)

const seedRows = 1000

type oroBenchProduct struct {
	oro.Model
	Code  string
	Price uint
}

func (oroBenchProduct) Define(s *oro.SchemaBuilder) {
	s.Table("products")
	s.Field("Code").String().Unique()
	s.Field("Price").Uint()
}

type gormBenchProduct struct {
	ID        uint64 `gorm:"primaryKey"`
	Code      string `gorm:"size:191;uniqueIndex"`
	Price     uint
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

func (gormBenchProduct) TableName() string {
	return "products"
}

type xormBenchProduct struct {
	ID        uint64    `xorm:"'id' pk autoincr"`
	Code      string    `xorm:"'code' varchar(191) unique"`
	Price     uint      `xorm:"'price'"`
	CreatedAt time.Time `xorm:"'created_at'"`
	UpdatedAt time.Time `xorm:"'updated_at'"`
	DeletedAt time.Time `xorm:"'deleted_at' deleted"`
}

func (xormBenchProduct) TableName() string {
	return "products"
}

type bunBenchProduct struct {
	bun.BaseModel `bun:"table:products,alias:p"`
	ID            uint64 `bun:",pk,autoincrement"`
	Code          string `bun:"type:varchar(191),unique"`
	Price         uint
	CreatedAt     time.Time `bun:",nullzero"`
	UpdatedAt     time.Time `bun:",nullzero"`
	DeletedAt     time.Time `bun:",soft_delete,nullzero"`
}

type benchCase struct {
	name string
	run  func(*testing.B)
}

func BenchmarkCreate(b *testing.B) {
	runCases(b, []benchCase{
		{name: "Oro", run: benchOroCreate},
		{name: "GORM", run: benchGORMCreate},
		{name: "XORM", run: benchXORMCreate},
		{name: "Bun", run: benchBunCreate},
	})
}

func BenchmarkCreateMany100(b *testing.B) {
	runCases(b, []benchCase{
		{name: "Oro", run: benchOroCreateMany100},
		{name: "GORM", run: benchGORMCreateMany100},
		{name: "XORM", run: benchXORMCreateMany100},
		{name: "Bun", run: benchBunCreateMany100},
	})
}

func BenchmarkFirstByCode(b *testing.B) {
	runCases(b, []benchCase{
		{name: "Oro", run: benchOroFirstByCode},
		{name: "GORM", run: benchGORMFirstByCode},
		{name: "XORM", run: benchXORMFirstByCode},
		{name: "Bun", run: benchBunFirstByCode},
	})
}

func BenchmarkWhereList(b *testing.B) {
	runCases(b, []benchCase{
		{name: "Oro", run: benchOroWhereList},
		{name: "GORM", run: benchGORMWhereList},
		{name: "XORM", run: benchXORMWhereList},
		{name: "Bun", run: benchBunWhereList},
	})
}

func BenchmarkUpdateByCode(b *testing.B) {
	runCases(b, []benchCase{
		{name: "Oro", run: benchOroUpdateByCode},
		{name: "GORM", run: benchGORMUpdateByCode},
		{name: "XORM", run: benchXORMUpdateByCode},
		{name: "Bun", run: benchBunUpdateByCode},
	})
}

func BenchmarkDeleteByCode(b *testing.B) {
	runCases(b, []benchCase{
		{name: "Oro", run: benchOroDeleteByCode},
		{name: "GORM", run: benchGORMDeleteByCode},
		{name: "XORM", run: benchXORMDeleteByCode},
		{name: "Bun", run: benchBunDeleteByCode},
	})
}

func runCases(b *testing.B, cases []benchCase) {
	for _, item := range cases {
		b.Run(item.name, item.run)
	}
}

func benchOroCreate(b *testing.B) {
	ctx := context.Background()
	db := openOroBenchDB(b, ctx)
	defer closeOroBenchDB(b, ctx, db)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, err := db.Use[oroBenchProduct]().Create(ctx, &oroBenchProduct{
			Code:  createCode(index),
			Price: uint(index % 1000),
		})
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchOroCreateMany100(b *testing.B) {
	ctx := context.Background()
	db := openOroBenchDB(b, ctx)
	defer closeOroBenchDB(b, ctx, db)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		products := makeOroBenchProducts(createManyCode(index), 100)
		result, err := db.Use[oroBenchProduct]().CreateMany(ctx, products)
		if err != nil {
			b.Fatal(err)
		}
		if result.RowsAffected != int64(len(products)) {
			b.Fatalf("expected %d created rows, got %d", len(products), result.RowsAffected)
		}
		if result.IDCount() != len(products) {
			b.Fatalf("expected %d ids, got %d", len(products), result.IDCount())
		}
	}
}

func benchOroFirstByCode(b *testing.B) {
	ctx := context.Background()
	db := openOroBenchDB(b, ctx)
	defer closeOroBenchDB(b, ctx, db)
	seedOroProducts(b, ctx, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		product, err := db.Use[oroBenchProduct]().Where("Code", seedCode(index)).First(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if product == nil {
			b.Fatal("expected product")
		}
	}
}

func benchOroWhereList(b *testing.B) {
	ctx := context.Background()
	db := openOroBenchDB(b, ctx)
	defer closeOroBenchDB(b, ctx, db)
	seedOroProducts(b, ctx, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		products, err := db.Use[oroBenchProduct]().
			Where("Price", ">=", uint(index%100)).
			OrderBy("ID").
			Limit(20).
			Get(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if len(products) == 0 {
			b.Fatal("expected products")
		}
	}
}

func benchOroUpdateByCode(b *testing.B) {
	ctx := context.Background()
	db := openOroBenchDB(b, ctx)
	defer closeOroBenchDB(b, ctx, db)
	seedOroProducts(b, ctx, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		affected, err := db.Use[oroBenchProduct]().
			Where("Code", seedCode(index)).
			Update(ctx, oro.Map{"Price": uint(index)})
		if err != nil {
			b.Fatal(err)
		}
		if affected != 1 {
			b.Fatalf("expected 1 affected row, got %d", affected)
		}
	}
}

func benchOroDeleteByCode(b *testing.B) {
	ctx := context.Background()
	db := openOroBenchDB(b, ctx)
	defer closeOroBenchDB(b, ctx, db)
	seedOroProductsWithCode(b, ctx, db, b.N, deleteCode)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		affected, err := db.Use[oroBenchProduct]().
			Where("Code", deleteCode(index)).
			ForceDelete(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if affected != 1 {
			b.Fatalf("expected 1 affected row, got %d", affected)
		}
	}
}

func benchGORMCreate(b *testing.B) {
	db := openGORMBenchDB(b)
	defer closeGORMBenchDB(b, db)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		err := db.Create(newGORMBenchProduct(createCode(index), uint(index%1000))).Error
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchGORMCreateMany100(b *testing.B) {
	db := openGORMBenchDB(b)
	defer closeGORMBenchDB(b, db)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		products := makeGORMBenchProducts(createManyCode(index), 100)
		if err := db.CreateInBatches(products, len(products)).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func benchGORMFirstByCode(b *testing.B) {
	db := openGORMBenchDB(b)
	defer closeGORMBenchDB(b, db)
	seedGORMProducts(b, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		var product gormBenchProduct
		err := db.Where("code = ?", seedCode(index)).First(&product).Error
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchGORMWhereList(b *testing.B) {
	db := openGORMBenchDB(b)
	defer closeGORMBenchDB(b, db)
	seedGORMProducts(b, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		var products []gormBenchProduct
		err := db.Where("price >= ?", uint(index%100)).Order("id").Limit(20).Find(&products).Error
		if err != nil {
			b.Fatal(err)
		}
		if len(products) == 0 {
			b.Fatal("expected products")
		}
	}
}

func benchGORMUpdateByCode(b *testing.B) {
	db := openGORMBenchDB(b)
	defer closeGORMBenchDB(b, db)
	seedGORMProducts(b, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result := db.Model(&gormBenchProduct{}).Where("code = ?", seedCode(index)).Update("price", uint(index))
		if result.Error != nil {
			b.Fatal(result.Error)
		}
		if result.RowsAffected != 1 {
			b.Fatalf("expected 1 affected row, got %d", result.RowsAffected)
		}
	}
}

func benchGORMDeleteByCode(b *testing.B) {
	db := openGORMBenchDB(b)
	defer closeGORMBenchDB(b, db)
	seedGORMProductsWithCode(b, db, b.N, deleteCode)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result := db.Where("code = ?", deleteCode(index)).Delete(&gormBenchProduct{})
		if result.Error != nil {
			b.Fatal(result.Error)
		}
		if result.RowsAffected != 1 {
			b.Fatalf("expected 1 affected row, got %d", result.RowsAffected)
		}
	}
}

func benchXORMCreate(b *testing.B) {
	engine := openXORMBenchDB(b)
	defer closeXORMBenchDB(b, engine)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, err := engine.Insert(newXORMBenchProduct(createCode(index), uint(index%1000)))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchXORMCreateMany100(b *testing.B) {
	engine := openXORMBenchDB(b)
	defer closeXORMBenchDB(b, engine)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		products := makeXORMBenchProducts(createManyCode(index), 100)
		affected, err := engine.Insert(&products)
		if err != nil {
			b.Fatal(err)
		}
		if affected != int64(len(products)) {
			b.Fatalf("expected %d affected rows, got %d", len(products), affected)
		}
	}
}

func benchXORMFirstByCode(b *testing.B) {
	engine := openXORMBenchDB(b)
	defer closeXORMBenchDB(b, engine)
	seedXORMProducts(b, engine, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		var product xormBenchProduct
		ok, err := engine.Where("code = ?", seedCode(index)).Get(&product)
		if err != nil {
			b.Fatal(err)
		}
		if !ok {
			b.Fatal("expected product")
		}
	}
}

func benchXORMWhereList(b *testing.B) {
	engine := openXORMBenchDB(b)
	defer closeXORMBenchDB(b, engine)
	seedXORMProducts(b, engine, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		var products []xormBenchProduct
		err := engine.Where("price >= ?", uint(index%100)).OrderBy("id").Limit(20).Find(&products)
		if err != nil {
			b.Fatal(err)
		}
		if len(products) == 0 {
			b.Fatal("expected products")
		}
	}
}

func benchXORMUpdateByCode(b *testing.B) {
	engine := openXORMBenchDB(b)
	defer closeXORMBenchDB(b, engine)
	seedXORMProducts(b, engine, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		affected, err := engine.Table("products").
			Where("code = ?", seedCode(index)).
			Update(map[string]any{"price": uint(index)})
		if err != nil {
			b.Fatal(err)
		}
		if affected != 1 {
			b.Fatalf("expected 1 affected row, got %d", affected)
		}
	}
}

func benchXORMDeleteByCode(b *testing.B) {
	engine := openXORMBenchDB(b)
	defer closeXORMBenchDB(b, engine)
	seedXORMProductsWithCode(b, engine, b.N, deleteCode)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		affected, err := engine.Where("code = ?", deleteCode(index)).Delete(&xormBenchProduct{})
		if err != nil {
			b.Fatal(err)
		}
		if affected != 1 {
			b.Fatalf("expected 1 affected row, got %d", affected)
		}
	}
}

func benchBunCreate(b *testing.B) {
	ctx := context.Background()
	db := openBunBenchDB(b, ctx)
	defer closeBunBenchDB(b, db)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_, err := db.NewInsert().Model(newBunBenchProduct(createCode(index), uint(index%1000))).Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchBunCreateMany100(b *testing.B) {
	ctx := context.Background()
	db := openBunBenchDB(b, ctx)
	defer closeBunBenchDB(b, db)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		products := makeBunBenchProducts(createManyCode(index), 100)
		result, err := db.NewInsert().Model(&products).Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if affected, err := result.RowsAffected(); err != nil {
			b.Fatal(err)
		} else if affected != int64(len(products)) {
			b.Fatalf("expected %d affected rows, got %d", len(products), affected)
		}
	}
}

func benchBunFirstByCode(b *testing.B) {
	ctx := context.Background()
	db := openBunBenchDB(b, ctx)
	defer closeBunBenchDB(b, db)
	seedBunProducts(b, ctx, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		var product bunBenchProduct
		err := db.NewSelect().Model(&product).Where("code = ?", seedCode(index)).Limit(1).Scan(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchBunWhereList(b *testing.B) {
	ctx := context.Background()
	db := openBunBenchDB(b, ctx)
	defer closeBunBenchDB(b, db)
	seedBunProducts(b, ctx, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		var products []bunBenchProduct
		err := db.NewSelect().
			Model(&products).
			Where("price >= ?", uint(index%100)).
			Order("id").
			Limit(20).
			Scan(ctx)
		if err != nil {
			b.Fatal(err)
		}
		if len(products) == 0 {
			b.Fatal("expected products")
		}
	}
}

func benchBunUpdateByCode(b *testing.B) {
	ctx := context.Background()
	db := openBunBenchDB(b, ctx)
	defer closeBunBenchDB(b, db)
	seedBunProducts(b, ctx, db, seedRows)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := db.NewUpdate().
			Model((*bunBenchProduct)(nil)).
			Set("price = ?", uint(index)).
			Where("code = ?", seedCode(index)).
			Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			b.Fatal(err)
		}
		if affected != 1 {
			b.Fatalf("expected 1 affected row, got %d", affected)
		}
	}
}

func benchBunDeleteByCode(b *testing.B) {
	ctx := context.Background()
	db := openBunBenchDB(b, ctx)
	defer closeBunBenchDB(b, db)
	seedBunProductsWithCode(b, ctx, db, b.N, deleteCode)

	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		result, err := db.NewDelete().
			Model((*bunBenchProduct)(nil)).
			Where("code = ?", deleteCode(index)).
			Exec(ctx)
		if err != nil {
			b.Fatal(err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			b.Fatal(err)
		}
		if affected != 1 {
			b.Fatalf("expected 1 affected row, got %d", affected)
		}
	}
}

func openOroBenchDB(b *testing.B, ctx context.Context) *oro.DB {
	b.Helper()
	config := currentBenchConfig()
	db, err := oro.Open(oro.Config{
		SkipDefaultTransaction: true,
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: config.oroDriver()},
		},
	})
	if err != nil {
		b.Fatal(err)
	}
	resetOroBenchTables(b, ctx, db)
	if err := db.Register(oroBenchProduct{}); err != nil {
		b.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		b.Fatal(err)
	}
	return db
}

func closeOroBenchDB(b *testing.B, ctx context.Context, db *oro.DB) {
	b.Helper()
	if err := db.Close(ctx); err != nil {
		b.Fatal(err)
	}
}

func seedOroProducts(b *testing.B, ctx context.Context, db *oro.DB, count int) {
	b.Helper()
	seedOroProductsWithCode(b, ctx, db, count, seedCode)
}

func seedOroProductsWithCode(b *testing.B, ctx context.Context, db *oro.DB, count int, code func(int) string) {
	b.Helper()
	products := make([]*oroBenchProduct, 0, count)
	for index := 0; index < count; index++ {
		products = append(products, &oroBenchProduct{Code: code(index), Price: uint(index % 1000)})
	}
	if _, err := db.Use[oroBenchProduct]().CreateMany(ctx, products); err != nil {
		b.Fatal(err)
	}
}

func openGORMBenchDB(b *testing.B) *gorm.DB {
	b.Helper()
	config := currentBenchConfig()
	db, err := gorm.Open(config.gormDialector(), &gorm.Config{SkipDefaultTransaction: true})
	if err != nil {
		b.Fatal(err)
	}
	resetGORMBenchTables(b, db)
	if err := db.AutoMigrate(&gormBenchProduct{}); err != nil {
		b.Fatal(err)
	}
	return db
}

func closeGORMBenchDB(b *testing.B, db *gorm.DB) {
	b.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		b.Fatal(err)
	}
}

func seedGORMProducts(b *testing.B, db *gorm.DB, count int) {
	b.Helper()
	seedGORMProductsWithCode(b, db, count, seedCode)
}

func seedGORMProductsWithCode(b *testing.B, db *gorm.DB, count int, code func(int) string) {
	b.Helper()
	products := make([]gormBenchProduct, 0, count)
	for index := 0; index < count; index++ {
		products = append(products, *newGORMBenchProduct(code(index), uint(index%1000)))
	}
	if err := db.CreateInBatches(products, 500).Error; err != nil {
		b.Fatal(err)
	}
}

func openXORMBenchDB(b *testing.B) *xorm.Engine {
	b.Helper()
	config := currentBenchConfig()
	engine, err := xorm.NewEngine(config.xormDriverName(), config.xormDSN())
	if err != nil {
		b.Fatal(err)
	}
	engine.ShowSQL(false)
	resetXORMBenchTables(b, engine)
	if err := engine.Sync(new(xormBenchProduct)); err != nil {
		b.Fatal(err)
	}
	return engine
}

func closeXORMBenchDB(b *testing.B, engine *xorm.Engine) {
	b.Helper()
	if err := engine.Close(); err != nil {
		b.Fatal(err)
	}
}

func seedXORMProducts(b *testing.B, engine *xorm.Engine, count int) {
	b.Helper()
	seedXORMProductsWithCode(b, engine, count, seedCode)
}

func seedXORMProductsWithCode(b *testing.B, engine *xorm.Engine, count int, code func(int) string) {
	b.Helper()
	const batchSize = 500
	for offset := 0; offset < count; offset += batchSize {
		end := min(offset+batchSize, count)
		products := make([]xormBenchProduct, 0, end-offset)
		for index := offset; index < end; index++ {
			products = append(products, *newXORMBenchProduct(code(index), uint(index%1000)))
		}
		if _, err := engine.Insert(&products); err != nil {
			b.Fatal(err)
		}
	}
}

func openBunBenchDB(b *testing.B, ctx context.Context) *bun.DB {
	b.Helper()
	config := currentBenchConfig()
	sqlDB, err := sql.Open(config.sqlDriverName(), config.sqlDSN())
	if err != nil {
		b.Fatal(err)
	}
	db := bun.NewDB(sqlDB, config.bunDialect())
	resetBunBenchTables(b, ctx, db)
	if _, err := db.NewCreateTable().Model((*bunBenchProduct)(nil)).IfNotExists().Exec(ctx); err != nil {
		b.Fatal(err)
	}
	if _, err := db.NewCreateIndex().Model((*bunBenchProduct)(nil)).Index("idx_products_code").Unique().Column("code").Exec(ctx); err != nil {
		b.Fatal(err)
	}
	return db
}

func closeBunBenchDB(b *testing.B, db *bun.DB) {
	b.Helper()
	if err := db.Close(); err != nil {
		b.Fatal(err)
	}
}

func seedBunProducts(b *testing.B, ctx context.Context, db *bun.DB, count int) {
	b.Helper()
	seedBunProductsWithCode(b, ctx, db, count, seedCode)
}

func seedBunProductsWithCode(b *testing.B, ctx context.Context, db *bun.DB, count int, code func(int) string) {
	b.Helper()
	products := make([]bunBenchProduct, 0, count)
	for index := 0; index < count; index++ {
		products = append(products, *newBunBenchProduct(code(index), uint(index%1000)))
	}
	if _, err := db.NewInsert().Model(&products).Exec(ctx); err != nil {
		b.Fatal(err)
	}
}

func resetOroBenchTables(b *testing.B, ctx context.Context, db *oro.DB) {
	b.Helper()
	for _, statement := range currentBenchConfig().resetSQL() {
		if _, err := db.Raw(statement).Exec(ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func resetGORMBenchTables(b *testing.B, db *gorm.DB) {
	b.Helper()
	for _, statement := range currentBenchConfig().resetSQL() {
		if err := db.Exec(statement).Error; err != nil {
			b.Fatal(err)
		}
	}
}

func resetXORMBenchTables(b *testing.B, engine *xorm.Engine) {
	b.Helper()
	for _, statement := range currentBenchConfig().resetSQL() {
		if _, err := engine.Exec(statement); err != nil {
			b.Fatal(err)
		}
	}
}

func resetBunBenchTables(b *testing.B, ctx context.Context, db *bun.DB) {
	b.Helper()
	for _, statement := range currentBenchConfig().resetSQL() {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			b.Fatal(err)
		}
	}
}

type benchDriverConfig struct {
	name         string
	dsn          string
	sqliteDriver string
}

func currentBenchConfig() benchDriverConfig {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("ORO_BENCH_DRIVER")))
	if name == "" {
		name = "sqlite"
	}
	switch name {
	case "mysql":
		return benchDriverConfig{name: name, dsn: envOr("ORO_BENCH_DSN", "root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=true&clientFoundRows=true")}
	case "pgsql", "postgres", "postgresql":
		return benchDriverConfig{name: "pgsql", dsn: envOr("ORO_BENCH_DSN", "postgres://root@localhost/duxorm?sslmode=disable")}
	default:
		return benchDriverConfig{
			name:         "sqlite",
			dsn:          memoryDSN(),
			sqliteDriver: sqliteBenchDriver(),
		}
	}
}

func (config benchDriverConfig) oroDriver() oro.Driver {
	switch config.name {
	case "mysql":
		return oromysql.Open(config.dsn)
	case "pgsql":
		return oropgsql.Open(config.dsn)
	default:
		if config.sqliteDriver == "mattn" {
			sqlDB, err := sql.Open("sqlite3", config.dsn)
			if err != nil {
				return benchErrorDriver{err: err}
			}
			return orosqlite.Wrap(sqlDB, orosqlite.OwnsDB(true))
		}
		return orosqlite.Open(config.dsn)
	}
}

func (config benchDriverConfig) gormDialector() gorm.Dialector {
	switch config.name {
	case "mysql":
		return gormmysql.Open(config.dsn)
	case "pgsql":
		return gormpostgres.Open(config.dsn)
	default:
		return gormsqlite.Open(config.dsn)
	}
}

func (config benchDriverConfig) xormDriverName() string {
	switch config.name {
	case "mysql":
		return "mysql"
	case "pgsql":
		return "pgx"
	default:
		if config.sqliteDriver == "mattn" {
			return "sqlite3"
		}
		return "sqlite"
	}
}

func (config benchDriverConfig) xormDSN() string {
	return config.sqlDSN()
}

func (config benchDriverConfig) sqlDriverName() string {
	switch config.name {
	case "mysql":
		return "mysql"
	case "pgsql":
		return "pgx"
	default:
		if config.sqliteDriver == "mattn" {
			return "sqlite3"
		}
		return sqliteshim.ShimName
	}
}

func (config benchDriverConfig) sqlDSN() string {
	return config.dsn
}

func (config benchDriverConfig) bunDialect() schema.Dialect {
	switch config.name {
	case "mysql":
		return mysqldialect.New()
	case "pgsql":
		return pgdialect.New()
	default:
		return sqlitedialect.New()
	}
}

func (config benchDriverConfig) resetSQL() []string {
	switch config.name {
	case "mysql":
		return []string{"drop table if exists products", "drop table if exists oro_schema"}
	case "pgsql":
		return []string{"drop table if exists products cascade", "drop table if exists oro_schema cascade"}
	default:
		return []string{"drop table if exists products", "drop table if exists oro_schema"}
	}
}

func envOr(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func sqliteBenchDriver() string {
	name := strings.ToLower(strings.TrimSpace(os.Getenv("ORO_BENCH_SQLITE_DRIVER")))
	switch name {
	case "", "modernc", "purego", "pure":
		return "modernc"
	case "mattn", "sqlite3", "cgo":
		return "mattn"
	default:
		return "modernc"
	}
}

type benchErrorDriver struct {
	err error
}

func (driver benchErrorDriver) Name() string {
	return "error"
}

func (driver benchErrorDriver) Open(context.Context) (*sql.DB, error) {
	return nil, driver.err
}

func (driver benchErrorDriver) Dialect() oro.Dialect {
	return orosqlite.Open(":memory:").Dialect()
}

func (driver benchErrorDriver) Inspector(db *sql.DB) oro.Inspector {
	return orosqlite.Open(":memory:").Inspector(db)
}

func (driver benchErrorDriver) TranslateError(err error) error {
	return err
}

func (driver benchErrorDriver) Owned() bool {
	return false
}

var benchDBCounter atomic.Uint64

func memoryDSN() string {
	return fmt.Sprintf("file:oro_bench_%d?mode=memory&cache=shared", benchDBCounter.Add(1))
}

func createCode(index int) string {
	return fmt.Sprintf("C%08d", index)
}

func createManyCode(index int) func(int) string {
	return func(offset int) string {
		return fmt.Sprintf("M%08d_%03d", index, offset)
	}
}

func deleteCode(index int) string {
	return fmt.Sprintf("D%08d", index)
}

func seedCode(index int) string {
	return fmt.Sprintf("S%08d", index%seedRows)
}

func makeOroBenchProducts(code func(int) string, count int) []*oroBenchProduct {
	products := make([]*oroBenchProduct, 0, count)
	for index := 0; index < count; index++ {
		products = append(products, &oroBenchProduct{Code: code(index), Price: uint(index % 1000)})
	}
	return products
}

func makeGORMBenchProducts(code func(int) string, count int) []gormBenchProduct {
	products := make([]gormBenchProduct, 0, count)
	for index := 0; index < count; index++ {
		products = append(products, *newGORMBenchProduct(code(index), uint(index%1000)))
	}
	return products
}

func makeXORMBenchProducts(code func(int) string, count int) []xormBenchProduct {
	products := make([]xormBenchProduct, 0, count)
	for index := 0; index < count; index++ {
		products = append(products, *newXORMBenchProduct(code(index), uint(index%1000)))
	}
	return products
}

func makeBunBenchProducts(code func(int) string, count int) []bunBenchProduct {
	products := make([]bunBenchProduct, 0, count)
	for index := 0; index < count; index++ {
		products = append(products, *newBunBenchProduct(code(index), uint(index%1000)))
	}
	return products
}

func newGORMBenchProduct(code string, price uint) *gormBenchProduct {
	now := time.Now()
	return &gormBenchProduct{Code: code, Price: price, CreatedAt: now, UpdatedAt: now}
}

func newXORMBenchProduct(code string, price uint) *xormBenchProduct {
	now := time.Now()
	return &xormBenchProduct{Code: code, Price: price, CreatedAt: now, UpdatedAt: now}
}

func newBunBenchProduct(code string, price uint) *bunBenchProduct {
	now := time.Now()
	return &bunBenchProduct{Code: code, Price: price, CreatedAt: now, UpdatedAt: now}
}
