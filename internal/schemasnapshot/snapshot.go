package schemasnapshot

import (
	"database/sql"
	"time"

	"github.com/duxweb/oro/internal/meta"
	"github.com/duxweb/oro/internal/queryast"
	internaltypes "github.com/duxweb/oro/internal/types"
)

const Table = "oro_schema"

type Dialect interface {
	QuoteIdent(name string) string
	Placeholder(index int) string
	DataType(column meta.ColumnSpec) (string, error)
}

type Snapshot struct {
	Model   string
	Table   string
	Columns []meta.ColumnSpec
}

func CreateTableSQL(dialect Dialect, tableName string) (queryast.CompiledSQL, error) {
	if tableName == "" {
		tableName = Table
	}
	textType, err := dialect.DataType(meta.ColumnSpec{ColumnName: "model", Type: "string", Nullable: false})
	if err != nil {
		return queryast.CompiledSQL{}, err
	}
	timeType, err := dialect.DataType(meta.ColumnSpec{ColumnName: "synced_at", Type: "time.Time", Nullable: false})
	if err != nil {
		return queryast.CompiledSQL{}, err
	}

	return queryast.CompiledSQL{SQL: "create table if not exists " + dialect.QuoteIdent(tableName) + " (" +
		dialect.QuoteIdent("model") + " " + textType + " not null, " +
		dialect.QuoteIdent("table_name") + " " + textType + " not null, " +
		dialect.QuoteIdent("field_name") + " " + textType + " not null, " +
		dialect.QuoteIdent("column_name") + " " + textType + " not null, " +
		dialect.QuoteIdent("logical_type") + " " + textType + " not null, " +
		dialect.QuoteIdent("nullable") + " integer not null, " +
		dialect.QuoteIdent("primary_key") + " integer not null, " +
		dialect.QuoteIdent("synced_at") + " " + timeType + " not null, " +
		"primary key (" + dialect.QuoteIdent("model") + ", " + dialect.QuoteIdent("field_name") + ")" +
		")"}, nil
}

func SelectSQL(dialect Dialect, tableName string, model string) queryast.CompiledSQL {
	if tableName == "" {
		tableName = Table
	}
	return queryast.CompiledSQL{
		SQL: "select " +
			dialect.QuoteIdent("model") + ", " +
			dialect.QuoteIdent("table_name") + ", " +
			dialect.QuoteIdent("field_name") + ", " +
			dialect.QuoteIdent("column_name") + ", " +
			dialect.QuoteIdent("logical_type") + ", " +
			dialect.QuoteIdent("nullable") + ", " +
			dialect.QuoteIdent("primary_key") +
			" from " + dialect.QuoteIdent(tableName) +
			" where " + dialect.QuoteIdent("model") + " = " + dialect.Placeholder(1) +
			" order by " + dialect.QuoteIdent("field_name"),
		Args: []any{model},
	}
}

func DeleteSQL(dialect Dialect, tableName string, model string) queryast.CompiledSQL {
	if tableName == "" {
		tableName = Table
	}
	return queryast.CompiledSQL{
		SQL:  "delete from " + dialect.QuoteIdent(tableName) + " where " + dialect.QuoteIdent("model") + " = " + dialect.Placeholder(1),
		Args: []any{model},
	}
}

func InsertSQL(dialect Dialect, tableName string, model string, table meta.TableSpec, column meta.ColumnSpec, syncedAt time.Time) queryast.CompiledSQL {
	if tableName == "" {
		tableName = Table
	}
	return queryast.CompiledSQL{
		SQL: "insert into " + dialect.QuoteIdent(tableName) +
			" (" +
			dialect.QuoteIdent("model") + ", " +
			dialect.QuoteIdent("table_name") + ", " +
			dialect.QuoteIdent("field_name") + ", " +
			dialect.QuoteIdent("column_name") + ", " +
			dialect.QuoteIdent("logical_type") + ", " +
			dialect.QuoteIdent("nullable") + ", " +
			dialect.QuoteIdent("primary_key") + ", " +
			dialect.QuoteIdent("synced_at") +
			") values (" +
			dialect.Placeholder(1) + ", " +
			dialect.Placeholder(2) + ", " +
			dialect.Placeholder(3) + ", " +
			dialect.Placeholder(4) + ", " +
			dialect.Placeholder(5) + ", " +
			dialect.Placeholder(6) + ", " +
			dialect.Placeholder(7) + ", " +
			dialect.Placeholder(8) +
			")",
		Args: []any{
			model,
			table.Name,
			column.FieldName,
			column.ColumnName,
			column.Type,
			BoolInt(column.Nullable),
			BoolInt(column.Primary),
			syncedAt,
		},
	}
}

func FromRows(model string, rows []internaltypes.Map) *Snapshot {
	if len(rows) == 0 {
		return nil
	}
	snapshot := &Snapshot{Model: model}
	for _, row := range rows {
		if snapshot.Table == "" {
			if tableName, ok := row["table_name"].(string); ok {
				snapshot.Table = tableName
			}
		}
		fieldName, _ := row["field_name"].(string)
		columnName, _ := row["column_name"].(string)
		logicalType, _ := row["logical_type"].(string)
		snapshot.Columns = append(snapshot.Columns, meta.ColumnSpec{
			FieldName:  fieldName,
			ColumnName: columnName,
			Type:       logicalType,
			Nullable:   Truthy(row["nullable"]),
			Primary:    Truthy(row["primary_key"]),
		})
	}
	return snapshot
}

func TableSpec(snapshot *Snapshot) *meta.TableSpec {
	if snapshot == nil {
		return nil
	}
	return &meta.TableSpec{
		Name:    snapshot.Table,
		Columns: append([]meta.ColumnSpec(nil), snapshot.Columns...),
	}
}

func BoolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func Truthy(value any) bool {
	switch typedValue := value.(type) {
	case bool:
		return typedValue
	case int:
		return typedValue != 0
	case int64:
		return typedValue != 0
	case uint64:
		return typedValue != 0
	case []byte:
		return len(typedValue) > 0 && typedValue[0] != '0'
	case string:
		return typedValue != "" && typedValue != "0"
	case sql.NullBool:
		return typedValue.Valid && typedValue.Bool
	default:
		return false
	}
}
