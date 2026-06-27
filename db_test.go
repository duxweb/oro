package oro

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

type fakeDriver struct {
	db *sql.DB
}

func (driver fakeDriver) Name() string { return "fake" }
func (driver fakeDriver) Open(ctx context.Context) (*sql.DB, error) {
	return driver.db, nil
}
func (driver fakeDriver) Dialect() Dialect { return fakeDialect{} }
func (driver fakeDriver) Inspector(db *sql.DB) Inspector {
	return nil
}
func (driver fakeDriver) TranslateError(err error) error { return err }
func (driver fakeDriver) Owned() bool                    { return false }

type fakeDialect struct{}

func (fakeDialect) Name() string { return "fake" }
func (fakeDialect) Capabilities() Capabilities {
	return Capabilities{}
}
func (fakeDialect) QuoteIdent(name string) string              { return name }
func (fakeDialect) Placeholder(index int) string               { return "?" }
func (fakeDialect) DataType(column ColumnSpec) (string, error) { return column.Type, nil }
func (fakeDialect) NormalizeType(dbType string) (ColumnType, error) {
	return ColumnType{DBType: dbType}, nil
}
func (fakeDialect) Compile(stmt Statement) (CompiledSQL, error) {
	switch statement := stmt.(type) {
	case SelectAST:
		return compileFakeSelect(statement), nil
	case UpdateAST:
		return compileFakeUpdate(statement), nil
	case DeleteAST:
		return compileFakeDelete(statement), nil
	default:
		return CompiledSQL{}, nil
	}
}
func (fakeDialect) CompileSchema(change SchemaChange) ([]CompiledSQL, error) {
	return nil, nil
}

func compileFakeSelect(stmt SelectAST) CompiledSQL {
	fields := "*"
	if len(stmt.Select) > 0 {
		parts := make([]string, 0, len(stmt.Select))
		for _, item := range stmt.Select {
			if item.Alias != "" {
				parts = append(parts, item.Expr+" as "+item.Alias)
			} else {
				parts = append(parts, item.Expr)
			}
		}
		fields = strings.Join(parts, ", ")
	}
	sql := "select " + fields + " from " + stmt.Table
	where, args := compileFakeWhere(stmt.Where)
	if where != "" {
		sql += " where " + where
	}
	if stmt.Limit != nil {
		sql += fmt.Sprintf(" limit %d", *stmt.Limit)
	}
	return CompiledSQL{SQL: sql, Args: args}
}

func compileFakeUpdate(stmt UpdateAST) CompiledSQL {
	sets := make([]string, 0, len(stmt.Values))
	args := make([]any, 0, len(stmt.Values)+len(stmt.Where))
	for key, value := range stmt.Values {
		sets = append(sets, key+" = ?")
		args = append(args, value)
	}
	sql := "update " + stmt.Table + " set " + strings.Join(sets, ", ")
	where, whereArgs := compileFakeWhere(stmt.Where)
	if where != "" {
		sql += " where " + where
		args = append(args, whereArgs...)
	}
	return CompiledSQL{SQL: sql, Args: args}
}

func compileFakeDelete(stmt DeleteAST) CompiledSQL {
	sql := "delete from " + stmt.Table
	where, args := compileFakeWhere(stmt.Where)
	if where != "" {
		sql += " where " + where
	}
	return CompiledSQL{SQL: sql, Args: args}
}

func compileFakeWhere(conditions []Condition) (string, []any) {
	parts := make([]string, 0, len(conditions))
	args := []any{}
	for _, condition := range conditions {
		switch strings.ToLower(condition.Op) {
		case "is null":
			parts = append(parts, condition.Field+" is null")
		default:
			parts = append(parts, condition.Field+" "+condition.Op+" ?")
			args = append(args, condition.Value)
		}
	}
	return strings.Join(parts, " and "), args
}

func TestOpenRequiresConnections(t *testing.T) {
	_, err := Open(Config{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestConnectionCloneKeepsRuntime(t *testing.T) {
	db, err := Open(Config{
		Connections: map[string]ConnectionConfig{
			"default": {Driver: fakeDriver{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	clone := db.Connection("other")
	if clone == db {
		t.Fatal("expected clone")
	}
	if clone.runtime != db.runtime {
		t.Fatal("expected shared runtime")
	}
	if clone.session.connection != "other" {
		t.Fatalf("got %q", clone.session.connection)
	}
}

func TestReadQueriesUseReplicasAndUsePrimaryForcesPrimary(t *testing.T) {
	ctx := context.Background()
	primary, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	read, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer primary.Close()
	defer read.Close()

	for _, setup := range []struct {
		db    *sql.DB
		value string
	}{
		{db: primary, value: "primary"},
		{db: read, value: "read"},
	} {
		if _, err := setup.db.ExecContext(ctx, `create table items (id integer primary key, name text)`); err != nil {
			t.Fatal(err)
		}
		if _, err := setup.db.ExecContext(ctx, `insert into items (id, name) values (1, ?)`, setup.value); err != nil {
			t.Fatal(err)
		}
	}

	db, err := Open(Config{
		Connections: map[string]ConnectionConfig{
			"default": {
				Driver: fakeDriver{db: primary},
				Reads:  []Driver{fakeDriver{db: read}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	row, err := db.Table("items").Where("id", 1).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row["name"] != "read" {
		t.Fatalf("expected read replica row, got %#v", row)
	}

	row, err = db.Table("items").UsePrimary().Where("id", 1).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row["name"] != "primary" {
		t.Fatalf("expected primary row, got %#v", row)
	}

	if _, err := db.Table("items").Where("id", 1).Update(ctx, Map{"name": "written"}); err != nil {
		t.Fatal(err)
	}
	row, err = db.Table("items").UsePrimary().Where("id", 1).First(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if row["name"] != "written" {
		t.Fatalf("expected write on primary, got %#v", row)
	}
}

type connectionModel struct {
	Model
	Name string
}

func (connectionModel) Define(s *SchemaBuilder) {
	s.Connection("read_model")
	s.Table("connection_models")
	s.Field("Name").String()
}

func TestModelConnectionAndManualOverride(t *testing.T) {
	ctx := context.Background()
	defaultDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	modelDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer defaultDB.Close()
	defer modelDB.Close()

	for _, setup := range []struct {
		db    *sql.DB
		value string
	}{
		{db: defaultDB, value: "default"},
		{db: modelDB, value: "model"},
	} {
		if _, err := setup.db.ExecContext(ctx, `create table connection_models (id integer primary key, name text, created_at datetime, updated_at datetime, deleted_at datetime)`); err != nil {
			t.Fatal(err)
		}
		if _, err := setup.db.ExecContext(ctx, `insert into connection_models (id, name) values (1, ?)`, setup.value); err != nil {
			t.Fatal(err)
		}
	}

	db, err := Open(Config{
		Connections: map[string]ConnectionConfig{
			"default":    {Driver: fakeDriver{db: defaultDB}},
			"read_model": {Driver: fakeDriver{db: modelDB}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Register(connectionModel{}); err != nil {
		t.Fatal(err)
	}

	model, err := db.Use[connectionModel]().Find(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if model == nil || model.Name != "model" {
		t.Fatalf("expected model connection, got %#v", model)
	}

	model, err = db.Connection("default").Use[connectionModel]().Find(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if model == nil || model.Name != "default" {
		t.Fatalf("expected manual connection override, got %#v", model)
	}
}

type badConnectionModel struct {
	Model
	Name string
}

func (badConnectionModel) Define(s *SchemaBuilder) {
	s.Connection("missing")
	s.Table("bad_connection_models")
	s.Field("Name").String()
}

func TestRegisterUnknownModelConnection(t *testing.T) {
	db, err := Open(Config{
		Connections: map[string]ConnectionConfig{
			"default": {Driver: fakeDriver{}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	err = db.Register(badConnectionModel{})
	if !errors.Is(err, ErrUnknownConnection) {
		t.Fatalf("expected unknown connection, got %v", err)
	}
}

func TestChunkMapsForCreateParams(t *testing.T) {
	values := []Map{
		{"a": 1, "b": 2},
		{"a": 3, "b": 4},
		{"a": 5, "b": 6},
	}
	chunks := chunkMapsForCreateParams(values, 100, 4)
	if len(chunks) != 2 || len(chunks[0]) != 2 || len(chunks[1]) != 1 {
		t.Fatalf("unexpected chunks %#v", chunks)
	}
}
