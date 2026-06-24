package schemameta

import "github.com/duxweb/oro/internal/meta"

func TableSpecFromSchema(schema *meta.ModelSchema) meta.TableSpec {
	table := meta.TableSpec{
		Name:    schema.Table,
		Columns: make([]meta.ColumnSpec, 0, len(schema.Fields)),
		Indexes: make([]meta.IndexSpec, 0, len(schema.Indexes)),
	}
	for _, field := range schema.Fields {
		if field.Ignore || field.Virtual {
			continue
		}
		table.Columns = append(table.Columns, meta.ColumnSpec{
			FieldName:  field.Name,
			ColumnName: field.Column,
			Type:       field.Type,
			Size:       field.Size,
			SizeSet:    field.SizeSet,
			Precision:  field.Precision,
			Scale:      field.Scale,
			Default:    field.Default,
			EnumValues: append([]string(nil), field.EnumValues...),
			Comment:    field.Comment,
			Primary:    field.Primary,
			Nullable:   field.Nullable,
		})
	}
	table.Indexes = append(table.Indexes, schema.Indexes...)
	return table
}
