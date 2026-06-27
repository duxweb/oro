package pgsql

import (
	"context"
	"database/sql"
	"slices"
	"strings"

	"github.com/duxweb/oro"
)

type inspector struct {
	db *sql.DB
}

func (inspector inspector) Tables(ctx context.Context) ([]oro.TableInfo, error) {
	rows, err := inspector.db.QueryContext(ctx, `
		select table_name
		from information_schema.tables
		where table_schema = current_schema()
			and table_type = 'BASE TABLE'
		order by table_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tables := []oro.TableInfo{}
	for rows.Next() {
		var table oro.TableInfo
		if err := rows.Scan(&table.Name); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	return tables, rows.Err()
}

func (inspector inspector) Table(ctx context.Context, name string) (*oro.TableSpec, error) {
	primary, err := inspector.primaryColumns(ctx, name)
	if err != nil {
		return nil, err
	}

	rows, err := inspector.db.QueryContext(ctx, `
		select column_name, data_type, is_nullable
		from information_schema.columns
		where table_schema = current_schema()
			and table_name = $1
		order by ordinal_position
	`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	table := &oro.TableSpec{Name: name}
	for rows.Next() {
		var columnName string
		var dataType string
		var nullable string
		if err := rows.Scan(&columnName, &dataType, &nullable); err != nil {
			return nil, err
		}
		table.Columns = append(table.Columns, oro.ColumnSpec{
			FieldName:  columnName,
			ColumnName: columnName,
			Type:       strings.ToLower(dataType),
			Primary:    primary[columnName],
			Nullable:   nullable == "YES",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(table.Columns) == 0 {
		return nil, nil
	}
	indexes, err := inspector.Indexes(ctx, name)
	if err != nil {
		return nil, err
	}
	table.Indexes = indexes
	return table, nil
}

func (inspector inspector) Indexes(ctx context.Context, table string) ([]oro.IndexSpec, error) {
	rows, err := inspector.db.QueryContext(ctx, `
		select index_class.relname, index_info.indisunique, access_method.amname, pg_get_indexdef(index_info.indexrelid), pg_get_indexdef(index_info.indexrelid, key_order.ordinality::int, true)
		from pg_index index_info
		join pg_class table_class on table_class.oid = index_info.indrelid
		join pg_namespace namespace on namespace.oid = table_class.relnamespace
		join pg_class index_class on index_class.oid = index_info.indexrelid
		join pg_am access_method on access_method.oid = index_class.relam
		join lateral unnest(index_info.indkey::int2[]) with ordinality as key_order(attnum, ordinality) on true
		where namespace.nspname = current_schema()
			and table_class.relname = $1
			and not index_info.indisprimary
			and key_order.attnum > 0
		order by index_class.relname, key_order.ordinality
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexByName := map[string]*oro.IndexSpec{}
	order := []string{}
	for rows.Next() {
		var name string
		var unique bool
		var accessMethod string
		var definition string
		var field string
		if err := rows.Scan(&name, &unique, &accessMethod, &definition, &field); err != nil {
			return nil, err
		}
		index := indexByName[name]
		if index == nil {
			index = &oro.IndexSpec{Name: name, Unique: unique, FullText: accessMethod == "gin"}
			indexByName[name] = index
			order = append(order, name)
		}
		if index.FullText {
			index.Fields = pgFullTextIndexFields(definition)
			continue
		}
		index.Fields = append(index.Fields, field)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	indexes := make([]oro.IndexSpec, 0, len(order))
	for _, name := range order {
		indexes = append(indexes, *indexByName[name])
	}
	return indexes, nil
}

func pgFullTextIndexFields(definition string) []string {
	fields := []string{}
	remaining := definition
	for {
		start := strings.Index(remaining, "coalesce(")
		if start < 0 {
			break
		}
		remaining = remaining[start+len("coalesce("):]
		end := strings.Index(remaining, ",")
		if end < 0 {
			break
		}
		field := strings.TrimSpace(remaining[:end])
		field = strings.Trim(field, `"`)
		if field != "" && !slices.Contains(fields, field) {
			fields = append(fields, field)
		}
		remaining = remaining[end+1:]
	}
	return fields
}

func (inspector inspector) Constraints(ctx context.Context, table string) ([]oro.ConstraintSpec, error) {
	rows, err := inspector.db.QueryContext(ctx, `
		select constraint_name, constraint_type
		from information_schema.table_constraints
		where table_schema = current_schema()
			and table_name = $1
		order by constraint_name
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	constraints := []oro.ConstraintSpec{}
	for rows.Next() {
		var constraint oro.ConstraintSpec
		if err := rows.Scan(&constraint.Name, &constraint.Type); err != nil {
			return nil, err
		}
		constraints = append(constraints, constraint)
	}
	return constraints, rows.Err()
}

func (inspector inspector) primaryColumns(ctx context.Context, table string) (map[string]bool, error) {
	rows, err := inspector.db.QueryContext(ctx, `
		select key_usage.column_name
		from information_schema.table_constraints constraints
		join information_schema.key_column_usage key_usage
			on key_usage.constraint_schema = constraints.constraint_schema
			and key_usage.constraint_name = constraints.constraint_name
			and key_usage.table_schema = constraints.table_schema
			and key_usage.table_name = constraints.table_name
		where constraints.table_schema = current_schema()
			and constraints.table_name = $1
			and constraints.constraint_type = 'PRIMARY KEY'
		order by key_usage.ordinal_position
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns := map[string]bool{}
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns[column] = true
	}
	return columns, rows.Err()
}
