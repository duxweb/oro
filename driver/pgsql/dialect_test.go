package pgsql

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
			{Field: "deleted_at", Op: "is null"},
		},
		Order: []oro.OrderExpr{
			{Expr: "created_at", Desc: true},
			{Expr: "id"},
		},
		Limit: &limit,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "products" where "code" = $1 and "deleted_at" is null order by "created_at" desc, "id" asc limit 1`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 1 || sql.Args[0] != "A001" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectEmptyInValues(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{oro.Field("id").In()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sql.SQL != `select * from "products" where 1 = 0` {
		t.Fatalf("got SQL %q", sql.SQL)
	}

	sql, err = (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{oro.Field("id").NotIn()},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sql.SQL != `select * from "products" where 1 = 1` {
		t.Fatalf("got SQL %q", sql.SQL)
	}
}

func TestCompileSelectJSONLikeCondition(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{oro.JSON("meta").Path("name").Like("%pear%")},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "products" where "meta" #>> $1 like $2`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "{name}" || sql.Args[1] != "%pear%" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSchemaUUIDPrimaryKeyDoesNotUseIdentity(t *testing.T) {
	sql, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind: oro.SchemaCreateTable,
		Table: oro.TableSpec{
			Name: "users",
			Columns: []oro.ColumnSpec{
				{ColumnName: "id", Type: "uuid", Primary: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `create table if not exists "users" ("id" uuid primary key)`
	if len(sql) != 1 || sql[0].SQL != want {
		t.Fatalf("got SQL %#v, want %q", sql, want)
	}
}

func TestCompileSelectLock(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "jobs",
		Lock:  oro.LockSpec{Mode: oro.LockUpdate, SkipLocked: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "jobs" for update skip locked`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}

	sql, err = (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Lock:  oro.LockSpec{Mode: oro.LockShare, NoWait: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	want = `select * from "products" for share nowait`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
}

func TestCompileSelectJoin(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "shop.orders",
		Alias: "o",
		Select: []oro.SelectExpr{
			{Expr: "o.id"},
			{Expr: "u.name", Alias: "user_name"},
		},
		Joins: []oro.JoinAST{{
			Type:  oro.JoinLeft,
			Table: "account.users",
			Alias: "u",
			Conditions: []oro.JoinCondition{
				{Bool: "and", Left: "u.id", Op: "=", Right: "o.user_id", Column: true},
				{Bool: "and", Left: "u.status", Op: "=", Value: "active"},
			},
		}},
		Where: []oro.Condition{{Field: "o.total", Op: ">=", Value: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "o"."id", "u"."name" as "user_name" from "shop"."orders" as "o" left join "account"."users" as "u" on "u"."id" = "o"."user_id" and "u"."status" = $1 where "o"."total" >= $2`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "active" || sql.Args[1] != 100 {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectJoinSubqueryRebasesPlaceholders(t *testing.T) {
	subquery := oro.SelectAST{
		Table: "payments",
		Select: []oro.SelectExpr{
			{Expr: "order_id"},
			{Expr: "max(created_at)", Alias: "paid_at", Raw: true},
		},
		Where: []oro.Condition{{Field: "status", Op: "=", Value: "paid"}},
		Group: []string{"order_id"},
	}
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "orders",
		Alias: "o",
		Select: []oro.SelectExpr{
			{Expr: "o.id"},
			{Expr: "p.paid_at", Alias: "paid_at"},
		},
		Joins: []oro.JoinAST{{
			Type: oro.JoinLeft,
			Source: oro.SourceAST{
				Query: &subquery,
				Alias: "p",
			},
			Conditions: []oro.JoinCondition{
				{Bool: "and", Left: "p.order_id", Op: "=", Right: "o.id", Column: true},
				{Bool: "and", Left: "p.paid_at", Op: ">=", Value: "2026-01-01"},
			},
		}},
		Where: []oro.Condition{{Field: "o.total", Op: ">=", Value: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "o"."id", "p"."paid_at" as "paid_at" from "orders" as "o" left join (select "order_id", max(created_at) as "paid_at" from "payments" where "status" = $1 group by "order_id") as "p" on "p"."order_id" = "o"."id" and "p"."paid_at" >= $2 where "o"."total" >= $3`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 3 || sql.Args[0] != "paid" || sql.Args[1] != "2026-01-01" || sql.Args[2] != 100 {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectWhereHavingSubqueryRebasesPlaceholders(t *testing.T) {
	countQuery := oro.SelectAST{
		Table: "orders",
		Alias: "o",
		Select: []oro.SelectExpr{
			{Expr: "count(*)", Raw: true},
		},
		Where: []oro.Condition{
			{Field: "o.user_id", Op: "column", Value: oro.ColumnCondition{Op: "=", Right: "u.id"}},
			{Field: "o.status", Op: "=", Value: "paid"},
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
	avgTotal := oro.SelectAST{
		Table: "orders",
		Select: []oro.SelectExpr{
			{Expr: "avg(total)", Raw: true},
		},
		Where: []oro.Condition{
			{Field: "status", Op: "=", Value: "paid"},
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
			{Field: "u.score", Op: ">", Value: 10},
		},
		Group: []string{"u.id"},
		Having: []oro.Condition{
			{Field: "sum(total)", Op: ">", Value: &oro.SourceAST{Query: &avgTotal}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "u"."id", (select count(*) from "orders" as "o" where "o"."user_id" = "u"."id" and "o"."status" = $1) as "order_count" from "users" as "u" where "u"."id" in (select "id" from "users" where "status" = $2) and "u"."score" > $3 group by "u"."id" having "sum(total)" > (select avg(total) from "orders" where "status" = $4)`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 4 || sql.Args[0] != "paid" || sql.Args[1] != "active" || sql.Args[2] != 10 || sql.Args[3] != "paid" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectJoinRaw(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "orders",
		Alias: "o",
		Joins: []oro.JoinAST{{
			Raw: &oro.RawSpec{SQL: "left join payments p on p.order_id = o.id and p.status = $1", Args: []any{"paid"}},
		}},
		Where: []oro.Condition{{Field: "o.total", Op: ">", Value: 100}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "orders" as "o" left join payments p on p.order_id = o.id and p.status = $1 where "o"."total" > $2`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "paid" || sql.Args[1] != 100 {
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
	want := `select * from "products" where "meta" #> $1 = $2::jsonb and "meta" #> $3 is not null`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 3 || sql.Args[0] != "{vip}" || sql.Args[1] != "true" || sql.Args[2] != "{profile,country}" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectHavingRawQuestionPlaceholder(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table:  "orders",
		Select: []oro.SelectExpr{{Expr: "user_id"}, {Expr: "count(*)", Alias: "total", Raw: true}},
		Group:  []string{"user_id"},
		Having: []oro.Condition{{Op: "raw", Field: "count(*) > ?", Value: []any{1}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "user_id", count(*) as "total" from "orders" group by "user_id" having count(*) > $1`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 1 || sql.Args[0] != 1 {
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
			oro.RawCondition("meta->>'vip' = $1", true),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "products" where "status" = $1 and ("code" like $2 or ("price" >= $3 and "price" <= $4)) and not ("type" = $5) and "id" in ($6, $7) and "kind" not in ($8, $9) and "score" between $10 and $11 and meta->>'vip' = $12`
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

func TestCompileSelectRejectsUnsafeOperator(t *testing.T) {
	_, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{{Field: "id", Op: "= 1 OR 1=1 --", Value: 99}},
	})
	if !errors.Is(err, oro.ErrInvalidArgument) {
		t.Fatalf("expected invalid argument, got %v", err)
	}
}

func TestCompileSelectEscapedLike(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "products",
		Where: []oro.Condition{
			oro.Field("code").Contains(`100%_ok`),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select * from "products" where "code" like $1 escape '\'`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 1 || sql.Args[0] != `%100\%\_ok%` {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectRawExprArgs(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table:  "products",
		Select: []oro.SelectExpr{{Expr: "? as marker", Raw: true, Args: []any{"selected"}}},
		Where:  []oro.Condition{{Field: "code", Op: "=", Value: "A001"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select $1 as marker from "products" where "code" = $2`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "selected" || sql.Args[1] != "A001" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSelectFullText(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.SelectAST{
		Table: "articles",
		Select: []oro.SelectExpr{
			{Expr: "id"},
			{Expr: "__oro_fulltext_score__", Alias: "score", Raw: true, Args: []any{oro.FullText("title", "content").Score("golang orm")}},
		},
		Where: []oro.Condition{
			oro.FullText("title", "content").Match("golang orm"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "id", ts_rank(to_tsvector(coalesce("title"::text, '') || ' ' || coalesce("content"::text, '')), plainto_tsquery($1)) as "score" from "articles" where to_tsvector(coalesce("title"::text, '') || ' ' || coalesce("content"::text, '')) @@ plainto_tsquery($2)`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "golang orm" || sql.Args[1] != "golang orm" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileInsert(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.InsertAST{
		Table:     "products",
		Returning: true,
		Values: []oro.Map{{
			"code":  "A001",
			"price": 100,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `insert into "products" ("code", "price") values ($1, $2) returning *`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "A001" || sql.Args[1] != 100 {
		t.Fatalf("got args %#v", sql.Args)
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
	want := `insert into "products" ("code", "price") values ($1, $2), ($3, $4) returning *`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	wantArgs := []any{"A001", 100, "A002", 200}
	if len(sql.Args) != len(wantArgs) {
		t.Fatalf("got args %#v", sql.Args)
	}
	for index := range wantArgs {
		if sql.Args[index] != wantArgs[index] {
			t.Fatalf("got args %#v, want %#v", sql.Args, wantArgs)
		}
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
	want := `insert into "products" ("code", "price") values ($1, $2) on conflict ("code") do update set "price" = excluded."price" returning *`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != "A001" || sql.Args[1] != 100 {
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
	want := `insert into "products" ("code", "price") values ($1, $2) on conflict ("code") do update set "price" = "price" - $3 returning *`
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
			{Field: "count(*) > $1", Op: "raw", Value: []any{1}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `select "code", count(*) as "total" from "products" group by "code" having count(*) > $1`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 1 || sql.Args[0] != 1 {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileUpdatePlaceholderOrder(t *testing.T) {
	sql, err := (dialect{}).Compile(oro.UpdateAST{
		Table: "products",
		Values: oro.Map{
			"price": 100,
		},
		Where: []oro.Condition{
			{Field: "code", Op: "=", Value: "A001"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `update "products" set "price" = $1 where "code" = $2`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != 100 || sql.Args[1] != "A001" {
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
	want := `update "products" set "price" = "price" + $1 where "code" = $2`
	if sql.SQL != want {
		t.Fatalf("got SQL %q, want %q", sql.SQL, want)
	}
	if len(sql.Args) != 2 || sql.Args[0] != 5 || sql.Args[1] != "A001" {
		t.Fatalf("got args %#v", sql.Args)
	}
}

func TestCompileSchemaCreateTable(t *testing.T) {
	statements, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind: oro.SchemaCreateTable,
		Table: oro.TableSpec{
			Name: "products",
			Columns: []oro.ColumnSpec{
				{ColumnName: "id", Type: "uint64", Primary: true},
				{ColumnName: "code", Type: "string", Size: 64, Nullable: true, Comment: "product code"},
				{ColumnName: "price", Type: "decimal", Precision: 12, Scale: 2, Default: &oro.DefaultSpec{Value: 0}},
				{ColumnName: "meta", Type: "json", Nullable: true},
				{ColumnName: "created_at", Type: "time.Time", Nullable: true},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `create table if not exists "products" ("id" bigint primary key generated by default as identity, "code" varchar(64), "price" numeric(12,2) not null default 0, "meta" jsonb, "created_at" timestamp)`
	comment := `comment on column "products"."code" is 'product code'`
	if len(statements) != 2 || statements[0].SQL != want || statements[1].SQL != comment {
		t.Fatalf("got %#v, want %q", statements, want)
	}
}

func TestCompileSchemaAddColumnWithComment(t *testing.T) {
	statements, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind:  oro.SchemaAddColumn,
		Table: oro.TableSpec{Name: "products"},
		Column: oro.ColumnSpec{
			ColumnName: "stock",
			Type:       "uint",
			Nullable:   true,
			Comment:    "stock's value",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `alter table "products" add column "stock" integer`
	comment := `comment on column "products"."stock" is 'stock''s value'`
	if len(statements) != 2 || statements[0].SQL != want || statements[1].SQL != comment {
		t.Fatalf("got %#v, want %q then %q", statements, want, comment)
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

func TestCompileSchemaCreateFullTextIndex(t *testing.T) {
	statements, err := (dialect{}).CompileSchema(oro.SchemaChange{
		Kind:  oro.SchemaCreateIndex,
		Table: oro.TableSpec{Name: "products"},
		Index: oro.IndexSpec{
			Name:     "ft_products_code_title",
			Fields:   []string{"code", "title"},
			FullText: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := `create index if not exists "ft_products_code_title" on "products" using gin (to_tsvector('simple', coalesce("code", '') || ' ' || coalesce("title", '')))`
	if len(statements) != 1 || statements[0].SQL != want {
		t.Fatalf("got %#v, want %q", statements, want)
	}
}
