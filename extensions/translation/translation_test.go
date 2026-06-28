package translation_test

import (
	"context"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/translation"
	_ "modernc.org/sqlite"
)

type product struct {
	oro.Model
	translation.Fields

	Code        string
	Name        string
	Description string
}

func (product) Define(s *oro.SchemaBuilder) {
	s.Table("products")
	s.Field("Code").String().Unique()
	s.Field("Name").String().Nullable()
	s.Field("Description").Text().Nullable()
}

func openDB(t *testing.T) (*oro.DB, context.Context) {
	t.Helper()
	ctx := context.Background()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		Extensions: []oro.Extension{
			translation.Extension(
				translation.DefaultLocale("zh-CN"),
				translation.FallbackLocale("en-US"),
				translation.TranslatedFields("Name", "Description"),
			),
		},
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(ctx); err != nil {
			t.Fatalf("close: %v", err)
		}
	})
	if err := db.Register(product{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if err := db.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return db, ctx
}

func TestCreateWithTranslationValuesAndFallback(t *testing.T) {
	db, ctx := openDB(t)

	created, err := translation.Use[product](db).Create(ctx, &product{Code: "P001", Name: "Original"}, translation.Values{
		"zh-CN": oro.Map{"Name": "苹果", "Description": "红色苹果"},
		"en-US": oro.Map{"Name": "Apple", "Description": "Red apple"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Name != "苹果" {
		t.Fatalf("expected zh-CN name, got %q", created.Name)
	}
	base, err := db.Use[product]().Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find base: %v", err)
	}
	if base == nil || base.Name != "Original" || base.Description != "红色苹果" {
		t.Fatalf("expected original columns to use default locale, got %#v", base)
	}

	en, err := translation.Use[product](db).Locale("en-US").Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find en: %v", err)
	}
	if en == nil || en.Name != "Apple" || en.Description != "Red apple" {
		t.Fatalf("unexpected en product %#v", en)
	}

	jp, err := translation.Use[product](db).Locale("ja-JP").Fallback("en-US").Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find fallback: %v", err)
	}
	if jp == nil || jp.Name != "Apple" {
		t.Fatalf("expected fallback name, got %#v", jp)
	}
}

func TestCreateWithCurrentLocaleAndUpdateTranslations(t *testing.T) {
	db, ctx := openDB(t)

	created, err := translation.Use[product](db).Locale("zh-CN").Create(ctx, &product{
		Code:        "P002",
		Name:        "梨子",
		Description: "黄色梨子",
	})
	if err != nil {
		t.Fatalf("create locale: %v", err)
	}
	if created.Name != "梨子" {
		t.Fatalf("expected zh-CN value, got %q", created.Name)
	}

	if _, err := translation.Use[product](db).Where("ID", created.ID).Update(ctx, oro.Map{"Code": "P002B"}, translation.Values{
		"en-US": oro.Map{"Name": "Pear", "Description": "Yellow pear"},
	}); err != nil {
		t.Fatalf("update translations: %v", err)
	}

	zh, err := translation.Use[product](db).Locale("zh-CN").Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find zh: %v", err)
	}
	if zh == nil || zh.Name != "梨子" || zh.Description != "黄色梨子" {
		t.Fatalf("expected zh-CN to be preserved, got %#v", zh)
	}

	en, err := translation.Use[product](db).Locale("en-US").Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find en: %v", err)
	}
	if en == nil || en.Code != "P002B" || en.Name != "Pear" || en.Description != "Yellow pear" {
		t.Fatalf("unexpected en product %#v", en)
	}

	if _, err := translation.Use[product](db).Locale("en-US").WhereTrans("Name", "Pear").First(ctx); err != nil {
		t.Fatalf("where trans: %v", err)
	}
	if found, err := translation.Use[product](db).Locale("en-US").WhereTransLike("Name", "%ea%").First(ctx); err != nil || found == nil || found.Code != "P002B" {
		t.Fatalf("where trans like found=%#v err=%v", found, err)
	}

	if _, err := translation.Use[product](db).Locale("zh-CN").Where("ID", created.ID).Update(ctx, oro.Map{
		"Name": "新梨子",
	}); err != nil {
		t.Fatalf("update current locale: %v", err)
	}

	en, err = translation.Use[product](db).Locale("en-US").Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find preserved en: %v", err)
	}
	if en == nil || en.Name != "Pear" {
		t.Fatalf("expected en-US to be preserved, got %#v", en)
	}

	zh, err = translation.Use[product](db).Locale("zh-CN").Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find updated zh: %v", err)
	}
	if zh == nil || zh.Name != "新梨子" {
		t.Fatalf("expected zh-CN update, got %#v", zh)
	}

	base, err := db.Use[product]().Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find base after update: %v", err)
	}
	if base == nil || base.Name != "新梨子" {
		t.Fatalf("expected original column to track default locale, got %#v", base)
	}
}

func TestContextLocaleFallbackAndOriginalValue(t *testing.T) {
	db, ctx := openDB(t)
	ctx = translation.WithFallback(translation.WithLocale(ctx, "ja-JP"), "en-US")

	created, err := translation.Use[product](db).Create(context.Background(), &product{Code: "P004", Name: "Original"}, translation.Values{
		"zh-CN": oro.Map{"Name": "中文"},
		"en-US": oro.Map{"Name": "English"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	found, err := translation.Use[product](db).Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find with context locale: %v", err)
	}
	if found == nil || found.Name != "English" {
		t.Fatalf("expected context fallback value, got %#v", found)
	}

	missing, err := translation.Use[product](db).Locale("fr-FR").Fallback("de-DE").Find(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("find missing locales: %v", err)
	}
	if missing == nil || missing.Name != "Original" {
		t.Fatalf("expected original fallback, got %#v", missing)
	}
}

func TestTranslateHelperFallsBackToOriginal(t *testing.T) {
	item := &product{Code: "P003", Name: "Original"}
	if got := translation.Translate(item, "zh-CN", "en-US").String("Name"); got != "Original" {
		t.Fatalf("expected original fallback, got %q", got)
	}
	if err := translation.Translate(item, "zh-CN").Set("Name", "葡萄"); err != nil {
		t.Fatalf("set translation: %v", err)
	}
	if got := translation.Translate(item, "zh-CN").String("Name"); got != "葡萄" {
		t.Fatalf("expected translated value, got %q", got)
	}
}

func TestApplyTranslationOnModelQuery(t *testing.T) {
	db, ctx := openDB(t)

	created, err := db.Use[product]().Apply(translation.Write(translation.Values{
		"zh-CN": oro.Map{"Name": "香蕉"},
		"en-US": oro.Map{"Name": "Banana"},
	})).Create(ctx, &product{Code: "P005"})
	if err != nil {
		t.Fatalf("apply create: %v", err)
	}

	found, err := db.Use[product]().
		Apply(translation.Locale("en-US"), translation.WhereTrans("Name", "Banana")).
		First(ctx)
	if err != nil {
		t.Fatalf("apply where: %v", err)
	}
	if found == nil || found.ID != created.ID || found.Name != "Banana" {
		t.Fatalf("unexpected apply result %#v", found)
	}

	if _, err := db.Use[product]().
		Apply(translation.Locale("zh-CN"), translation.Write(translation.Values{"en-US": oro.Map{"Description": "Yellow banana"}})).
		Where("ID", created.ID).
		Update(ctx, oro.Map{"Name": "大香蕉"}); err != nil {
		t.Fatalf("apply update: %v", err)
	}

	updated, err := db.Use[product]().Apply(translation.Locale("zh-CN")).Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("apply find: %v", err)
	}
	if updated == nil || updated.Name != "大香蕉" {
		t.Fatalf("unexpected updated zh result %#v", updated)
	}
}

func TestApplyTranslationUpdateCanUseOnlyWriteValues(t *testing.T) {
	db, ctx := openDB(t)

	created, err := db.Use[product]().
		Apply(translation.Write(translation.Values{
			"zh-CN": oro.Map{"Name": "梨子"},
		})).
		Create(ctx, &product{Code: "P006"})
	if err != nil {
		t.Fatalf("apply create: %v", err)
	}

	affected, err := db.Use[product]().
		Apply(translation.Write(translation.Values{
			"en-US": oro.Map{"Name": "Pear"},
		})).
		Where("ID", created.ID).
		Update(ctx, nil)
	if err != nil {
		t.Fatalf("apply update only translations: %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected 1 affected row, got %d", affected)
	}

	found, err := db.Use[product]().Apply(translation.Locale("en-US")).Find(ctx, created.ID)
	if err != nil {
		t.Fatalf("find translated: %v", err)
	}
	if found == nil || found.Name != "Pear" {
		t.Fatalf("unexpected translated result %#v", found)
	}
}

func TestApplyTranslationUpsert(t *testing.T) {
	db, ctx := openDB(t)

	upserted, err := db.Use[product]().
		Apply(translation.Write(translation.Values{
			"zh-CN": oro.Map{"Name": "葡萄"},
			"en-US": oro.Map{"Name": "Grape"},
		})).
		Upsert(ctx, &product{Code: "P007"}, oro.ConflictBy("Code").Update("Name", "Translations"))
	if err != nil {
		t.Fatalf("apply upsert: %v", err)
	}
	if upserted == nil || upserted.Name != "葡萄" {
		t.Fatalf("unexpected upserted result %#v", upserted)
	}

	found, err := db.Use[product]().Apply(translation.Locale("en-US")).Find(ctx, upserted.ID)
	if err != nil {
		t.Fatalf("find translated upsert: %v", err)
	}
	if found == nil || found.Name != "Grape" {
		t.Fatalf("unexpected translated upsert result %#v", found)
	}
}
