package mysql

import (
	"context"
	"database/sql"
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
		where table_schema = database()
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
	rows, err := inspector.db.QueryContext(ctx, `
		select column_name, data_type, column_type, is_nullable, column_key
		from information_schema.columns
		where table_schema = database()
			and table_name = ?
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
		var columnType string
		var nullable string
		var columnKey sql.NullString
		if err := rows.Scan(&columnName, &dataType, &columnType, &nullable, &columnKey); err != nil {
			return nil, err
		}
		dbType := columnType
		if dbType == "" {
			dbType = dataType
		}
		table.Columns = append(table.Columns, oro.ColumnSpec{
			FieldName:  columnName,
			ColumnName: columnName,
			Type:       strings.ToLower(dbType),
			Primary:    columnKey.Valid && columnKey.String == "PRI",
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
		select index_name, non_unique, column_name, index_type
		from information_schema.statistics
		where table_schema = database()
			and table_name = ?
		order by index_name, seq_in_index
	`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexByName := map[string]*oro.IndexSpec{}
	order := []string{}
	for rows.Next() {
		var name string
		var nonUnique int
		var columnName string
		var indexType string
		if err := rows.Scan(&name, &nonUnique, &columnName, &indexType); err != nil {
			return nil, err
		}
		if name == "PRIMARY" {
			continue
		}
		index := indexByName[name]
		if index == nil {
			index = &oro.IndexSpec{Name: name, Unique: nonUnique == 0, FullText: strings.EqualFold(indexType, "FULLTEXT")}
			indexByName[name] = index
			order = append(order, name)
		}
		index.Fields = append(index.Fields, columnName)
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

func (inspector inspector) Constraints(ctx context.Context, table string) ([]oro.ConstraintSpec, error) {
	rows, err := inspector.db.QueryContext(ctx, `
		select constraint_name, constraint_type
		from information_schema.table_constraints
		where table_schema = database()
			and table_name = ?
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
