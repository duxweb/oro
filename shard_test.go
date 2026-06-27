package oro_test

import (
	"context"
	"errors"
	"testing"

	oro "github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/tenant"
	_ "modernc.org/sqlite"
)

type shardOrder struct {
	oro.Model
	TenantID uint64
	Code     string
	Status   string
}

func (shardOrder) Define(s *oro.SchemaBuilder) {
	s.Table("shard_orders")
	s.Shard("orders", "TenantID")
	s.Field("TenantID").UnsignedBigInt().Index()
	s.Field("Code").String().Unique()
	s.Field("Status").String()
}

type shardLog struct {
	oro.Model
	TenantID uint64
	Message  string
}

func (shardLog) Define(s *oro.SchemaBuilder) {
	s.Table("shard_logs")
	s.Shard("logs", "TenantID")
	s.Field("TenantID").UnsignedBigInt()
	s.Field("Message").String()
}

func newShardTestDB(t *testing.T) (*oro.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Default: "shard_0",
		Connections: map[string]oro.ConnectionConfig{
			"shard_0": {Driver: sqlite.Open(":memory:")},
			"shard_1": {Driver: sqlite.Open(":memory:")},
		},
		Shards: map[string]oro.ShardConfig{
			"orders": {Connections: []string{"shard_0", "shard_1"}, Strategy: oro.ModShard("TenantID")},
			"logs":   {Connections: []string{"shard_0", "shard_1"}, Strategy: oro.ModShard("TenantID")},
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
	if err := db.Register(shardOrder{}, shardLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	return db, ctx
}

func TestShardModelRoutesCRUD(t *testing.T) {
	db, ctx := newShardTestDB(t)

	_, err := db.Use[shardOrder]().Create(ctx, &shardOrder{TenantID: 1, Code: "missing", Status: "new"})
	if !errors.Is(err, oro.ErrShardRequired) {
		t.Fatalf("expected ErrShardRequired, got %v", err)
	}

	created1, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(1)}).Create(ctx, &shardOrder{TenantID: 1, Code: "A001", Status: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if created1.ID == 0 || created1.Code != "A001" {
		t.Fatalf("unexpected created shard order %#v", created1)
	}
	created2, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(2)}).Create(ctx, &shardOrder{TenantID: 2, Code: "B001", Status: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if created2.ID == 0 || created2.Code != "B001" {
		t.Fatalf("unexpected created shard order %#v", created2)
	}

	shard1Rows, err := db.Connection("shard_1").Table("shard_orders").OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shard1Rows) != 1 || shard1Rows[0]["code"] != "A001" {
		t.Fatalf("expected tenant 1 on shard_1, got %#v", shard1Rows)
	}
	shard0Rows, err := db.Connection("shard_0").Table("shard_orders").OrderBy("code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(shard0Rows) != 1 || shard0Rows[0]["code"] != "B001" {
		t.Fatalf("expected tenant 2 on shard_0, got %#v", shard0Rows)
	}

	updated, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(1)}).Where("Code", "A001").Update(ctx, oro.Map{"Status": "paid"})
	if err != nil {
		t.Fatal(err)
	}
	if updated != 1 {
		t.Fatalf("expected one update, got %d", updated)
	}
	order, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(1)}).Where("Code", "A001").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != "paid" {
		t.Fatalf("expected updated status, got %#v", order)
	}

	deleted, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(2)}).Where("Code", "B001").ForceDelete(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("expected one delete, got %d", deleted)
	}
}

func TestShardUsesTenantValuesForRouting(t *testing.T) {
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Default: "shard_0",
		Connections: map[string]oro.ConnectionConfig{
			"shard_0": {Driver: sqlite.Open(":memory:")},
			"shard_1": {Driver: sqlite.Open(":memory:")},
		},
		Extensions: []oro.Extension{tenant.Extension(tenant.Fields("TenantID"))},
		Shards: map[string]oro.ShardConfig{
			"orders": {Connections: []string{"shard_0", "shard_1"}, Strategy: oro.ModShard("TenantID")},
			"logs":   {Connections: []string{"shard_0", "shard_1"}, Strategy: oro.ModShard("TenantID")},
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
	if err := db.Register(shardOrder{}, shardLog{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}

	created, err := tenant.Use(db, oro.Map{"TenantID": uint64(3)}).Use[shardOrder]().Create(ctx, &shardOrder{TenantID: 3, Code: "T003", Status: "new"})
	if err != nil {
		t.Fatal(err)
	}
	if created.TenantID != 3 {
		t.Fatalf("unexpected tenant routed row %#v", created)
	}
	count, err := tenant.Use(db, oro.Map{"TenantID": uint64(3)}).Use[shardOrder]().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected tenant routed count 1, got %d", count)
	}
	_, err = tenant.Use(db, oro.Map{"TenantID": uint64(3)}).Use[shardOrder]().Create(ctx, &shardOrder{TenantID: 4, Code: "bad-tenant", Status: "new"})
	if !errors.Is(err, oro.ErrShardConflict) {
		t.Fatalf("expected shard conflict on tenant routed create, got %v", err)
	}
}

func TestShardAllShardsQueries(t *testing.T) {
	db, ctx := newShardTestDB(t)
	if _, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(1)}).Create(ctx, &shardOrder{TenantID: 1, Code: "A001", Status: "new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(2)}).Create(ctx, &shardOrder{TenantID: 2, Code: "B001", Status: "new"}); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(3)}).Create(ctx, &shardOrder{TenantID: 3, Code: "AAA", Status: "new"}); err != nil {
		t.Fatal(err)
	}

	rows, err := db.Use[shardOrder]().AllShards().OrderBy("Code").Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 || rows[0].Code != "A001" || rows[1].Code != "AAA" || rows[2].Code != "B001" {
		t.Fatalf("unexpected all shard global order %#v", rows)
	}
	first, err := db.Use[shardOrder]().AllShards().OrderBy("Code").First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil || first.Code != "A001" {
		t.Fatalf("expected global first A001, got %#v", first)
	}
	paged, err := db.Use[shardOrder]().AllShards().OrderBy("Code").Offset(1).Limit(1).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(paged) != 1 || paged[0].Code != "AAA" {
		t.Fatalf("unexpected global page %#v", paged)
	}
	count, err := db.Use[shardOrder]().AllShards().Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected all shard count 3, got %d", count)
	}
	exists, err := db.Use[shardOrder]().AllShards().Where("Code", "A001").Exists(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected all shard exists")
	}
	_, err = db.Use[shardOrder]().AllShards().First(ctx)
	if !errors.Is(err, oro.ErrOrderRequired) {
		t.Fatalf("expected ErrOrderRequired, got %v", err)
	}
}

func TestShardTableQuery(t *testing.T) {
	db, ctx := newShardTestDB(t)

	row, err := db.Table("shard_orders").Shard("orders", oro.Map{"TenantID": uint64(1)}).Create(ctx, oro.Map{"tenant_id": uint64(1), "code": "TBL2", "status": "new"})
	if err != nil {
		t.Fatal(err)
	}
	if row["code"] != "TBL2" {
		t.Fatalf("unexpected table shard row %#v", row)
	}
	if _, err := db.Table("shard_orders").Shard("orders", oro.Map{"TenantID": uint64(2)}).Create(ctx, oro.Map{"tenant_id": uint64(2), "code": "TBL1", "status": "new"}); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Table("shard_orders").AllShards("orders").OrderBy("code").Limit(1).Get(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0]["code"] != "TBL1" {
		t.Fatalf("expected globally sorted table shard row, got %#v", rows)
	}
}

func TestShardConflictsAndTransactions(t *testing.T) {
	db, ctx := newShardTestDB(t)

	_, err := db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(1)}).Create(ctx, &shardOrder{TenantID: 2, Code: "bad", Status: "new"})
	if !errors.Is(err, oro.ErrShardConflict) {
		t.Fatalf("expected shard conflict on create, got %v", err)
	}
	_, err = db.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(1)}).Where("Code", "none").Update(ctx, oro.Map{"TenantID": uint64(2)})
	if !errors.Is(err, oro.ErrShardConflict) {
		t.Fatalf("expected shard conflict on update, got %v", err)
	}

	err = db.Use[shardOrder]().AllShards().Chunk(ctx, 10, func(rows []*shardOrder) error { return nil })
	if !errors.Is(err, oro.ErrUnsupported) {
		t.Fatalf("expected unsupported all shard chunk, got %v", err)
	}

	err = db.Connection("shard_1").Transaction(ctx, func(tx *oro.DB) error {
		_, err := tx.Use[shardOrder]().Shard(oro.Map{"TenantID": uint64(2)}).Where("Code", "none").Update(ctx, oro.Map{"Status": "paid"})
		return err
	})
	if !errors.Is(err, oro.ErrCrossShardTransaction) {
		t.Fatalf("expected cross shard transaction, got %v", err)
	}
}
