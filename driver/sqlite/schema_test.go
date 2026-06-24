package sqlite

import (
	"context"
	"database/sql"
	"testing"

	"github.com/duxweb/oro"
	_ "modernc.org/sqlite"
)

func TestCompileSchemaCreateTable(t *testing.T) {
	statements, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind: oro.SchemaCreateTable,
		Table: oro.TableSpec{
			Name: "products",
			Columns: []oro.ColumnSpec{
				{ColumnName: "id", Type: "uint64", Primary: true},
				{ColumnName: "code", Type: "string", Size: 64, Nullable: true},
				{ColumnName: "price", Type: "decimal", Precision: 12, Scale: 2, Default: &oro.DefaultSpec{Value: 0}},
				{ColumnName: "meta", Type: "json", Nullable: true},
				{ColumnName: "created_at", Type: "time.Time", Nullable: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `create table if not exists "products" ("id" integer primary key autoincrement, "code" varchar(64), "price" numeric(12,2) not null default 0, "meta" text, "created_at" datetime)`
	if len(statements) != 1 || statements[0].SQL != want {
		t.Fatalf("got %#v, want %q", statements, want)
	}
}

func TestCompileSchemaAddColumn(t *testing.T) {
	statements, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind:  oro.SchemaAddColumn,
		Table: oro.TableSpec{Name: "products"},
		Column: oro.ColumnSpec{
			ColumnName: "stock",
			Type:       "uint",
			Nullable:   true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `alter table "products" add column "stock" integer`
	if len(statements) != 1 || statements[0].SQL != want {
		t.Fatalf("got %#v, want %q", statements, want)
	}
}

func TestCompileSchemaCreateIndex(t *testing.T) {
	statements, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind:  oro.SchemaCreateIndex,
		Table: oro.TableSpec{Name: "products"},
		Index: oro.IndexSpec{
			Name:   "uk_products_code",
			Fields: []string{"code"},
			Unique: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `create unique index if not exists "uk_products_code" on "products" ("code")`
	if len(statements) != 1 || statements[0].SQL != want {
		t.Fatalf("got %#v, want %q", statements, want)
	}
}

func TestCompileSchemaRenameColumn(t *testing.T) {
	statements, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind:  oro.SchemaRenameColumn,
		Table: oro.TableSpec{Name: "products"},
		Current: oro.ColumnSpec{
			ColumnName: "old_code",
		},
		Column: oro.ColumnSpec{
			ColumnName: "code",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `alter table "products" rename column "old_code" to "code"`
	if len(statements) != 1 || statements[0].SQL != want {
		t.Fatalf("got %#v, want %q", statements, want)
	}
}

func TestInspectorTableColumns(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, `create table products (id integer primary key autoincrement, code text, stock integer)`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.ExecContext(ctx, `create unique index uk_products_code on products (code)`)
	if err != nil {
		t.Fatal(err)
	}

	table, err := (inspector{db: db}).Table(ctx, "products")
	if err != nil {
		t.Fatal(err)
	}
	if table == nil || len(table.Columns) != 3 {
		t.Fatalf("unexpected table %#v", table)
	}
	if !table.Columns[0].Primary || table.Columns[0].ColumnName != "id" {
		t.Fatalf("unexpected primary column %#v", table.Columns[0])
	}
	if len(table.Indexes) != 1 || table.Indexes[0].Name != "uk_products_code" || !table.Indexes[0].Unique {
		t.Fatalf("unexpected indexes %#v", table.Indexes)
	}

	missing, err := (inspector{db: db}).Table(ctx, "missing")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Fatalf("expected nil missing table, got %#v", missing)
	}
}
