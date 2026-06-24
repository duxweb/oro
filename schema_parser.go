package oro

import (
	"reflect"
	"strings"

	"github.com/duxweb/oro/internal/schemautil"
)

type schemaParser struct{}

func (schemaParser) Parse(model any) (*ModelSchema, error) {
	typ := modelType(model)
	if typ.Kind() != reflect.Struct {
		return nil, &Error{Op: "schema.parse", Kind: ErrInvalidArgument}
	}

	builder := NewSchemaBuilder()
	if definer, ok := reflect.New(typ).Interface().(Definer); ok {
		definer.Define(builder)
	} else if definer, ok := reflect.Zero(typ).Interface().(Definer); ok {
		definer.Define(builder)
	}

	schema := &ModelSchema{
		Name:         typ.Name(),
		Table:        builder.table,
		Type:         typ,
		Connection:   builder.connection,
		ShardGroup:   builder.shardGroup,
		ShardFields:  append([]string(nil), builder.shardFields...),
		TenantFields: append([]string(nil), builder.tenantFields...),
		NoTenant:     builder.noTenant,
		FieldByGo:    map[string]FieldSchema{},
		FieldByDB:    map[string]FieldSchema{},
	}
	if schema.Table == "" {
		schema.Table = Snake(typ.Name()) + "s"
	}

	for index := 0; index < typ.NumField(); index++ {
		structField := typ.Field(index)
		if !structField.IsExported() {
			continue
		}
		if structField.Anonymous && structField.Type == reflect.TypeOf(Model{}) {
			if err := addModelFields(schema, structField.Index); err != nil {
				return nil, err
			}
			continue
		}

		fieldBuilder := builder.fields[structField.Name]
		if fieldBuilder != nil && fieldBuilder.typeConflict {
			return nil, &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: schema.Name, Field: structField.Name}
		}
		if fieldBuilder != nil && fieldBuilder.ignore {
			continue
		}

		field := FieldSchema{
			Name:     structField.Name,
			Column:   Snake(structField.Name),
			Type:     structField.Type.String(),
			Index:    append([]int(nil), structField.Index...),
			Nullable: true,
		}
		if fieldBuilder != nil {
			if fieldBuilder.column != "" {
				field.Column = fieldBuilder.column
			}
			if fieldBuilder.fieldTyp != "" {
				field.Type = fieldBuilder.fieldTyp
			}
			if fieldBuilder.nullable != nil {
				field.Nullable = *fieldBuilder.nullable
			}
			field.Size = fieldBuilder.size
			field.SizeSet = fieldBuilder.sizeSet
			field.Precision = fieldBuilder.precision
			field.Scale = fieldBuilder.scale
			field.Default = fieldBuilder.defaultVal
			field.EnumValues = append([]string(nil), fieldBuilder.enumValues...)
			field.Comment = fieldBuilder.comment
			field.Primary = fieldBuilder.primary
			field.Ignore = fieldBuilder.ignore
			field.Virtual = fieldBuilder.virtual
			field.Hidden = fieldBuilder.hidden
			field.Optimistic = fieldBuilder.optimistic
		}
		if err := validateFieldSchema(schema.Name, field, structField.Type); err != nil {
			return nil, err
		}
		if err := addField(schema, field); err != nil {
			return nil, err
		}
		if fieldBuilder != nil {
			if fieldBuilder.index != "" {
				addIndex(schema, builderIndexName(schema.Table, field.Column, fieldBuilder.index, false), []string{field.Column}, false)
			}
			if fieldBuilder.unique != "" {
				addIndex(schema, builderIndexName(schema.Table, field.Column, fieldBuilder.unique, true), []string{field.Column}, true)
			}
			if fieldBuilder.fullText != "" {
				addFullTextIndex(schema, builderFullTextIndexName(schema.Table, field.Column, fieldBuilder.fullText), []string{field.Column})
			}
		}
	}

	for fieldName := range builder.fields {
		if _, ok := fieldByName(typ, fieldName); !ok {
			return nil, &Error{Op: "schema.parse", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
	}
	for _, fieldName := range builder.shardFields {
		if _, ok := schema.FieldByGo[fieldName]; !ok {
			return nil, &Error{Op: "schema.parse", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
	}

	for _, index := range builder.indexes {
		fields, err := indexColumns(schema, index.Fields)
		if err != nil {
			return nil, err
		}
		name := index.Name
		if name == "" {
			if index.FullText {
				name = defaultCompositeFullTextIndexName(schema.Table, fields)
			} else {
				name = defaultCompositeIndexName(schema.Table, fields, index.Unique)
			}
		}
		if index.FullText {
			addFullTextIndex(schema, name, fields)
			continue
		}
		addIndex(schema, name, fields, index.Unique)
	}
	schema.DefaultSelect = defaultSelectColumns(schema)
	schema.DefaultExprs = defaultSelectExprs(schema.DefaultSelect)

	return schema, nil
}

func addModelFields(schema *ModelSchema, baseIndex []int) error {
	for _, field := range []FieldSchema{
		{Name: "ID", Column: "id", Type: "uint64", Index: fieldIndex(baseIndex, 0), Primary: true},
		{Name: "CreatedAt", Column: "created_at", Type: "time.Time", Index: fieldIndex(baseIndex, 1), Nullable: true, AutoCreate: true},
		{Name: "UpdatedAt", Column: "updated_at", Type: "time.Time", Index: fieldIndex(baseIndex, 2), Nullable: true, AutoUpdate: true},
		{Name: "DeletedAt", Column: "deleted_at", Type: "oro.Null[time.Time]", Index: fieldIndex(baseIndex, 3), Nullable: true, SoftDelete: true},
	} {
		if err := addField(schema, field); err != nil {
			return err
		}
	}
	return nil
}

func fieldIndex(base []int, index int) []int {
	out := append([]int(nil), base...)
	return append(out, index)
}

func addField(schema *ModelSchema, field FieldSchema) error {
	if _, ok := schema.FieldByGo[field.Name]; ok {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: schema.Name, Field: field.Name}
	}
	if _, ok := schema.FieldByDB[field.Column]; ok {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: schema.Name, Field: field.Name}
	}
	schema.Fields = append(schema.Fields, field)
	schema.FieldByGo[field.Name] = field
	schema.FieldByDB[field.Column] = field
	if field.Primary {
		schema.Primary = append(schema.Primary, field.Name)
	}
	return nil
}

func defaultSelectColumns(schema *ModelSchema) []string {
	columns := make([]string, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Hidden || field.Ignore || field.Virtual {
			continue
		}
		columns = append(columns, field.Column)
	}
	return columns
}

func defaultSelectExprs(columns []string) []SelectExpr {
	exprs := make([]SelectExpr, 0, len(columns))
	for _, column := range columns {
		exprs = append(exprs, SelectExpr{Expr: column})
	}
	return exprs
}

func validateFieldSchema(modelName string, field FieldSchema, goType reflect.Type) error {
	if field.SizeSet && field.Size <= 0 {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
	}
	if field.Size > 0 && !isSizableFieldType(field.Type) {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
	}
	if field.Precision < 0 || field.Scale < 0 || (field.Precision > 0 && field.Scale > field.Precision) {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
	}
	if field.Type == "decimal" && field.Precision <= 0 {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
	}
	if field.Type == "decimal" && !isDecimalGoType(goType) {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
	}
	if field.Precision > 0 && field.Type != "decimal" && !strings.Contains(field.Type, "Decimal") {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
	}
	if field.Optimistic && !isIntegerFieldType(field.Type) {
		return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
	}
	if field.Type == "enum" {
		if len(field.EnumValues) == 0 {
			return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
		}
		if field.Default != nil && field.Default.Expr == "" {
			defaultValue, ok := field.Default.Value.(string)
			if !ok || !containsString(field.EnumValues, defaultValue) {
				return &Error{Op: "schema.parse", Kind: ErrInvalidArgument, Model: modelName, Field: field.Name}
			}
		}
	}
	return nil
}

func fieldByName(typ reflect.Type, name string) (reflect.StructField, bool) {
	return schemautil.FieldByName(typ, name)
}

func isBaseModelField(name string) bool {
	return schemautil.IsBaseModelField(name)
}

func isDecimalGoType(goType reflect.Type) bool {
	return schemautil.IsDecimalGoType(goType)
}

func isSizableFieldType(fieldType string) bool {
	return schemautil.IsSizableFieldType(fieldType)
}

func containsString(values []string, value string) bool {
	return schemautil.ContainsString(values, value)
}

func isIntegerFieldType(fieldType string) bool {
	return schemautil.IsIntegerFieldType(fieldType)
}

func addIndex(schema *ModelSchema, name string, fields []string, unique bool) {
	schema.Indexes = append(schema.Indexes, IndexSpec{Name: name, Fields: fields, Unique: unique})
}

func addFullTextIndex(schema *ModelSchema, name string, fields []string) {
	schema.Indexes = append(schema.Indexes, IndexSpec{Name: name, Fields: fields, FullText: true})
}

func indexColumns(schema *ModelSchema, fields []string) ([]string, error) {
	columns := make([]string, 0, len(fields))
	for _, fieldName := range fields {
		field, ok := schema.FieldByGo[fieldName]
		if !ok {
			return nil, &Error{Op: "schema.parse", Kind: ErrUnknownField, Model: schema.Name, Field: fieldName}
		}
		columns = append(columns, field.Column)
	}
	return columns, nil
}

func builderIndexName(table string, column string, name string, unique bool) string {
	return schemautil.IndexName(table, column, name, defaultIndexMarker, unique)
}

func builderFullTextIndexName(table string, column string, name string) string {
	return schemautil.FullTextIndexName(table, column, name, defaultIndexMarker)
}

func defaultCompositeIndexName(table string, columns []string, unique bool) string {
	return schemautil.CompositeIndexName(table, columns, unique)
}

func defaultCompositeFullTextIndexName(table string, columns []string) string {
	return schemautil.CompositeFullTextIndexName(table, columns)
}
