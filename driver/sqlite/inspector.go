package sqlite

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
	rows, err := inspector.db.QueryContext(ctx, "select name from sqlite_master where type = 'table' and name not like 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []oro.TableInfo
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
	rows, err := inspector.db.QueryContext(ctx, `pragma table_info(`+quoteIdent(name)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	table := &oro.TableSpec{Name: name}
	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primary int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primary); err != nil {
			return nil, err
		}
		table.Columns = append(table.Columns, oro.ColumnSpec{
			FieldName:  columnName,
			ColumnName: columnName,
			Type:       strings.ToLower(columnType),
			Primary:    primary > 0,
			Nullable:   notNull == 0,
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
	rows, err := inspector.db.QueryContext(ctx, `pragma index_list(`+quoteIdent(table)+`)`)
	if err != nil {
		return nil, err
	}

	type indexRow struct {
		name   string
		unique bool
	}
	indexRows := []indexRow{}
	for rows.Next() {
		var seq int
		var name string
		var unique int
		var origin string
		var partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		if strings.HasPrefix(name, "sqlite_autoindex_") {
			continue
		}
		indexRows = append(indexRows, indexRow{
			name:   name,
			unique: unique > 0,
		})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}

	indexes := make([]oro.IndexSpec, 0, len(indexRows))
	for _, row := range indexRows {
		fields, err := inspector.indexFields(ctx, row.name)
		if err != nil {
			return nil, err
		}
		indexes = append(indexes, oro.IndexSpec{
			Name:   row.name,
			Fields: fields,
			Unique: row.unique,
		})
	}
	return indexes, nil
}

func (inspector inspector) Constraints(ctx context.Context, table string) ([]oro.ConstraintSpec, error) {
	return nil, nil
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func (inspector inspector) indexFields(ctx context.Context, name string) ([]string, error) {
	rows, err := inspector.db.QueryContext(ctx, `pragma index_info(`+quoteIdent(name)+`)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fields := []string{}
	for rows.Next() {
		var seqno int
		var cid int
		var columnName string
		if err := rows.Scan(&seqno, &cid, &columnName); err != nil {
			return nil, err
		}
		fields = append(fields, columnName)
	}
	return fields, rows.Err()
}
