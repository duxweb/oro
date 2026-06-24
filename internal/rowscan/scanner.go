package rowscan

import (
	"database/sql"
	"strings"

	"github.com/duxweb/oro/internal/scanconv"
	internaltypes "github.com/duxweb/oro/internal/types"
)

type Scanner struct {
	columns []string
	dbTypes []string
	values  []any
	dests   []any
}

func New(rows *sql.Rows) (Scanner, error) {
	columns, err := rows.Columns()
	if err != nil {
		return Scanner{}, err
	}
	columnTypes, _ := rows.ColumnTypes()
	dbTypes := make([]string, len(columns))
	for index := range columns {
		if index < len(columnTypes) && columnTypes[index] != nil {
			dbTypes[index] = strings.ToLower(columnTypes[index].DatabaseTypeName())
		}
	}
	values := make([]any, len(columns))
	dests := make([]any, len(columns))
	for index := range values {
		dests[index] = &values[index]
	}
	return Scanner{columns: columns, dbTypes: dbTypes, values: values, dests: dests}, nil
}

func (scanner *Scanner) Scan(rows *sql.Rows) (internaltypes.Map, error) {
	if err := rows.Scan(scanner.dests...); err != nil {
		return nil, err
	}

	row := make(internaltypes.Map, len(scanner.columns))
	for index, column := range scanner.columns {
		row[column] = scanconv.Normalize(scanner.values[index], scanner.dbTypes[index])
	}
	return row, nil
}

func All(rows *sql.Rows) ([]internaltypes.Map, error) {
	scanner, err := New(rows)
	if err != nil {
		return nil, err
	}

	results := make([]internaltypes.Map, 0)
	for rows.Next() {
		row, err := scanner.Scan(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, row)
	}

	return results, nil
}
