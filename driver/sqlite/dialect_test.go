package sqlite

import (
	"errors"
	"testing"

	"github.com/duxweb/oro"
)

func TestCompileSelect(t *testing.T) {
	limit := 1
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{
			{Field: "code", Op: "=", Value: "A001"},
		},
		Order: []oro.OrderExpr{
			{Expr: "id", Desc: true},
		},
		Limit: &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	if sql.SQL != `select * from "products" where "code" = ? order by "id" desc limit 1` {
		t.Fatalf("got SQL %q", sql.SQL)
	}
	if len(sql.Args) != 1 || sql.Args[0] != "A001" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectLock(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "jobs",
		Lock:  oro.LockSpec{Mode: oro.LockUpdate},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "jobs"`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}

	_, err = (dialect{}).Compile(oro.SelectAST{
		Table: "jobs",
		Lock:  oro.LockSpec{Mode: oro.LockUpdate, SkipLocked: true},
	})
	if err == nil {
		t.Fatal("expected unsupported lock option")
	}
}

func TestCompileSchemaFullTextIndexUnsupported(t *testing.T) {
	_, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind:  oro.SchemaCreateIndex,
		Table: oro.TableSpec{Name: "products"},
		Index: oro.IndexSpec{
			Name:     "ft_products_code",
			Fields:   []string{"code"},
			FullText: true,
		},
	})
	if !errors.Is(err, oro.ErrUnsupported) {
		t.Fatalf("expected unsupported fulltext index, got %v", err)
	}
}

func TestCompileSelectJoin(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "orders",
		Alias: "o",
		Select: []oro.SelectExpr{
			{Expr: "o.id"},
			{Expr: "u.name", Alias: "user_name"},
		},
		Joins: []oro.JoinAST{{
			Type:  oro.JoinLeft,
			Table: "users",
			Alias: "u",
			Conditions: []oro.JoinCondition{
				{Bool: "and", Left: "u.id", Op: "=", Right: "o.user_id", Column: true},
				{Bool: "and", Left: "u.status", Op: "=", Value: "active"},
			},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "o"."id", "u"."name" as "user_name" from "orders" as "o" left join "users" as "u" on "u"."id" = "o"."user_id" and "u"."status" = ?`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 1 || sql.Args[0] != "active" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectJoinRaw(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "orders",
		Alias: "o",
		Joins: []oro.JoinAST{{
			Raw: &oro.RawSpec{SQL: "left join payments p on p.order_id = o.id and p.status = ?", Args: []any{"paid"}},
		}},
		Where: []oro.Condition{{Field: "o.total", Op: ">", Value: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "orders" as "o" left join payments p on p.order_id = o.id and p.status = ? where "o"."total" > ?`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "paid" || sql.Args[1] != 100 {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectSubqueries(t *testing.T) {
	countQuery := oro.SelectAST{
		Table: "orders",
		Alias: "o",
		Select: []oro.SelectExpr{
			{Expr: "count(*)", Raw: true},
		},
		Where: []oro.Condition{
			{Field: "o.user_id", Op: "column", Value: oro.ColumnCondition{Op: "=", Right: "u.id"}},
		},
	}
	activeUsers := oro.SelectAST{
		Table: "users",
		Select: []oro.SelectExpr{
			{Expr: "id"},
		},
		Where: []oro.Condition{
			{Field: "status", Op: "=", Value: "active"},
		},
	}
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "users",
		Alias: "u",
		Select: []oro.SelectExpr{
			{Expr: "u.id"},
			{Alias: "order_count", Source: &oro.SourceAST{Query: &countQuery}},
		},
		Where: []oro.Condition{
			{Field: "u.id", Op: "in", Value: &oro.SourceAST{Query: &activeUsers}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "u"."id", (select count(*) from "orders" as "o" where "o"."user_id" = "u"."id") as "order_count" from "users" as "u" where "u"."id" in (select "id" from "users" where "status" = ?)`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 1 || sql.Args[0] != "active" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectJSONCondition(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{
			oro.JSON("meta").Path("vip").Eq(true),
			oro.JSON("meta").Path("profile", "country").Exists(),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "products" where json_extract("meta", ?) = ? and json_type("meta", ?) is not null`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 3 || sql.Args[0] != "$.vip" || sql.Args[1] != true || sql.Args[2] != "$.profile.country" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectConditionTree(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{
			oro.Field("status").Eq("active"),
			oro.Or(
				oro.Field("code").Like("A%"),
				oro.And(
					oro.Field("price").Gte(100),
					oro.Field("price").Lte(500),
				),
			),
			oro.Not(oro.Field("type").Eq("internal")),
			oro.Field("id").In(1, 2),
			oro.Field("kind").NotIn("x", "y"),
			oro.Field("score").Between(10, 20),
			oro.RawCondition("json_extract(meta, '$.vip') = ?", true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "products" where "status" = ? and ("code" like ? or ("price" >= ? and "price" <= ?)) and not ("type" = ?) and "id" in (?, ?) and "kind" not in (?, ?) and "score" between ? and ? and json_extract(meta, '$.vip') = ?`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	wantArgs := []any{"active", "A%", 100, 500, "internal", 1, 2, "x", "y", 10, 20, true}
	if len(sql.Args) != len(wantArgs) {
		t.Fatalf("got args %#v", sql.Args)
	}
	for index := range wantArgs {
		if sql.Args[index] != wantArgs[index] {
			t.Fatalf("got args %#v, want %#v", sql.Args, wantArgs)
		}
	}
}

func TestCompileSelectFullTextUnsupported(t *testing.T) {
	_, err := (dialect{}).Compile(oro.SelectAST{
		Table: "articles",
		Where: []oro.Condition{
			oro.FullText("title", "content").Match("golang orm"),
		},
	})
	if err == nil {
		t.Fatal("expected unsupported fulltext error")
	}

	_, err = (dialect{}).Compile(oro.SelectAST{
		Table: "articles",
		Select: []oro.SelectExpr{{
			Expr: "__oro_fulltext_score__",
			Raw:  true,
			Args: []any{oro.FullText("title").Score("golang orm")},
		}},
	})
	if err == nil {
		t.Fatal("expected unsupported fulltext score error")
	}
}

func TestCompileInsert(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.InsertAST{
		Table:     "products",
		Returning: true,
		Values: []oro.Map{{
			"code": "A001",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sql.SQL != `insert into "products" ("code") values (?) returning *` {
		t.Fatalf("got SQL %q", sql.SQL)
	}
	if len(sql.Args) != 1 || sql.Args[0] != "A001" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileInsertWithoutReturning(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.InsertAST{
		Table: "products",
		Values: []oro.Map{{
			"code": "A001",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sql.SQL != `insert into "products" ("code") values (?)` {
		t.Fatalf("got SQL %q", sql.SQL)
	}
}

func TestCompileInsertMany(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.InsertAST{
		Table:     "products",
		Returning: true,
		Values: []oro.Map{
			{"code": "A001", "price": 100},
			{"code": "A002", "price": 200},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `insert into "products" ("code", "price") values (?, ?), (?, ?) returning *`
	if sql.SQL != want {
		t.Fatalf("got SQL %q", sql.SQL)
	}
	if len(sql.Args) != 4 || sql.Args[0] != "A001" || sql.Args[1] != 100 || sql.Args[2] != "A002" || sql.Args[3] != 200 {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileUpdateExpressions(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.UpdateAST{
		Table: "products",
		Values: oro.Map{
			"price": oro.Increment(5),
		},
		Where: []oro.Condition{{Field: "code", Op: "=", Value: "A001"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `update "products" set "price" = "price" + ? where "code" = ?`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != 5 || sql.Args[1] != "A001" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileUpsert(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.InsertAST{
		Table: "products",
		Values: []oro.Map{{
			"code":  "A001",
			"price": 100,
		}},
		Conflict: oro.ConflictSpec{
			Columns: []string{"code"},
			Update:  []string{"price"},
		},
		Returning: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `insert into "products" ("code", "price") values (?, ?) on conflict ("code") do update set "price" = ? returning *`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 3 || sql.Args[0] != "A001" || sql.Args[1] != 100 || sql.Args[2] != 100 {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileUpsertExpressions(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.InsertAST{
		Table: "products",
		Values: []oro.Map{{
			"code":  "A001",
			"price": 100,
		}},
		Conflict: oro.ConflictSpec{
			Columns: []string{"code"},
			UpdateMap: oro.Map{
				"price": oro.Decrement(5),
			},
		},
		Returning: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `insert into "products" ("code", "price") values (?, ?) on conflict ("code") do update set "price" = "price" - ? returning *`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 3 || sql.Args[0] != "A001" || sql.Args[1] != 100 || sql.Args[2] != 5 {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileGroupHaving(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Select: []oro.SelectExpr{
			{Expr: "code"},
			{Expr: "count(*)", Alias: "total", Raw: true},
		},
		Group: []string{"code"},
		Having: []oro.Condition{
			{Field: "count(*) > ?", Op: "raw", Value: []any{1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "code", count(*) as "total" from "products" group by "code" having count(*) > ?`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 1 || sql.Args[0] != 1 {
		t.Fatalf("got args %#v", sql.Args)
	}
}
