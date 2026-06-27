package pgsql

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/internal/queryutil"
)

type dialect struct{}

var placeholderPattern = regexp.MustCompile(`\$(\d+)`)

func (dialect) Name() string {
	return "pgsql"
}

func (dialect) Capabilities() oro.Capabilities {
	return oro.Capabilities{
		Returning:       true,
		Upsert:          true,
		Savepoint:       true,
		LockForUpdate:   true,
		LockForShare:    true,
		LockNoWait:      true,
		LockSkipLocked:  true,
		FullJoin:        true,
		JSON:            true,
		FullText:        true,
		CheckConstraint: true,
	}
}

func (dialect) QuoteIdent(name string) string {
	parts := strings.Split(name, ".")
	for index, part := range parts {
		parts[index] = `"` + strings.ReplaceAll(part, `"`, `""`) + `"`
	}
	return strings.Join(parts, ".")
}

func (dialect) Placeholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func (dialect) DataType(column oro.ColumnSpec) (string, error) {
	switch column.Type {
	case "string":
		if column.Size > 0 {
			return fmt.Sprintf("varchar(%d)", column.Size), nil
		}
		return "text", nil
	case "text":
		return "text", nil
	case "bool":
		return "boolean", nil
	case "int", "int32", "uint", "uint32":
		return "integer", nil
	case "uint64", "int64":
		return "bigint", nil
	case "decimal":
		if column.Precision <= 0 {
			return "numeric", nil
		}
		return fmt.Sprintf("numeric(%d,%d)", column.Precision, column.Scale), nil
	case "float":
		return "real", nil
	case "double":
		return "double precision", nil
	case "binary":
		return "bytea", nil
	case "json", "string_array", "int_array":
		return "jsonb", nil
	case "uuid":
		return "uuid", nil
	case "time.Time":
		return "timestamp", nil
	case "date":
		return "date", nil
	case "time":
		return "time", nil
	case "enum", "email", "url", "ip", "mac", "phone", "slug", "color":
		if column.Size > 0 {
			return fmt.Sprintf("varchar(%d)", column.Size), nil
		}
		return "text", nil
	case "point":
		return "point", nil
	default:
		if strings.Contains(column.Type, "time.Time") {
			return "timestamp", nil
		}
		if strings.Contains(column.Type, "Decimal") {
			if column.Precision > 0 {
				return fmt.Sprintf("numeric(%d,%d)", column.Precision, column.Scale), nil
			}
			return "numeric", nil
		}
		if strings.Contains(column.Type, "JSON") {
			return "jsonb", nil
		}
		return "text", nil
	}
}

func (dialect) NormalizeType(dbType string) (oro.ColumnType, error) {
	return oro.ColumnType{Logical: strings.ToLower(dbType), DBType: dbType}, nil
}

func (d dialect) Compile(stmt oro.Statement) (oro.CompiledSQL, error) {
	switch statement := stmt.(type) {
	case oro.SelectAST:
		return d.compileSelect(statement)
	case oro.InsertAST:
		return d.compileInsert(statement)
	case oro.UpdateAST:
		return d.compileUpdate(statement)
	case oro.DeleteAST:
		return d.compileDelete(statement)
	default:
		return oro.CompiledSQL{}, &oro.Error{Op: "pgsql.compile", Kind: oro.ErrUnsupported}
	}
}

func (d dialect) CompileSchema(change oro.SchemaChange) ([]oro.CompiledSQL, error) {
	switch change.Kind {
	case oro.SchemaCreateTable:
		return d.compileCreateTable(change.Table)
	case oro.SchemaAddColumn:
		sql, err := d.compileColumn(change.Column, false)
		if err != nil {
			return nil, err
		}
		statements := []oro.CompiledSQL{{SQL: "alter table " + d.QuoteIdent(change.Table.Name) + " add column " + sql}}
		statements = append(statements, d.compileColumnComment(change.Table.Name, change.Column)...)
		return statements, nil
	case oro.SchemaCreateIndex:
		return d.compileCreateIndex(change.Table.Name, change.Index)
	case oro.SchemaRenameColumn:
		if change.Current.ColumnName == "" || change.Column.ColumnName == "" {
			return nil, &oro.Error{Op: "pgsql.schema", Kind: oro.ErrInvalidArgument}
		}
		return []oro.CompiledSQL{{SQL: "alter table " + d.QuoteIdent(change.Table.Name) + " rename column " + d.QuoteIdent(change.Current.ColumnName) + " to " + d.QuoteIdent(change.Column.ColumnName)}}, nil
	case oro.SchemaUnsafeChange:
		return nil, &oro.Error{Op: "pgsql.schema", Kind: oro.ErrUnsafeSchemaChange}
	default:
		return nil, &oro.Error{Op: "pgsql.schema", Kind: oro.ErrUnsupported}
	}
}

func (d dialect) compileSelect(stmt oro.SelectAST) (oro.CompiledSQL, error) {
	selectSQL := "*"
	selectArgs := []any{}
	selectPlaceholderIndex := 1
	if len(stmt.Select) > 0 {
		parts := make([]string, 0, len(stmt.Select))
		for _, item := range stmt.Select {
			expr, itemArgs, nextIndex, err := d.compileSelectExpr(item, selectPlaceholderIndex)
			if err != nil {
				return oro.CompiledSQL{}, err
			}
			selectArgs = append(selectArgs, itemArgs...)
			selectPlaceholderIndex = nextIndex
			parts = append(parts, expr)
		}
		selectSQL = strings.Join(parts, ", ")
	}

	sourceSQL, sourceArgs, placeholderIndex, err := d.compileSelectSource(stmt, selectPlaceholderIndex)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql := "select " + selectSQL + " from " + sourceSQL
	joins, joinArgs, placeholderIndex, err := d.compileJoins(stmt.Joins, placeholderIndex)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql += joins
	args := append(selectArgs, sourceArgs...)
	args = append(args, joinArgs...)
	where, whereArgs, err := d.compileWhere(stmt.Where, placeholderIndex)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	if where != "" {
		sql += " where " + where
		args = append(args, whereArgs...)
	}
	group := d.compileGroup(stmt.Group)
	if group != "" {
		sql += " group by " + group
	}
	having, havingArgs, err := d.compileWhere(stmt.Having, len(args)+1)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	if having != "" {
		sql += " having " + having
		args = append(args, havingArgs...)
	}
	order, orderArgs := d.compileOrder(stmt.Order)
	if order != "" {
		sql += " order by " + order
		args = append(args, orderArgs...)
	}
	if stmt.Limit != nil {
		if *stmt.Limit < 0 {
			return oro.CompiledSQL{}, &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument, Field: "Limit"}
		}
		sql += fmt.Sprintf(" limit %d", *stmt.Limit)
	}
	if stmt.Offset != nil {
		if *stmt.Offset < 0 {
			return oro.CompiledSQL{}, &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument, Field: "Offset"}
		}
		sql += fmt.Sprintf(" offset %d", *stmt.Offset)
	}
	lockSQL, err := d.compileLock(stmt.Lock)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql += lockSQL
	return oro.CompiledSQL{SQL: sql, Args: args}, nil
}

func (d dialect) compileSelectExpr(item oro.SelectExpr, start int) (string, []any, int, error) {
	if item.Source != nil {
		expr, args, nextIndex, err := d.compileScalarSource(*item.Source, start, "pgsql.select")
		if err != nil {
			return "", nil, start, err
		}
		if item.Alias == "" {
			return "", nil, start, &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument}
		}
		return expr + " as " + d.QuoteIdent(item.Alias), args, nextIndex, nil
	}
	if item.Expr == "__oro_relation_exists__" {
		if len(item.Args) != 1 {
			return "", nil, start, &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument}
		}
		source, ok := item.Args[0].(oro.SourceAST)
		if !ok {
			return "", nil, start, &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument}
		}
		expr, args, nextIndex, err := d.compileScalarSource(source, start, "pgsql.select")
		if err != nil {
			return "", nil, start, err
		}
		if item.Alias == "" {
			return "", nil, start, &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument}
		}
		return "exists " + expr + " as " + d.QuoteIdent(item.Alias), args, nextIndex, nil
	}
	expr := item.Expr
	args := []any{}
	placeholderIndex := start
	if item.Expr == "__oro_fulltext_score__" {
		scoreSQL, scoreArgs, nextIndex, err := d.compileFullTextScore(item, placeholderIndex)
		if err != nil {
			return "", nil, start, err
		}
		expr = scoreSQL
		args = append(args, scoreArgs...)
		placeholderIndex = nextIndex
	} else if item.Expr == "__oro_aggregate__" {
		aggregateSQL, err := d.compileAggregateSelect(item)
		if err != nil {
			return "", nil, start, err
		}
		expr = aggregateSQL
	} else if !item.Raw {
		expr = d.QuoteIdent(item.Expr)
	} else {
		expr = rebasePlaceholders(expr, placeholderIndex-1)
		args = append(args, item.Args...)
		placeholderIndex += len(item.Args)
	}
	if item.Alias != "" {
		expr += " as " + d.QuoteIdent(item.Alias)
	}
	return expr, args, placeholderIndex, nil
}

func (d dialect) compileAggregateSelect(item oro.SelectExpr) (string, error) {
	if len(item.Args) == 0 {
		return "", &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument}
	}
	expr, ok := item.Args[0].(oro.AggregateExpr)
	if !ok {
		return "", &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument}
	}
	sql, err := queryutil.AggregateSelectSQL(expr.Func, expr.Field, d.QuoteIdent)
	if err != nil {
		return "", &oro.Error{Op: "pgsql.select", Kind: oro.ErrInvalidArgument}
	}
	return sql, nil
}

func (d dialect) compileSelectSource(stmt oro.SelectAST, start int) (string, []any, int, error) {
	if stmt.From.Table != "" || stmt.From.Query != nil || stmt.From.Raw != nil {
		source := stmt.From
		if source.Alias == "" {
			source.Alias = stmt.Alias
		}
		return d.compileSource(source, start)
	}
	return d.compileSource(oro.SourceAST{Table: stmt.Table, Alias: stmt.Alias}, start)
}

func (d dialect) compileSource(source oro.SourceAST, start int) (string, []any, int, error) {
	switch {
	case source.Table != "":
		sql := d.QuoteIdent(source.Table)
		if source.Alias != "" {
			sql += " as " + d.QuoteIdent(source.Alias)
		}
		return sql, nil, start, nil
	case source.Query != nil:
		if source.Alias == "" {
			return "", nil, start, &oro.Error{Op: "pgsql.source", Kind: oro.ErrInvalidArgument}
		}
		compiled, err := d.compileSelect(*source.Query)
		if err != nil {
			return "", nil, start, err
		}
		sql := rebasePlaceholders(compiled.SQL, start-1)
		return "(" + sql + ") as " + d.QuoteIdent(source.Alias), compiled.Args, start + len(compiled.Args), nil
	case source.Raw != nil:
		if source.Alias == "" {
			return "", nil, start, &oro.Error{Op: "pgsql.source", Kind: oro.ErrInvalidArgument}
		}
		sql := rebasePlaceholders(source.Raw.SQL, start-1)
		return "(" + sql + ") as " + d.QuoteIdent(source.Alias), source.Raw.Args, start + len(source.Raw.Args), nil
	default:
		return "", nil, start, &oro.Error{Op: "pgsql.source", Kind: oro.ErrInvalidArgument}
	}
}

func rebasePlaceholders(sql string, offset int) string {
	if strings.Contains(sql, "?") {
		next := offset + 1
		var builder strings.Builder
		builder.Grow(len(sql) + 8)
		inSingleQuote := false
		for index := 0; index < len(sql); index++ {
			char := sql[index]
			if char == '\'' {
				inSingleQuote = !inSingleQuote
				builder.WriteByte(char)
				continue
			}
			if char == '?' && !inSingleQuote {
				builder.WriteString(fmt.Sprintf("$%d", next))
				next++
				continue
			}
			builder.WriteByte(char)
		}
		return builder.String()
	}
	return placeholderPattern.ReplaceAllStringFunc(sql, func(match string) string {
		value, err := strconv.Atoi(match[1:])
		if err != nil {
			return match
		}
		return fmt.Sprintf("$%d", value+offset)
	})
}

func (d dialect) compileJoins(joins []oro.JoinAST, start int) (string, []any, int, error) {
	if len(joins) == 0 {
		return "", nil, start, nil
	}
	parts := make([]string, 0, len(joins))
	args := []any{}
	placeholderIndex := start
	for _, join := range joins {
		joinSQL, conditionArgs, nextIndex, err := d.compileJoin(join, placeholderIndex)
		if err != nil {
			return "", nil, start, err
		}
		parts = append(parts, joinSQL)
		args = append(args, conditionArgs...)
		placeholderIndex = nextIndex
	}
	return " " + strings.Join(parts, " "), args, placeholderIndex, nil
}

func (d dialect) compileJoin(join oro.JoinAST, start int) (string, []any, int, error) {
	if join.Raw != nil {
		return rebasePlaceholders(join.Raw.SQL, start-1), join.Raw.Args, start + len(join.Raw.Args), nil
	}
	joinType := ""
	switch join.Type {
	case oro.JoinInner:
		joinType = "join"
	case oro.JoinLeft:
		joinType = "left join"
	case oro.JoinRight:
		joinType = "right join"
	case oro.JoinFull:
		joinType = "full join"
	case oro.JoinCross:
		source, sourceArgs, nextIndex, err := d.compileJoinSource(join, start)
		if err != nil {
			return "", nil, start, err
		}
		return "cross join " + source, sourceArgs, nextIndex, nil
	default:
		return "", nil, start, &oro.Error{Op: "pgsql.join", Kind: oro.ErrInvalidArgument}
	}
	if len(join.Conditions) == 0 {
		return "", nil, start, &oro.Error{Op: "pgsql.join", Kind: oro.ErrInvalidArgument}
	}
	source, sourceArgs, nextIndex, err := d.compileJoinSource(join, start)
	if err != nil {
		return "", nil, start, err
	}
	conditions, args, nextIndex := d.compileJoinConditions(join.Conditions, nextIndex)
	args = append(sourceArgs, args...)
	return joinType + " " + source + " on " + conditions, args, nextIndex, nil
}

func (d dialect) compileJoinSource(join oro.JoinAST, start int) (string, []any, int, error) {
	source := join.Source
	if source.Table == "" && source.Query == nil && source.Raw == nil {
		source.Table = join.Table
	}
	if join.Alias != "" {
		source.Alias = join.Alias
	}
	return d.compileSource(source, start)
}

func (d dialect) compileJoinConditions(conditions []oro.JoinCondition, start int) (string, []any, int) {
	parts := make([]string, 0, len(conditions))
	args := []any{}
	placeholderIndex := start
	for index, condition := range conditions {
		prefix := ""
		if index > 0 {
			prefix = strings.ToLower(condition.Bool) + " "
		}
		if len(condition.Group) > 0 {
			groupSQL, groupArgs, nextIndex := d.compileJoinConditions(condition.Group, placeholderIndex)
			parts = append(parts, prefix+"("+groupSQL+")")
			args = append(args, groupArgs...)
			placeholderIndex = nextIndex
			continue
		}
		if condition.Column {
			parts = append(parts, prefix+d.QuoteIdent(condition.Left)+" "+condition.Op+" "+d.QuoteIdent(condition.Right))
			continue
		}
		parts = append(parts, prefix+d.QuoteIdent(condition.Left)+" "+condition.Op+" "+d.Placeholder(placeholderIndex))
		args = append(args, condition.Value)
		placeholderIndex++
	}
	return strings.Join(parts, " "), args, placeholderIndex
}

func (d dialect) compileLock(lock oro.LockSpec) (string, error) {
	switch lock.Mode {
	case oro.LockNone:
		return "", nil
	case oro.LockUpdate:
		return d.compileLockOptions(" for update", lock), nil
	case oro.LockShare:
		return d.compileLockOptions(" for share", lock), nil
	default:
		return "", &oro.Error{Op: "pgsql.lock", Kind: oro.ErrInvalidArgument}
	}
}

func (d dialect) compileLockOptions(sql string, lock oro.LockSpec) string {
	if lock.NoWait {
		sql += " nowait"
	}
	if lock.SkipLocked {
		sql += " skip locked"
	}
	return sql
}

func (d dialect) compileInsert(stmt oro.InsertAST) (oro.CompiledSQL, error) {
	if len(stmt.Values) == 0 {
		return oro.CompiledSQL{}, &oro.Error{Op: "pgsql.insert", Kind: oro.ErrInvalidArgument}
	}
	if len(stmt.Conflict.Columns) > 0 {
		return d.compileUpsert(stmt)
	}
	row := stmt.Values[0]
	columnNames := sortedKeys(row)
	columns := make([]string, 0, len(row))
	for _, column := range columnNames {
		columns = append(columns, d.QuoteIdent(column))
	}
	args := make([]any, 0, len(columnNames)*len(stmt.Values))
	rows := make([]string, 0, len(stmt.Values))
	placeholderIndex := 1
	for _, value := range stmt.Values {
		if len(value) != len(columnNames) {
			return oro.CompiledSQL{}, &oro.Error{Op: "pgsql.insert", Kind: oro.ErrInvalidArgument}
		}
		placeholders := make([]string, 0, len(columnNames))
		for _, column := range columnNames {
			item, ok := value[column]
			if !ok {
				return oro.CompiledSQL{}, &oro.Error{Op: "pgsql.insert", Kind: oro.ErrInvalidArgument}
			}
			args = append(args, item)
			placeholders = append(placeholders, d.Placeholder(placeholderIndex))
			placeholderIndex++
		}
		rows = append(rows, "("+strings.Join(placeholders, ", ")+")")
	}
	sql := fmt.Sprintf("insert into %s (%s) values %s", d.QuoteIdent(stmt.Table), strings.Join(columns, ", "), strings.Join(rows, ", "))
	if stmt.Returning {
		sql += " returning *"
	}
	return oro.CompiledSQL{SQL: sql, Args: args}, nil
}

func (d dialect) compileUpsert(stmt oro.InsertAST) (oro.CompiledSQL, error) {
	compiled, err := d.compileInsert(oro.InsertAST{
		Table:  stmt.Table,
		Values: stmt.Values,
	})
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql := compiled.SQL
	args := compiled.Args

	target := make([]string, 0, len(stmt.Conflict.Columns))
	for _, column := range stmt.Conflict.Columns {
		target = append(target, d.QuoteIdent(column))
	}
	sql += " on conflict (" + strings.Join(target, ", ") + ")"
	updateSQL, updateArgs, err := d.compileConflictUpdate(stmt.Conflict, stmt.Values[0], len(args)+1)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql += updateSQL
	args = append(args, updateArgs...)
	if stmt.Returning {
		sql += " returning *"
	}
	return oro.CompiledSQL{SQL: sql, Args: args}, nil
}

func (d dialect) compileConflictUpdate(conflict oro.ConflictSpec, row oro.Map, start int) (string, []any, error) {
	if conflict.DoNothing {
		return " do nothing", nil, nil
	}

	updateValues := conflict.UpdateMap
	if len(updateValues) == 0 {
		updateValues = oro.Map{}
		fields := conflict.Update
		if conflict.UpdateAll {
			fields = sortedKeys(row)
		}
		for _, column := range fields {
			if value, ok := row[column]; ok {
				updateValues[column] = value
			}
		}
	}
	if len(updateValues) == 0 {
		return "", nil, &oro.Error{Op: "pgsql.upsert", Kind: oro.ErrInvalidArgument}
	}

	columnNames := sortedKeys(updateValues)
	sets := make([]string, 0, len(columnNames))
	args := make([]any, 0, len(columnNames))
	placeholderIndex := start
	for _, column := range columnNames {
		setSQL, setArgs, nextIndex := d.compileSet(column, updateValues[column], placeholderIndex)
		sets = append(sets, setSQL)
		args = append(args, setArgs...)
		placeholderIndex = nextIndex
	}
	return " do update set " + strings.Join(sets, ", "), args, nil
}

func (d dialect) compileUpdate(stmt oro.UpdateAST) (oro.CompiledSQL, error) {
	if len(stmt.Values) == 0 {
		return oro.CompiledSQL{}, &oro.Error{Op: "pgsql.update", Kind: oro.ErrInvalidArgument}
	}
	columnNames := sortedKeys(stmt.Values)
	sets := make([]string, 0, len(stmt.Values))
	args := make([]any, 0, len(stmt.Values))
	placeholderIndex := 1
	for _, column := range columnNames {
		setSQL, setArgs, nextIndex := d.compileSet(column, stmt.Values[column], placeholderIndex)
		sets = append(sets, setSQL)
		args = append(args, setArgs...)
		placeholderIndex = nextIndex
	}
	where, whereArgs, err := d.compileWhere(stmt.Where, placeholderIndex)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	args = append(args, whereArgs...)
	sql := "update " + d.QuoteIdent(stmt.Table) + " set " + strings.Join(sets, ", ")
	if where != "" {
		sql += " where " + where
	}
	return oro.CompiledSQL{SQL: sql, Args: args}, nil
}

func (d dialect) compileSet(column string, value any, placeholderIndex int) (string, []any, int) {
	switch expr := value.(type) {
	case oro.IncrementExpr:
		return d.QuoteIdent(column) + " = " + d.QuoteIdent(column) + " + " + d.Placeholder(placeholderIndex), []any{expr.Value}, placeholderIndex + 1
	case oro.DecrementExpr:
		return d.QuoteIdent(column) + " = " + d.QuoteIdent(column) + " - " + d.Placeholder(placeholderIndex), []any{expr.Value}, placeholderIndex + 1
	case oro.RawExpr:
		return d.QuoteIdent(column) + " = " + expr.SQL, expr.Args, placeholderIndex + len(expr.Args)
	default:
		return d.QuoteIdent(column) + " = " + d.Placeholder(placeholderIndex), []any{value}, placeholderIndex + 1
	}
}

func (d dialect) compileDelete(stmt oro.DeleteAST) (oro.CompiledSQL, error) {
	sql := "delete from " + d.QuoteIdent(stmt.Table)
	where, args, err := d.compileWhere(stmt.Where, 1)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	if where != "" {
		sql += " where " + where
	}
	return oro.CompiledSQL{SQL: sql, Args: args}, nil
}

func (d dialect) compileWhere(conditions []oro.Condition, start int) (string, []any, error) {
	parts := make([]string, 0, len(conditions))
	args := make([]any, 0, len(conditions))
	placeholderIndex := start
	for index, condition := range conditions {
		prefix := ""
		if index > 0 {
			prefix = " and "
			if strings.ToLower(strings.TrimSpace(condition.Bool)) == "or" {
				prefix = " or "
			}
		}
		op := strings.ToLower(strings.TrimSpace(condition.Op))
		switch op {
		case "invalid":
			if err, ok := condition.Value.(error); ok {
				return "", nil, err
			}
			return "", nil, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
		case "group":
			sql, groupArgs, err := d.compileWhere(condition.Conditions, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			if sql == "" {
				continue
			}
			parts = append(parts, prefix+"("+sql+")")
			args = append(args, groupArgs...)
			placeholderIndex += len(groupArgs)
		case "not":
			sql, groupArgs, err := d.compileWhere(condition.Conditions, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			if sql == "" {
				continue
			}
			parts = append(parts, prefix+"not ("+sql+")")
			args = append(args, groupArgs...)
			placeholderIndex += len(groupArgs)
		case "raw":
			parts = append(parts, prefix+rebasePlaceholders(condition.Field, placeholderIndex-1))
			if values, ok := condition.Value.([]any); ok {
				args = append(args, values...)
				placeholderIndex += len(values)
			}
		case "json":
			sql, jsonArgs, nextIndex, err := d.compileJSONCondition(condition, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, jsonArgs...)
			placeholderIndex = nextIndex
		case "fulltext":
			sql, fullTextArgs, nextIndex, err := d.compileFullTextCondition(condition, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, fullTextArgs...)
			placeholderIndex = nextIndex
		case "column":
			sql, err := d.compileColumnCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
		case "in":
			sql, inArgs, nextIndex, err := d.compileInCondition(condition, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, inArgs...)
			placeholderIndex = nextIndex
		case "in_values", "not_in_values":
			sql, inArgs, nextIndex, err := d.compileInValuesCondition(condition, placeholderIndex, op == "not_in_values")
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, inArgs...)
			placeholderIndex = nextIndex
		case "between":
			sql, inArgs, nextIndex, err := d.compileBetweenCondition(condition, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, inArgs...)
			placeholderIndex = nextIndex
		case "exists":
			sql, existsArgs, nextIndex, err := d.compileExistsCondition(condition, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, existsArgs...)
			placeholderIndex = nextIndex
		case "count":
			sql, countArgs, nextIndex, err := d.compileCountCondition(condition, placeholderIndex)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, countArgs...)
			placeholderIndex = nextIndex
		case "is null", "is not null":
			parts = append(parts, prefix+d.QuoteIdent(condition.Field)+" "+op)
		default:
			if !oro.IsSafeConditionOperator(condition.Op) {
				return "", nil, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
			}
			if source, ok := condition.Value.(*oro.SourceAST); ok {
				sql, sourceArgs, nextIndex, err := d.compileScalarSource(*source, placeholderIndex, "pgsql.where")
				if err != nil {
					return "", nil, err
				}
				parts = append(parts, prefix+d.QuoteIdent(condition.Field)+" "+oro.NormalizeConditionOperator(condition.Op)+" "+sql)
				args = append(args, sourceArgs...)
				placeholderIndex = nextIndex
			} else {
				sql := prefix + d.QuoteIdent(condition.Field) + " " + oro.NormalizeConditionOperator(condition.Op) + " " + d.Placeholder(placeholderIndex)
				if condition.Escape != "" {
					if condition.Escape != `\` {
						return "", nil, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
					}
					sql += " escape '\\'"
				}
				parts = append(parts, sql)
				args = append(args, condition.Value)
				placeholderIndex++
			}
		}
	}
	return strings.Join(parts, ""), args, nil
}

func (d dialect) compileColumnCondition(condition oro.Condition) (string, error) {
	columnCondition, ok := condition.Value.(oro.ColumnCondition)
	if !ok || columnCondition.Right == "" {
		return "", &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
	}
	if !oro.IsSafeColumnOperator(columnCondition.Op) {
		return "", &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
	}
	return d.QuoteIdent(condition.Field) + " " + oro.NormalizeConditionOperator(columnCondition.Op) + " " + d.QuoteIdent(columnCondition.Right), nil
}

func (d dialect) compileInCondition(condition oro.Condition, start int) (string, []any, int, error) {
	if source, ok := condition.Value.(*oro.SourceAST); ok {
		sql, args, nextIndex, err := d.compileScalarSource(*source, start, "pgsql.where")
		if err != nil {
			return "", nil, start, err
		}
		return d.QuoteIdent(condition.Field) + " in " + sql, args, nextIndex, nil
	}
	return "", nil, start, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
}

func (d dialect) compileInValuesCondition(condition oro.Condition, start int, not bool) (string, []any, int, error) {
	values, ok := condition.Value.([]any)
	if !ok {
		return "", nil, start, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
	}
	if len(values) == 0 {
		if not {
			return "1 = 1", nil, start, nil
		}
		return "1 = 0", nil, start, nil
	}
	placeholders := make([]string, len(values))
	for index := range placeholders {
		placeholders[index] = d.Placeholder(start + index)
	}
	op := " in "
	if not {
		op = " not in "
	}
	return d.QuoteIdent(condition.Field) + op + "(" + strings.Join(placeholders, ", ") + ")", values, start + len(values), nil
}

func (d dialect) compileBetweenCondition(condition oro.Condition, start int) (string, []any, int, error) {
	values, ok := condition.Value.([]any)
	if !ok || len(values) != 2 {
		return "", nil, start, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
	}
	return d.QuoteIdent(condition.Field) + " between " + d.Placeholder(start) + " and " + d.Placeholder(start+1), values, start + 2, nil
}

func (d dialect) compileExistsCondition(condition oro.Condition, start int) (string, []any, int, error) {
	source, ok := condition.Value.(*oro.SourceAST)
	if !ok {
		return "", nil, start, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument}
	}
	sql, args, nextIndex, err := d.compileScalarSource(*source, start, "pgsql.where")
	if err != nil {
		return "", nil, start, err
	}
	return "exists " + sql, args, nextIndex, nil
}

func (d dialect) compileCountCondition(condition oro.Condition, start int) (string, []any, int, error) {
	countCondition, ok := condition.Value.(oro.CountCondition)
	if !ok || countCondition.Source == nil {
		return "", nil, start, &oro.Error{Op: "pgsql.where", Kind: oro.ErrInvalidArgument}
	}
	sql, args, nextIndex, err := d.compileScalarSource(*countCondition.Source, start, "pgsql.where")
	if err != nil {
		return "", nil, start, err
	}
	return sql + " " + countCondition.Op + " " + d.Placeholder(nextIndex), append(args, countCondition.Value), nextIndex + 1, nil
}

func (d dialect) compileScalarSource(source oro.SourceAST, start int, op string) (string, []any, int, error) {
	switch {
	case source.Query != nil:
		compiled, err := d.compileSelect(*source.Query)
		if err != nil {
			return "", nil, start, err
		}
		sql := rebasePlaceholders(compiled.SQL, start-1)
		return "(" + sql + ")", compiled.Args, start + len(compiled.Args), nil
	case source.Raw != nil:
		sql := rebasePlaceholders(source.Raw.SQL, start-1)
		return "(" + sql + ")", source.Raw.Args, start + len(source.Raw.Args), nil
	default:
		return "", nil, start, &oro.Error{Op: op, Kind: oro.ErrInvalidArgument}
	}
}

func (d dialect) compileFullTextCondition(condition oro.Condition, start int) (string, []any, int, error) {
	expr, ok := condition.Value.(oro.FullTextExpr)
	if !ok || len(expr.Fields) == 0 {
		return "", nil, start, &oro.Error{Op: "pgsql.fulltext", Kind: oro.ErrInvalidArgument}
	}
	vector := d.fullTextVector(expr.Fields)
	return vector + " @@ plainto_tsquery(" + d.Placeholder(start) + ")", []any{expr.Query}, start + 1, nil
}

func (d dialect) compileFullTextScore(item oro.SelectExpr, start int) (string, []any, int, error) {
	expr, ok := item.Args[0].(oro.FullTextExpr)
	if !ok || len(expr.Fields) == 0 {
		return "", nil, start, &oro.Error{Op: "pgsql.fulltext", Kind: oro.ErrInvalidArgument}
	}
	vector := d.fullTextVector(expr.Fields)
	return "ts_rank(" + vector + ", plainto_tsquery(" + d.Placeholder(start) + "))", []any{expr.Query}, start + 1, nil
}

func (d dialect) fullTextVector(fields []string) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, "coalesce("+d.QuoteIdent(field)+"::text, '')")
	}
	return "to_tsvector(" + strings.Join(parts, " || ' ' || ") + ")"
}

func (d dialect) compileJSONCondition(condition oro.Condition, start int) (string, []any, int, error) {
	jsonCondition, ok := condition.Value.(oro.JSONCondition)
	if !ok {
		return "", nil, start, &oro.Error{Op: "pgsql.json", Kind: oro.ErrInvalidArgument}
	}
	expr := d.QuoteIdent(jsonCondition.Field) + " #> " + d.Placeholder(start)
	path := pgsqlJSONPath(jsonCondition.Parts)
	switch strings.ToLower(jsonCondition.Op) {
	case "=":
		value, err := jsonConditionArgument(jsonCondition.Value)
		if err != nil {
			return "", nil, start, err
		}
		return expr + " = " + d.Placeholder(start+1) + "::jsonb", []any{path, value}, start + 2, nil
	case "!=":
		value, err := jsonConditionArgument(jsonCondition.Value)
		if err != nil {
			return "", nil, start, err
		}
		return expr + " != " + d.Placeholder(start+1) + "::jsonb", []any{path, value}, start + 2, nil
	case "is null":
		return expr + " is null", []any{path}, start + 1, nil
	case "is not null":
		return expr + " is not null", []any{path}, start + 1, nil
	case "exists":
		return expr + " is not null", []any{path}, start + 1, nil
	case "contains":
		value, err := jsonConditionArgument(jsonCondition.Value)
		if err != nil {
			return "", nil, start, err
		}
		return expr + " @> " + d.Placeholder(start+1) + "::jsonb", []any{path, value}, start + 2, nil
	case "like":
		return d.QuoteIdent(jsonCondition.Field) + " #>> " + d.Placeholder(start) + " like " + d.Placeholder(start+1), []any{path, jsonCondition.Value}, start + 2, nil
	default:
		return "", nil, start, &oro.Error{Op: "pgsql.json", Kind: oro.ErrInvalidArgument}
	}
}

func jsonConditionArgument(value any) (string, error) {
	bytesValue, err := json.Marshal(value)
	if err != nil {
		return "", &oro.Error{Op: "pgsql.json", Kind: oro.ErrInvalidArgument, Cause: err}
	}
	return string(bytesValue), nil
}

func pgsqlJSONPath(parts []string) string {
	if len(parts) == 0 {
		return "{}"
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func (d dialect) compileGroup(fields []string) string {
	parts := make([]string, 0, len(fields))
	for _, field := range fields {
		parts = append(parts, d.QuoteIdent(field))
	}
	return strings.Join(parts, ", ")
}

func (d dialect) compileOrder(items []oro.OrderExpr) (string, []any) {
	parts := make([]string, 0, len(items))
	args := []any{}
	for _, item := range items {
		expr := item.Expr
		if !item.Raw {
			expr = d.QuoteIdent(item.Expr)
		}
		if item.Desc {
			expr += " desc"
		} else if !item.Raw {
			expr += " asc"
		}
		parts = append(parts, expr)
		args = append(args, item.Args...)
	}
	return strings.Join(parts, ", "), args
}

func (d dialect) compileCreateTable(table oro.TableSpec) ([]oro.CompiledSQL, error) {
	if table.Name == "" || len(table.Columns) == 0 {
		return nil, &oro.Error{Op: "pgsql.schema", Kind: oro.ErrInvalidArgument}
	}
	columns := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		sql, err := d.compileColumn(column, true)
		if err != nil {
			return nil, err
		}
		columns = append(columns, sql)
	}
	statements := []oro.CompiledSQL{{SQL: "create table if not exists " + d.QuoteIdent(table.Name) + " (" + strings.Join(columns, ", ") + ")"}}
	for _, column := range table.Columns {
		statements = append(statements, d.compileColumnComment(table.Name, column)...)
	}
	return statements, nil
}

func (d dialect) compileColumn(column oro.ColumnSpec, allowPrimary bool) (string, error) {
	dataType, err := d.DataType(column)
	if err != nil {
		return "", err
	}
	parts := []string{d.QuoteIdent(column.ColumnName), dataType}
	if allowPrimary && column.Primary {
		if isIntegerType(column.Type) {
			parts = append(parts, "primary key generated by default as identity")
		} else {
			parts = append(parts, "primary key")
		}
	}
	if !column.Nullable && !column.Primary {
		parts = append(parts, "not null")
	}
	if column.Default != nil {
		parts = append(parts, "default "+compileDefault(column.Default))
	}
	return strings.Join(parts, " "), nil
}

func isIntegerType(typ string) bool {
	return strings.Contains(typ, "int")
}

func (d dialect) compileColumnComment(table string, column oro.ColumnSpec) []oro.CompiledSQL {
	if column.Comment == "" {
		return nil
	}
	return []oro.CompiledSQL{{
		SQL: "comment on column " + d.QuoteIdent(table+"."+column.ColumnName) + " is " + schemaString(column.Comment),
	}}
}

func compileDefault(defaultValue *oro.DefaultSpec) string {
	if defaultValue.Expr != "" {
		return defaultValue.Expr
	}
	return oro.FormatDefaultValue(defaultValue.Value)
}

func schemaString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func (d dialect) compileCreateIndex(table string, index oro.IndexSpec) ([]oro.CompiledSQL, error) {
	columns := make([]string, 0, len(index.Fields))
	for _, field := range index.Fields {
		columns = append(columns, d.QuoteIdent(field))
	}
	if index.FullText {
		parts := make([]string, 0, len(index.Fields))
		for _, field := range index.Fields {
			parts = append(parts, "coalesce("+d.QuoteIdent(field)+", '')")
		}
		expr := "to_tsvector('simple', " + strings.Join(parts, " || ' ' || ") + ")"
		return []oro.CompiledSQL{{SQL: "create index if not exists " + d.QuoteIdent(index.Name) + " on " + d.QuoteIdent(table) + " using gin (" + expr + ")"}}, nil
	}
	unique := ""
	if index.Unique {
		unique = "unique "
	}
	return []oro.CompiledSQL{{SQL: "create " + unique + "index if not exists " + d.QuoteIdent(index.Name) + " on " + d.QuoteIdent(table) + " (" + strings.Join(columns, ", ") + ")"}}, nil
}

func sortedKeys(row oro.Map) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
