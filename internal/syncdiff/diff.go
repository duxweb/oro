package syncdiff

import (
	"strings"

	"github.com/duxweb/oro/internal/meta"
)

func Diff(current *meta.TableSpec, target meta.TableSpec) []meta.SchemaChange {
	return DiffWithSnapshot(current, target, nil)
}

func DiffWithSnapshot(current *meta.TableSpec, target meta.TableSpec, snapshot *meta.TableSpec) []meta.SchemaChange {
	if current == nil || len(current.Columns) == 0 {
		changes := []meta.SchemaChange{{
			Kind:  meta.SchemaCreateTable,
			Table: target,
		}}
		for _, index := range target.Indexes {
			changes = append(changes, meta.SchemaChange{
				Kind:  meta.SchemaCreateIndex,
				Table: meta.TableSpec{Name: target.Name},
				Index: index,
			})
		}
		return changes
	}

	renameChanges, renamedCurrent, renamedTarget, ambiguous := detectRenameChanges(current, target, snapshot)
	if ambiguous {
		return []meta.SchemaChange{{
			Kind:  meta.SchemaUnsafeChange,
			Table: meta.TableSpec{Name: target.Name},
		}}
	}

	currentColumns := map[string]meta.ColumnSpec{}
	for _, column := range current.Columns {
		currentColumns[column.ColumnName] = column
	}
	snapshotColumns := map[string]meta.ColumnSpec{}
	if snapshot != nil {
		for _, column := range snapshot.Columns {
			snapshotColumns[column.ColumnName] = column
		}
	}

	changes := make([]meta.SchemaChange, 0)
	changes = append(changes, renameChanges...)
	for _, column := range target.Columns {
		if renamedTarget[column.ColumnName] {
			continue
		}
		currentColumn, exists := currentColumns[column.ColumnName]
		if !exists {
			changes = append(changes, meta.SchemaChange{
				Kind:   meta.SchemaAddColumn,
				Table:  meta.TableSpec{Name: target.Name},
				Column: column,
			})
			continue
		}
		logicalCurrent := currentColumn
		if snapshotColumn, ok := snapshotColumns[column.ColumnName]; ok {
			logicalCurrent.Type = snapshotColumn.Type
		}
		if unsafeColumnChange(logicalCurrent, column) {
			changes = append(changes, meta.SchemaChange{
				Kind:    meta.SchemaUnsafeChange,
				Table:   meta.TableSpec{Name: target.Name},
				Column:  column,
				Current: currentColumn,
			})
		}
	}
	targetColumns := map[string]bool{}
	for _, column := range target.Columns {
		targetColumns[column.ColumnName] = true
	}
	for _, column := range current.Columns {
		if renamedCurrent[column.ColumnName] {
			continue
		}
		if !targetColumns[column.ColumnName] {
			changes = append(changes, meta.SchemaChange{
				Kind:    meta.SchemaUnsafeChange,
				Table:   meta.TableSpec{Name: target.Name},
				Current: column,
			})
		}
	}
	currentIndexes := map[string]meta.IndexSpec{}
	for _, index := range current.Indexes {
		currentIndexes[index.Name] = index
	}
	for _, index := range target.Indexes {
		currentIndex, ok := currentIndexes[index.Name]
		if !ok {
			changes = append(changes, meta.SchemaChange{
				Kind:  meta.SchemaCreateIndex,
				Table: meta.TableSpec{Name: target.Name},
				Index: index,
			})
			continue
		}
		if !sameIndexSpec(currentIndex, index) {
			changes = append(changes, meta.SchemaChange{
				Kind:  meta.SchemaUnsafeChange,
				Table: meta.TableSpec{Name: target.Name},
				Index: index,
			})
		}
	}
	return changes
}

func detectRenameChanges(current *meta.TableSpec, target meta.TableSpec, snapshot *meta.TableSpec) ([]meta.SchemaChange, map[string]bool, map[string]bool, bool) {
	renamedCurrent := map[string]bool{}
	renamedTarget := map[string]bool{}
	if snapshot == nil || len(snapshot.Columns) == 0 {
		return nil, renamedCurrent, renamedTarget, false
	}

	currentByColumn := map[string]meta.ColumnSpec{}
	for _, column := range current.Columns {
		currentByColumn[column.ColumnName] = column
	}
	targetByField := map[string]meta.ColumnSpec{}
	targetByColumn := map[string]meta.ColumnSpec{}
	for _, column := range target.Columns {
		targetByField[column.FieldName] = column
		targetByColumn[column.ColumnName] = column
	}

	candidates := []meta.SchemaChange{}
	for _, oldColumn := range snapshot.Columns {
		if oldColumn.FieldName == "" {
			continue
		}
		currentColumn, currentExists := currentByColumn[oldColumn.ColumnName]
		targetColumn, targetFieldExists := targetByField[oldColumn.FieldName]
		if !currentExists || !targetFieldExists {
			continue
		}
		if _, targetColumnAlreadyExists := currentByColumn[targetColumn.ColumnName]; targetColumnAlreadyExists {
			continue
		}
		if oldColumn.ColumnName == targetColumn.ColumnName {
			continue
		}
		if !compatibleColumnType(currentColumn.Type, targetColumn.Type) {
			continue
		}
		candidates = append(candidates, meta.SchemaChange{
			Kind:    meta.SchemaRenameColumn,
			Table:   meta.TableSpec{Name: target.Name},
			Current: currentColumn,
			Column:  targetColumn,
		})
	}

	if len(candidates) > 1 {
		return nil, renamedCurrent, renamedTarget, true
	}
	if len(candidates) == 0 {
		return nil, renamedCurrent, renamedTarget, false
	}

	change := candidates[0]
	if _, ok := targetByColumn[change.Column.ColumnName]; !ok {
		return nil, renamedCurrent, renamedTarget, false
	}
	renamedCurrent[change.Current.ColumnName] = true
	renamedTarget[change.Column.ColumnName] = true
	return candidates, renamedCurrent, renamedTarget, false
}

func unsafeColumnChange(current meta.ColumnSpec, target meta.ColumnSpec) bool {
	if current.Primary != target.Primary {
		return true
	}
	if !target.Primary && current.Nullable && !target.Nullable {
		return true
	}
	if !compatibleColumnType(current.Type, target.Type) {
		return true
	}
	return false
}

func compatibleColumnType(current string, target string) bool {
	currentKind := columnTypeKind(current)
	targetKind := columnTypeKind(target)
	if currentKind == "" || targetKind == "" {
		return current == target
	}
	return currentKind == targetKind
}

func columnTypeKind(typ string) string {
	typ = strings.ToLower(strings.TrimSpace(typ))
	if typ == "" {
		return ""
	}
	switch typ {
	case "bool", "boolean", "tinyint(1)":
		return "bool"
	case "point":
		return "point"
	case "json", "jsonb", "oro.jsonraw":
		return "json"
	case "string_array", "int_array":
		return "json"
	}
	if strings.Contains(typ, "time.time") || typ == "datetime" || strings.HasPrefix(typ, "timestamp") || typ == "date" || strings.HasPrefix(typ, "time ") {
		return "time"
	}
	if strings.Contains(typ, "json") {
		return "json"
	}
	if strings.Contains(typ, "decimal") || strings.Contains(typ, "numeric") {
		return "decimal"
	}
	if strings.Contains(typ, "blob") || strings.Contains(typ, "bytea") || strings.Contains(typ, "binary") {
		return "binary"
	}
	if strings.Contains(typ, "char") || strings.Contains(typ, "text") || strings.Contains(typ, "clob") || strings.Contains(typ, "enum") ||
		strings.Contains(typ, "email") || strings.Contains(typ, "url") || strings.Contains(typ, "uuid") ||
		strings.Contains(typ, "ip") || strings.Contains(typ, "mac") || strings.Contains(typ, "phone") ||
		strings.Contains(typ, "slug") || strings.Contains(typ, "color") {
		return "string"
	}
	if strings.Contains(typ, "uint") || strings.Contains(typ, "int") || strings.Contains(typ, "serial") {
		return "integer"
	}
	if strings.Contains(typ, "float") || strings.Contains(typ, "double") || strings.Contains(typ, "real") {
		return "float"
	}
	return typ
}

func sameIndexSpec(left meta.IndexSpec, right meta.IndexSpec) bool {
	if left.Name != right.Name || left.Unique != right.Unique || left.FullText != right.FullText || len(left.Fields) != len(right.Fields) {
		return false
	}
	for index := range left.Fields {
		if left.Fields[index] != right.Fields[index] {
			return false
		}
	}
	return true
}
