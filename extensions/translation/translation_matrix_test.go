package translation_test

import (
	"context"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/extensions/internal/exttest"
	"github.com/duxweb/oro/extensions/translation"
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
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
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

func openTranslationMatrixDB(t *testing.T, testCase exttest.DriverCase) (*oro.DB, context.Context) {
	t.Helper()
	return exttest.Open(t, testCase, exttest.OpenOptions{
		Models: []oro.Definer{matrixProduct{}},
		Tables: []string{"oro_translation_matrix_products"},
		Prefix: "translation_matrix_",
		Extensions: []oro.Extension{
			translation.Extension(
				translation.DefaultLocale("zh-CN"),
				translation.FallbackLocale("en-US"),
				translation.TranslatedFields("Name"),
			),
		},
	})
}
