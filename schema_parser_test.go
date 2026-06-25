package oro

import (
	"errors"
	"testing"
	"time"
)

type testProduct struct {
	Model
	TestSoftDeleteFields
	Code  string
	Price uint
	State string
	Meta  JSONRaw
}

type TestSoftDeleteFields struct {
	DeletedAt Null[time.Time]
}

func (TestSoftDeleteFields) OroEmbeddedFields() {}

func (TestSoftDeleteFields) DefineOroFields(s *SchemaBuilder) {
	s.Field("DeletedAt").Column("deleted_at").Timestamp().SoftDelete()
}

func (testProduct) Define(s *SchemaBuilder) {
	s.Table("products")
	s.Field("Code").Column("product_code").String().Unique()
	s.Field("Price").Decimal(12, 2).Default(0).NotNull()
	s.Field("State").Enum("draft", "active").Default("draft").Size(32)
	s.Field("Meta").JSON().Nullable().FullText()
	s.Index("idx_products_state_price", "State", "Price")
	s.FullText("ft_products_code_state", "Code", "State")
}

func TestSchemaParserParse(t *testing.T) {
	schema, err := schemaParser{}.Parse(testProduct{})
	if err != nil {
		t.Fatal(err)
	}

	if schema.Table != "products" {
		t.Fatalf("got table %q", schema.Table)
	}
	if schema.FieldByGo["Code"].Column != "product_code" {
		t.Fatalf("got column %q", schema.FieldByGo["Code"].Column)
	}
	if len(schema.Primary) == 0 || schema.Primary[0] != "ID" {
		t.Fatalf("expected ID primary, got %#v", schema.Primary)
	}
	if len(schema.Indexes) != 5 {
		t.Fatalf("expected five indexes, got %#v", schema.Indexes)
	}
	if _, ok := schema.FieldByGo["DeletedAt"]; !ok {
		t.Fatal("expected soft delete field")
	}
	price := schema.FieldByGo["Price"]
	if price.Type != "decimal" || price.Precision != 12 || price.Scale != 2 || price.Nullable {
		t.Fatalf("unexpected price field %#v", price)
	}
	if price.Default == nil || price.Default.Value != 0 {
		t.Fatalf("unexpected price default %#v", price.Default)
	}
	state := schema.FieldByGo["State"]
	if state.Type != "enum" || state.Size != 32 || len(state.EnumValues) != 2 {
		t.Fatalf("unexpected state field %#v", state)
	}
	if schema.FieldByGo["Meta"].Type != "json" || !schema.FieldByGo["Meta"].Nullable {
		t.Fatalf("unexpected meta field %#v", schema.FieldByGo["Meta"])
	}
	indexes := map[string]IndexSpec{}
	for _, index := range schema.Indexes {
		indexes[index.Name] = index
	}
	if index := indexes["uk_products_product_code"]; !index.Unique {
		t.Fatalf("unexpected unique index %#v", index)
	}
	if index := indexes["idx_products_state_price"]; len(index.Fields) != 2 {
		t.Fatalf("unexpected composite index %#v", index)
	}
	if index := indexes["ft_products_meta"]; !index.FullText || len(index.Fields) != 1 || index.Fields[0] != "meta" {
		t.Fatalf("unexpected field fulltext index %#v", index)
	}
	if index := indexes["ft_products_code_state"]; !index.FullText || len(index.Fields) != 2 {
		t.Fatalf("unexpected composite fulltext index %#v", index)
	}
	if index := indexes["idx_products_deleted_at"]; len(index.Fields) != 1 || index.Fields[0] != "deleted_at" {
		t.Fatalf("unexpected soft delete index %#v", index)
	}
}

type testPlainModel struct {
	Model
	Code string
}

func TestSchemaParserModelDoesNotEnableSoftDelete(t *testing.T) {
	schema, err := schemaParser{}.Parse(testPlainModel{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.FieldByGo["DeletedAt"]; ok {
		t.Fatal("did not expect DeletedAt from base Model")
	}
	if _, ok := softDeleteField(schema); ok {
		t.Fatal("did not expect soft delete field")
	}
}

type testCustomSoftDelete struct {
	Model
	RemovedAt Null[time.Time]
}

func (testCustomSoftDelete) Define(s *SchemaBuilder) {
	s.Field("RemovedAt").Column("removed_at").SoftDelete()
}

func TestSchemaParserCustomSoftDeleteField(t *testing.T) {
	schema, err := schemaParser{}.Parse(testCustomSoftDelete{})
	if err != nil {
		t.Fatal(err)
	}
	field, ok := softDeleteField(schema)
	if !ok || field.Name != "RemovedAt" || field.Column != "removed_at" {
		t.Fatalf("unexpected soft delete field %#v", field)
	}
}

func TestSchemaParserRejectsUnknownDefineField(t *testing.T) {
	_, err := schemaParser{}.Parse(testUnknownField{})
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !errors.Is(err, ErrUnknownField) {
		t.Fatalf("expected ErrUnknownField, got %v", err)
	}
}

type testUnknownField struct {
	Code string
}

func (testUnknownField) Define(s *SchemaBuilder) {
	s.Field("Codde").String()
}

func TestSchemaParserRejectsInvalidFieldDefinitions(t *testing.T) {
	tests := []struct {
		name  string
		model any
		field string
	}{
		{name: "size on number", model: testInvalidSizeField{}, field: "Age"},
		{name: "zero size", model: testInvalidZeroSizeField{}, field: "Code"},
		{name: "decimal on string", model: testInvalidDecimalField{}, field: "Code"},
		{name: "decimal precision", model: testInvalidDecimalPrecisionField{}, field: "Price"},
		{name: "decimal scale", model: testInvalidDecimalScaleField{}, field: "Price"},
		{name: "empty enum", model: testInvalidEmptyEnumField{}, field: "State"},
		{name: "enum default", model: testInvalidEnumDefaultField{}, field: "State"},
		{name: "type conflict", model: testInvalidTypeConflictField{}, field: "Code"},
		{name: "column conflict", model: testInvalidColumnConflictField{}, field: "Name"},
		{name: "index field", model: testInvalidIndexField{}, field: "Missing"},
		{name: "optimistic lock type", model: testInvalidOptimisticLockField{}, field: "State"},
		{name: "shard field", model: testInvalidShardField{}, field: "TenantID"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := schemaParser{}.Parse(tt.model)
			if err == nil {
				t.Fatal("expected invalid field error")
			}
			if !errors.Is(err, ErrInvalidArgument) && !errors.Is(err, ErrUnknownField) {
				t.Fatalf("expected schema error, got %v", err)
			}
			var ormError *Error
			if !errors.As(err, &ormError) {
				t.Fatalf("expected *Error, got %T", err)
			}
			if ormError.Field != tt.field {
				t.Fatalf("expected field %q, got %q", tt.field, ormError.Field)
			}
		})
	}
}

func TestSchemaParserAllowsIgnoredField(t *testing.T) {
	schema, err := schemaParser{}.Parse(testIgnoredField{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.FieldByGo["Secret"]; ok {
		t.Fatal("expected ignored field to be excluded")
	}
}

func TestSchemaParserAllowsVirtualField(t *testing.T) {
	schema, err := schemaParser{}.Parse(testVirtualField{})
	if err != nil {
		t.Fatal(err)
	}
	field, ok := schema.FieldByGo["TotalCount"]
	if !ok || !field.Virtual || field.Column != "total_count" {
		t.Fatalf("unexpected virtual field %#v", field)
	}
	table := tableSpecFromSchema(schema)
	for _, column := range table.Columns {
		if column.FieldName == "TotalCount" {
			t.Fatalf("virtual field should not become column %#v", column)
		}
	}
}

type testInvalidSizeField struct {
	Age int
}

func (testInvalidSizeField) Define(s *SchemaBuilder) {
	s.Field("Age").Size(10)
}

type testInvalidZeroSizeField struct {
	Code string
}

func (testInvalidZeroSizeField) Define(s *SchemaBuilder) {
	s.Field("Code").String().Size(0)
}

type testInvalidDecimalField struct {
	Code string
}

func (testInvalidDecimalField) Define(s *SchemaBuilder) {
	s.Field("Code").Decimal(12, 2)
}

type testInvalidDecimalPrecisionField struct {
	Price uint
}

func (testInvalidDecimalPrecisionField) Define(s *SchemaBuilder) {
	s.Field("Price").Decimal(0, 0)
}

type testInvalidDecimalScaleField struct {
	Price uint
}

func (testInvalidDecimalScaleField) Define(s *SchemaBuilder) {
	s.Field("Price").Decimal(10, 11)
}

type testInvalidEmptyEnumField struct {
	State string
}

func (testInvalidEmptyEnumField) Define(s *SchemaBuilder) {
	s.Field("State").Enum()
}

type testInvalidEnumDefaultField struct {
	State string
}

func (testInvalidEnumDefaultField) Define(s *SchemaBuilder) {
	s.Field("State").Enum("draft", "active").Default("archived")
}

type testInvalidTypeConflictField struct {
	Code string
}

func (testInvalidTypeConflictField) Define(s *SchemaBuilder) {
	s.Field("Code").String().Int()
}

type testInvalidColumnConflictField struct {
	Code string
	Name string
}

func (testInvalidColumnConflictField) Define(s *SchemaBuilder) {
	s.Field("Code").Column("same")
	s.Field("Name").Column("same")
}

type testInvalidIndexField struct {
	Code string
}

func (testInvalidIndexField) Define(s *SchemaBuilder) {
	s.Index("idx_missing", "Missing")
}

type testIgnoredField struct {
	Code   string
	Secret string
}

func (testIgnoredField) Define(s *SchemaBuilder) {
	s.Field("Secret").Ignore()
}

type testVirtualField struct {
	Code       string
	TotalCount int64
}

func (testVirtualField) Define(s *SchemaBuilder) {
	s.Field("TotalCount").Virtual()
}

type testInvalidOptimisticLockField struct {
	State string
}

func (testInvalidOptimisticLockField) Define(s *SchemaBuilder) {
	s.Field("State").OptimisticLock()
}

type testInvalidShardField struct {
	ID uint64
}

func (testInvalidShardField) Define(s *SchemaBuilder) {
	s.Shard("orders", "TenantID")
}

func TestSnake(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "Product", want: "product"},
		{in: "ProductCode", want: "product_code"},
		{in: "ID", want: "id"},
		{in: "ProductID", want: "product_id"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := Snake(tt.in); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
