package translation_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/mysql"
	"github.com/duxweb/oro/driver/pgsql"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/translation"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type matrixProduct struct {
	oro.Model
	translation.Fields

	Code string
	Name string
}

func (matrixProduct) Define(s *oro.SchemaBuilder) {
	s.Table("oro_translation_matrix_products")
	s.Field("Code").String().Unique()
	s.Field("Name").String().Nullable()
}

func TestTranslationDriverMatrix(t *testing.T) {
	for _, testCase := range translationDriverCases() {
		t.Run(testCase.name, func(t *testing.T) {
			db, ctx := openTranslationMatrixDB(t, testCase)

			created, err := translation.Use[matrixProduct](db).Create(ctx, &matrixProduct{Code: "TR001"}, translation.Values{
				"zh-CN": oro.Map{"Name": "苹果"},
				"en-US": oro.Map{"Name": "Apple"},
			})
			if err != nil {
				t.Fatalf("create: %v", err)
			}

			en, err := translation.Use[matrixProduct](db).
				Locale("en-US").
				WhereTrans("Name", "Apple").
				First(ctx)
			if err != nil {
				t.Fatalf("where trans en-US: %v", err)
			}
			if en == nil || en.ID != created.ID || en.Name != "Apple" {
				t.Fatalf("unexpected en-US product %#v", en)
			}

			fallback, err := translation.Use[matrixProduct](db).
				Locale("ja-JP").
				Fallback("en-US").
				Find(ctx, created.ID)
			if err != nil {
				t.Fatalf("find fallback: %v", err)
			}
			if fallback == nil || fallback.Name != "Apple" {
				t.Fatalf("expected fallback translation, got %#v", fallback)
			}

			if _, err := translation.Use[matrixProduct](db).
				Locale("zh-CN").
				Where("ID", created.ID).
				Update(ctx, oro.Map{"Name": "新苹果"}); err != nil {
				t.Fatalf("update zh-CN: %v", err)
			}

			preserved, err := translation.Use[matrixProduct](db).Locale("en-US").Find(ctx, created.ID)
			if err != nil {
				t.Fatalf("find preserved en-US: %v", err)
			}
			if preserved == nil || preserved.Name != "Apple" {
				t.Fatalf("expected en-US to be preserved, got %#v", preserved)
			}

			updated, err := translation.Use[matrixProduct](db).Locale("zh-CN").Find(ctx, created.ID)
			if err != nil {
				t.Fatalf("find updated zh-CN: %v", err)
			}
			if updated == nil || updated.Name != "新苹果" {
				t.Fatalf("expected updated zh-CN, got %#v", updated)
			}
		})
	}
}

type translationDriverCase struct {
	name   string
	driver oro.Driver
}

func translationDriverCases() []translationDriverCase {
	mysqlDSN := os.Getenv("ORO_MYSQL_DSN")
	if mysqlDSN == "" {
		mysqlDSN = "root:root@tcp(localhost:3306)/duxorm?parseTime=true&multiStatements=false"
	}
	pgsqlDSN := os.Getenv("ORO_PGSQL_DSN")
	if pgsqlDSN == "" {
		pgsqlDSN = "postgres://root@localhost:5432/duxorm?sslmode=disable"
	}
	return []translationDriverCase{
		{name: "sqlite", driver: sqlite.Open(":memory:")},
		{name: "mysql", driver: mysql.Open(mysqlDSN)},
		{name: "pgsql", driver: pgsql.Open(pgsqlDSN)},
	}
}

func openTranslationMatrixDB(t *testing.T, testCase translationDriverCase) (*oro.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: testCase.driver},
		},
		Pool: oro.PoolConfig{
			MaxOpenConns: 4,
			MaxIdleConns: 2,
			PingOnOpen:   true,
		},
		Timeout: oro.TimeoutConfig{
			Connect: 3 * time.Second,
			Query:   10 * time.Second,
		},
		Extensions: []oro.Extension{
			translation.Extension(
				translation.DefaultLocale("zh-CN"),
				translation.FallbackLocale("en-US"),
				translation.TranslatedFields("Name"),
			),
		},
	})
	if err != nil {
		if translationDBUnavailable(err) {
			t.Skipf("%s translation database unavailable: %v", testCase.name, err)
		}
		t.Fatal(err)
	}
	resetTranslationMatrix(t, ctx, db)
	if err := db.Register(matrixProduct{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		resetTranslationMatrix(t, ctx, db)
		if err := db.Close(ctx); err != nil {
			t.Fatal(err)
		}
	})
	return db, ctx
}

func resetTranslationMatrix(t *testing.T, ctx context.Context, db *oro.DB) {
	t.Helper()
	_, err := db.Raw("drop table if exists oro_translation_matrix_products").Exec(ctx)
	if err != nil && !translationDBUnavailable(err) {
		t.Fatal(err)
	}
	_, err = db.Raw("drop table if exists oro_schema").Exec(ctx)
	if err != nil && !translationDBUnavailable(err) {
		t.Fatal(err)
	}
}

func translationDBUnavailable(err error) bool {
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
