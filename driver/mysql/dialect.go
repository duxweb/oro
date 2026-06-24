package mysql

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/internal/queryutil"
)

type dialect struct{}

func (dialect) Name() string {
	return "mysql"
}

func (dialect) Capabilities() oro.Capabilities {
	return oro.Capabilities{
		Upsert:         true,
		Savepoint:      true,
		LockForUpdate:  true,
		LockForShare:   true,
		LockNoWait:     true,
		LockSkipLocked: true,
		JSON:           true,
		FullText:       true,
	}
}

func (dialect) QuoteIdent(name string) string {
	parts := strings.Split(name, ".")
	for index, part := range parts {
		parts[index] = "`" + strings.ReplaceAll(part, "`", "``") + "`"
	}
	return strings.Join(parts, ".")
}

func (dialect) Placeholder(index int) string {
	return "?"
}

func (dialect) DataType(column oro.ColumnSpec) (string, error) {
	switch column.Type {
	case "string":
		if column.Size > 0 {
			return fmt.Sprintf("varchar(%d)", column.Size), nil
		}
		return "varchar(255)", nil
	case "text":
		return "text", nil
	case "bool":
		return "boolean", nil
	case "int", "int32":
		return "int", nil
	case "uint", "uint32":
		return "int unsigned", nil
	case "uint64":
		return "bigint unsigned", nil
	case "int64":
		return "bigint", nil
	case "decimal":
		if column.Precision <= 0 {
			return "decimal", nil
		}
		return fmt.Sprintf("decimal(%d,%d)", column.Precision, column.Scale), nil
	case "float":
		return "float", nil
	case "double":
		return "double", nil
	case "binary":
		return "blob", nil
	case "json", "string_array", "int_array":
		return "json", nil
	case "uuid":
		return "char(36)", nil
	case "time.Time":
		return "datetime", nil
	case "date":
		return "date", nil
	case "time":
		return "time", nil
	case "enum", "email", "url", "ip", "mac", "phone", "slug", "color":
		if column.Size > 0 {
			return fmt.Sprintf("varchar(%d)", column.Size), nil
		}
		return "varchar(255)", nil
	case "point":
		return "point", nil
	default:
		if strings.Contains(column.Type, "time.Time") {
			return "datetime", nil
		}
		if strings.Contains(column.Type, "Decimal") {
			if column.Precision > 0 {
				return fmt.Sprintf("decimal(%d,%d)", column.Precision, column.Scale), nil
			}
			return "decimal", nil
		}
		if strings.Contains(column.Type, "JSON") {
			return "json", nil
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
		return oro.CompiledSQL{}, &oro.Error{Op: "mysql.compile", Kind: oro.ErrUnsupported}
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
		return []oro.CompiledSQL{{SQL: "alter table " + d.QuoteIdent(change.Table.Name) + " add column " + sql}}, nil
	case oro.SchemaCreateIndex:
		return d.compileCreateIndex(change.Table.Name, change.Index)
	case oro.SchemaRenameColumn:
		if change.Current.ColumnName == "" || change.Column.ColumnName == "" {
			return nil, &oro.Error{Op: "mysql.schema", Kind: oro.ErrInvalidArgument}
		}
		return []oro.CompiledSQL{{SQL: "alter table " + d.QuoteIdent(change.Table.Name) + " rename column " + d.QuoteIdent(change.Current.ColumnName) + " to " + d.QuoteIdent(change.Column.ColumnName)}}, nil
	case oro.SchemaUnsafeChange:
		return nil, &oro.Error{Op: "mysql.schema", Kind: oro.ErrUnsafeSchemaChange}
	default:
		return nil, &oro.Error{Op: "mysql.schema", Kind: oro.ErrUnsupported}
	}
}

func (d dialect) compileSelect(stmt oro.SelectAST) (oro.CompiledSQL, error) {
	selectSQL := "*"
	selectArgs := []any{}
	if len(stmt.Select) > 0 {
		parts := make([]string, 0, len(stmt.Select))
		for _, item := range stmt.Select {
			expr, itemArgs, err := d.compileSelectExpr(item)
			if err != nil {
				return oro.CompiledSQL{}, err
			}
			selectArgs = append(selectArgs, itemArgs...)
			parts = append(parts, expr)
		}
		selectSQL = strings.Join(parts, ", ")
	}

	sourceSQL, sourceArgs, err := d.compileSelectSource(stmt)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql := "select " + selectSQL + " from " + sourceSQL
	joins, joinArgs, err := d.compileJoins(stmt.Joins)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql += joins
	args := append(selectArgs, sourceArgs...)
	args = append(args, joinArgs...)
	where, whereArgs, err := d.compileWhere(stmt.Where)
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
	having, havingArgs, err := d.compileWhere(stmt.Having)
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
			return oro.CompiledSQL{}, &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument, Field: "Limit"}
		}
		sql += fmt.Sprintf(" limit %d", *stmt.Limit)
	}
	if stmt.Offset != nil {
		if *stmt.Offset < 0 {
			return oro.CompiledSQL{}, &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument, Field: "Offset"}
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

func (d dialect) compileSelectExpr(item oro.SelectExpr) (string, []any, error) {
	if item.Source != nil {
		expr, args, err := d.compileScalarSource(*item.Source, "mysql.select")
		if err != nil {
			return "", nil, err
		}
		if item.Alias == "" {
			return "", nil, &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument}
		}
		return expr + " as " + d.QuoteIdent(item.Alias), args, nil
	}
	if item.Expr == "__oro_relation_exists__" {
		if len(item.Args) != 1 {
			return "", nil, &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument}
		}
		source, ok := item.Args[0].(oro.SourceAST)
		if !ok {
			return "", nil, &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument}
		}
		expr, args, err := d.compileScalarSource(source, "mysql.select")
		if err != nil {
			return "", nil, err
		}
		if item.Alias == "" {
			return "", nil, &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument}
		}
		return "exists " + expr + " as " + d.QuoteIdent(item.Alias), args, nil
	}
	expr := item.Expr
	args := []any{}
	if item.Expr == "__oro_fulltext_score__" {
		scoreSQL, scoreArgs, err := d.compileFullTextScore(item)
		if err != nil {
			return "", nil, err
		}
		expr = scoreSQL
		args = append(args, scoreArgs...)
	} else if item.Expr == "__oro_aggregate__" {
		aggregateSQL, err := d.compileAggregateSelect(item)
		if err != nil {
			return "", nil, err
		}
		expr = aggregateSQL
	} else if !item.Raw {
		expr = d.QuoteIdent(item.Expr)
	}
	if item.Alias != "" {
		expr += " as " + d.QuoteIdent(item.Alias)
	}
	return expr, args, nil
}

func (d dialect) compileAggregateSelect(item oro.SelectExpr) (string, error) {
	if len(item.Args) == 0 {
		return "", &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument}
	}
	expr, ok := item.Args[0].(oro.AggregateExpr)
	if !ok {
		return "", &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument}
	}
	sql, err := queryutil.AggregateSelectSQL(expr.Func, expr.Field, d.QuoteIdent)
	if err != nil {
		return "", &oro.Error{Op: "mysql.select", Kind: oro.ErrInvalidArgument}
	}
	return sql, nil
}

func (d dialect) compileSelectSource(stmt oro.SelectAST) (string, []any, error) {
	if stmt.From.Table != "" || stmt.From.Query != nil || stmt.From.Raw != nil {
		source := stmt.From
		if source.Alias == "" {
			source.Alias = stmt.Alias
		}
		return d.compileSource(source)
	}
	return d.compileSource(oro.SourceAST{Table: stmt.Table, Alias: stmt.Alias})
}

func (d dialect) compileSource(source oro.SourceAST) (string, []any, error) {
	switch {
	case source.Table != "":
		sql := d.QuoteIdent(source.Table)
		if source.Alias != "" {
			sql += " as " + d.QuoteIdent(source.Alias)
		}
		return sql, nil, nil
	case source.Query != nil:
		if source.Alias == "" {
			return "", nil, &oro.Error{Op: "mysql.source", Kind: oro.ErrInvalidArgument}
		}
		compiled, err := d.compileSelect(*source.Query)
		if err != nil {
			return "", nil, err
		}
		return "(" + compiled.SQL + ") as " + d.QuoteIdent(source.Alias), compiled.Args, nil
	case source.Raw != nil:
		if source.Alias == "" {
			return "", nil, &oro.Error{Op: "mysql.source", Kind: oro.ErrInvalidArgument}
		}
		return "(" + source.Raw.SQL + ") as " + d.QuoteIdent(source.Alias), source.Raw.Args, nil
	default:
		return "", nil, &oro.Error{Op: "mysql.source", Kind: oro.ErrInvalidArgument}
	}
}

func (d dialect) compileJoins(joins []oro.JoinAST) (string, []any, error) {
	if len(joins) == 0 {
		return "", nil, nil
	}
	parts := make([]string, 0, len(joins))
	args := []any{}
	for _, join := range joins {
		joinSQL, conditionArgs, err := d.compileJoin(join)
		if err != nil {
			return "", nil, err
		}
		parts = append(parts, joinSQL)
		args = append(args, conditionArgs...)
	}
	return " " + strings.Join(parts, " "), args, nil
}

func (d dialect) compileJoin(join oro.JoinAST) (string, []any, error) {
	if join.Raw != nil {
		return join.Raw.SQL, join.Raw.Args, nil
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
		source, sourceArgs, err := d.compileJoinSource(join)
		if err != nil {
			return "", nil, err
		}
		return "cross join " + source, sourceArgs, nil
	default:
		return "", nil, &oro.Error{Op: "mysql.join", Kind: oro.ErrInvalidArgument}
	}
	if len(join.Conditions) == 0 {
		return "", nil, &oro.Error{Op: "mysql.join", Kind: oro.ErrInvalidArgument}
	}
	conditions, args := d.compileJoinConditions(join.Conditions)
	source, sourceArgs, err := d.compileJoinSource(join)
	if err != nil {
		return "", nil, err
	}
	args = append(sourceArgs, args...)
	return joinType + " " + source + " on " + conditions, args, nil
}

func (d dialect) compileJoinSource(join oro.JoinAST) (string, []any, error) {
	source := join.Source
	if source.Table == "" && source.Query == nil && source.Raw == nil {
		source.Table = join.Table
	}
	if join.Alias != "" {
		source.Alias = join.Alias
	}
	return d.compileSource(source)
}

func (d dialect) compileJoinConditions(conditions []oro.JoinCondition) (string, []any) {
	parts := make([]string, 0, len(conditions))
	args := []any{}
	for index, condition := range conditions {
		prefix := ""
		if index > 0 {
			prefix = strings.ToLower(condition.Bool) + " "
		}
		if len(condition.Group) > 0 {
			groupSQL, groupArgs := d.compileJoinConditions(condition.Group)
			parts = append(parts, prefix+"("+groupSQL+")")
			args = append(args, groupArgs...)
			continue
		}
		if condition.Column {
			parts = append(parts, prefix+d.QuoteIdent(condition.Left)+" "+condition.Op+" "+d.QuoteIdent(condition.Right))
			continue
		}
		parts = append(parts, prefix+d.QuoteIdent(condition.Left)+" "+condition.Op+" ?")
		args = append(args, condition.Value)
	}
	return strings.Join(parts, " "), args
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
		return "", &oro.Error{Op: "mysql.lock", Kind: oro.ErrInvalidArgument}
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
		return oro.CompiledSQL{}, &oro.Error{Op: "mysql.insert", Kind: oro.ErrInvalidArgument}
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
	rowPlaceholder := "(" + strings.TrimRight(strings.Repeat("?, ", len(columnNames)), ", ") + ")"
	args := make([]any, 0, len(columnNames)*len(stmt.Values))
	rows := make([]string, 0, len(stmt.Values))
	for _, value := range stmt.Values {
		if len(value) != len(columnNames) {
			return oro.CompiledSQL{}, &oro.Error{Op: "mysql.insert", Kind: oro.ErrInvalidArgument}
		}
		rows = append(rows, rowPlaceholder)
		for _, column := range columnNames {
			item, ok := value[column]
			if !ok {
				return oro.CompiledSQL{}, &oro.Error{Op: "mysql.insert", Kind: oro.ErrInvalidArgument}
			}
			args = append(args, item)
		}
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

	updateSQL, updateArgs, err := d.compileConflictUpdate(stmt.Conflict, stmt.Values[0])
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	sql += updateSQL
	args = append(args, updateArgs...)
	return oro.CompiledSQL{SQL: sql, Args: args}, nil
}

func (d dialect) compileConflictUpdate(conflict oro.ConflictSpec, row oro.Map) (string, []any, error) {
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
	if conflict.DoNothing || len(updateValues) == 0 {
		if len(conflict.Columns) == 0 {
			return "", nil, &oro.Error{Op: "mysql.upsert", Kind: oro.ErrInvalidArgument}
		}
		column := conflict.Columns[0]
		return " on duplicate key update " + d.QuoteIdent(column) + " = " + d.QuoteIdent(column), nil, nil
	}

	columnNames := sortedKeys(updateValues)
	sets := make([]string, 0, len(columnNames))
	args := make([]any, 0, len(columnNames))
	for _, column := range columnNames {
		setSQL, setArgs := d.compileSet(column, updateValues[column])
		sets = append(sets, setSQL)
		args = append(args, setArgs...)
	}
	return " on duplicate key update " + strings.Join(sets, ", "), args, nil
}

func (d dialect) compileUpdate(stmt oro.UpdateAST) (oro.CompiledSQL, error) {
	if len(stmt.Values) == 0 {
		return oro.CompiledSQL{}, &oro.Error{Op: "mysql.update", Kind: oro.ErrInvalidArgument}
	}
	columnNames := sortedKeys(stmt.Values)
	sets := make([]string, 0, len(stmt.Values))
	args := make([]any, 0, len(stmt.Values))
	for _, column := range columnNames {
		setSQL, setArgs := d.compileSet(column, stmt.Values[column])
		sets = append(sets, setSQL)
		args = append(args, setArgs...)
	}
	where, whereArgs, err := d.compileWhere(stmt.Where)
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

func (d dialect) compileSet(column string, value any) (string, []any) {
	switch expr := value.(type) {
	case oro.IncrementExpr:
		return d.QuoteIdent(column) + " = " + d.QuoteIdent(column) + " + ?", []any{expr.Value}
	case oro.DecrementExpr:
		return d.QuoteIdent(column) + " = " + d.QuoteIdent(column) + " - ?", []any{expr.Value}
	case oro.RawExpr:
		return d.QuoteIdent(column) + " = " + expr.SQL, expr.Args
	default:
		return d.QuoteIdent(column) + " = ?", []any{value}
	}
}

func (d dialect) compileDelete(stmt oro.DeleteAST) (oro.CompiledSQL, error) {
	sql := "delete from " + d.QuoteIdent(stmt.Table)
	where, args, err := d.compileWhere(stmt.Where)
	if err != nil {
		return oro.CompiledSQL{}, err
	}
	if where != "" {
		sql += " where " + where
	}
	return oro.CompiledSQL{SQL: sql, Args: args}, nil
}

func (d dialect) compileWhere(conditions []oro.Condition) (string, []any, error) {
	parts := make([]string, 0, len(conditions))
	args := make([]any, 0, len(conditions))
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
			return "", nil, &oro.Error{Op: "mysql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
		case "group":
			sql, groupArgs, err := d.compileWhere(condition.Conditions)
			if err != nil {
				return "", nil, err
			}
			if sql == "" {
				continue
			}
			parts = append(parts, prefix+"("+sql+")")
			args = append(args, groupArgs...)
		case "not":
			sql, groupArgs, err := d.compileWhere(condition.Conditions)
			if err != nil {
				return "", nil, err
			}
			if sql == "" {
				continue
			}
			parts = append(parts, prefix+"not ("+sql+")")
			args = append(args, groupArgs...)
		case "raw":
			parts = append(parts, prefix+condition.Field)
			if values, ok := condition.Value.([]any); ok {
				args = append(args, values...)
			}
		case "json":
			sql, jsonArgs, err := d.compileJSONCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, jsonArgs...)
		case "fulltext":
			sql, fullTextArgs, err := d.compileFullTextCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, fullTextArgs...)
		case "column":
			sql, err := d.compileColumnCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
		case "in":
			sql, inArgs, err := d.compileInCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, inArgs...)
		case "in_values", "not_in_values":
			sql, inArgs, err := d.compileInValuesCondition(condition, op == "not_in_values")
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, inArgs...)
		case "between":
			sql, betweenArgs, err := d.compileBetweenCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, betweenArgs...)
		case "exists":
			sql, existsArgs, err := d.compileExistsCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, existsArgs...)
		case "count":
			sql, countArgs, err := d.compileCountCondition(condition)
			if err != nil {
				return "", nil, err
			}
			parts = append(parts, prefix+sql)
			args = append(args, countArgs...)
		case "is null", "is not null":
			parts = append(parts, prefix+d.QuoteIdent(condition.Field)+" "+op)
		default:
			if source, ok := condition.Value.(*oro.SourceAST); ok {
				sql, sourceArgs, err := d.compileScalarSource(*source, "mysql.where")
				if err != nil {
					return "", nil, err
				}
				parts = append(parts, prefix+d.QuoteIdent(condition.Field)+" "+condition.Op+" "+sql)
				args = append(args, sourceArgs...)
			} else {
				parts = append(parts, prefix+d.QuoteIdent(condition.Field)+" "+condition.Op+" ?")
				args = append(args, condition.Value)
			}
		}
	}
	return strings.Join(parts, ""), args, nil
}

func (d dialect) compileColumnCondition(condition oro.Condition) (string, error) {
	columnCondition, ok := condition.Value.(oro.ColumnCondition)
	if !ok || columnCondition.Right == "" {
		return "", &oro.Error{Op: "mysql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
	}
	return d.QuoteIdent(condition.Field) + " " + columnCondition.Op + " " + d.QuoteIdent(columnCondition.Right), nil
}

func (d dialect) compileInCondition(condition oro.Condition) (string, []any, error) {
	if source, ok := condition.Value.(*oro.SourceAST); ok {
		sql, args, err := d.compileScalarSource(*source, "mysql.where")
		if err != nil {
			return "", nil, err
		}
		return d.QuoteIdent(condition.Field) + " in " + sql, args, nil
	}
	return "", nil, &oro.Error{Op: "mysql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
}

func (d dialect) compileInValuesCondition(condition oro.Condition, not bool) (string, []any, error) {
	values, ok := condition.Value.([]any)
	if !ok || len(values) == 0 {
		return "", nil, &oro.Error{Op: "mysql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
	}
	placeholders := make([]string, len(values))
	for index := range placeholders {
		placeholders[index] = "?"
	}
	op := " in "
	if not {
		op = " not in "
	}
	return d.QuoteIdent(condition.Field) + op + "(" + strings.Join(placeholders, ", ") + ")", values, nil
}

func (d dialect) compileBetweenCondition(condition oro.Condition) (string, []any, error) {
	values, ok := condition.Value.([]any)
	if !ok || len(values) != 2 {
		return "", nil, &oro.Error{Op: "mysql.where", Kind: oro.ErrInvalidArgument, Field: condition.Field}
	}
	return d.QuoteIdent(condition.Field) + " between ? and ?", values, nil
}

func (d dialect) compileExistsCondition(condition oro.Condition) (string, []any, error) {
	source, ok := condition.Value.(*oro.SourceAST)
	if !ok {
		return "", nil, &oro.Error{Op: "mysql.where", Kind: oro.ErrInvalidArgument}
	}
	sql, args, err := d.compileScalarSource(*source, "mysql.where")
	if err != nil {
		return "", nil, err
	}
	return "exists " + sql, args, nil
}

func (d dialect) compileCountCondition(condition oro.Condition) (string, []any, error) {
	countCondition, ok := condition.Value.(oro.CountCondition)
	if !ok || countCondition.Source == nil {
		return "", nil, &oro.Error{Op: "mysql.where", Kind: oro.ErrInvalidArgument}
	}
	sql, args, err := d.compileScalarSource(*countCondition.Source, "mysql.where")
	if err != nil {
		return "", nil, err
	}
	return sql + " " + countCondition.Op + " ?", append(args, countCondition.Value), nil
}

func (d dialect) compileScalarSource(source oro.SourceAST, op string) (string, []any, error) {
	switch {
	case source.Query != nil:
		compiled, err := d.compileSelect(*source.Query)
		if err != nil {
			return "", nil, err
		}
		return "(" + compiled.SQL + ")", compiled.Args, nil
	case source.Raw != nil:
		return "(" + source.Raw.SQL + ")", source.Raw.Args, nil
	default:
		return "", nil, &oro.Error{Op: op, Kind: oro.ErrInvalidArgument}
	}
}

func (d dialect) compileFullTextCondition(condition oro.Condition) (string, []any, error) {
	expr, ok := condition.Value.(oro.FullTextExpr)
	if !ok || len(expr.Fields) == 0 {
		return "", nil, &oro.Error{Op: "mysql.fulltext", Kind: oro.ErrInvalidArgument}
	}
	fields := make([]string, 0, len(expr.Fields))
	for _, field := range expr.Fields {
		fields = append(fields, d.QuoteIdent(field))
	}
	return "match(" + strings.Join(fields, ", ") + ") against (?)", []any{expr.Query}, nil
}

func (d dialect) compileFullTextScore(item oro.SelectExpr) (string, []any, error) {
	expr, ok := item.Args[0].(oro.FullTextExpr)
	if !ok || len(expr.Fields) == 0 {
		return "", nil, &oro.Error{Op: "mysql.fulltext", Kind: oro.ErrInvalidArgument}
	}
	fields := make([]string, 0, len(expr.Fields))
	for _, field := range expr.Fields {
		fields = append(fields, d.QuoteIdent(field))
	}
	return "match(" + strings.Join(fields, ", ") + ") against (?)", []any{expr.Query}, nil
}

func (d dialect) compileJSONCondition(condition oro.Condition) (string, []any, error) {
	jsonCondition, ok := condition.Value.(oro.JSONCondition)
	if !ok {
		return "", nil, &oro.Error{Op: "mysql.json", Kind: oro.ErrInvalidArgument}
	}
	expr := "json_extract(" + d.QuoteIdent(jsonCondition.Field) + ", ?)"
	path := mysqlJSONPath(jsonCondition.Parts)
	switch strings.ToLower(jsonCondition.Op) {
	case "=":
		value, err := jsonConditionArgument(jsonCondition.Value)
		if err != nil {
			return "", nil, err
		}
		return expr + " = cast(? as json)", []any{path, value}, nil
	case "!=":
		value, err := jsonConditionArgument(jsonCondition.Value)
		if err != nil {
			return "", nil, err
		}
		return expr + " != cast(? as json)", []any{path, value}, nil
	case "is null":
		return expr + " is null", []any{path}, nil
	case "is not null":
		return expr + " is not null", []any{path}, nil
	case "exists":
		return "json_contains_path(" + d.QuoteIdent(jsonCondition.Field) + ", 'one', ?) = 1", []any{path}, nil
	case "contains":
		value, err := jsonConditionArgument(jsonCondition.Value)
		if err != nil {
			return "", nil, err
		}
		return "json_contains(" + expr + ", cast(? as json)) = 1", []any{path, value}, nil
	default:
		return "", nil, &oro.Error{Op: "mysql.json", Kind: oro.ErrInvalidArgument}
	}
}

func jsonConditionArgument(value any) (string, error) {
	bytesValue, err := json.Marshal(value)
	if err != nil {
		return "", &oro.Error{Op: "mysql.json", Kind: oro.ErrInvalidArgument, Cause: err}
	}
	return string(bytesValue), nil
}

func mysqlJSONPath(parts []string) string {
	path := "$"
	for _, part := range parts {
		path += "." + part
	}
	return path
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
		return nil, &oro.Error{Op: "mysql.schema", Kind: oro.ErrInvalidArgument}
	}
	columns := make([]string, 0, len(table.Columns))
	for _, column := range table.Columns {
		sql, err := d.compileColumn(column, true)
		if err != nil {
			return nil, err
		}
		columns = append(columns, sql)
	}
	return []oro.CompiledSQL{{SQL: "create table if not exists " + d.QuoteIdent(table.Name) + " (" + strings.Join(columns, ", ") + ")"}}, nil
}

func (d dialect) compileColumn(column oro.ColumnSpec, allowPrimary bool) (string, error) {
	dataType, err := d.DataType(column)
	if err != nil {
		return "", err
	}
	parts := []string{d.QuoteIdent(column.ColumnName), dataType}
	if allowPrimary && column.Primary {
		parts = append(parts, "primary key auto_increment")
	}
	if !column.Nullable && !column.Primary {
		parts = append(parts, "not null")
	}
	if column.Default != nil {
		parts = append(parts, "default "+compileDefault(column.Default))
	}
	if column.Comment != "" {
		parts = append(parts, "comment "+schemaString(column.Comment))
	}
	return strings.Join(parts, " "), nil
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
	unique := ""
	if index.Unique {
		unique = "unique "
	}
	if index.FullText {
		return []oro.CompiledSQL{{SQL: "create fulltext index " + d.QuoteIdent(index.Name) + " on " + d.QuoteIdent(table) + " (" + strings.Join(columns, ", ") + ")"}}, nil
	}
	return []oro.CompiledSQL{{SQL: "create " + unique + "index " + d.QuoteIdent(index.Name) + " on " + d.QuoteIdent(table) + " (" + strings.Join(columns, ", ") + ")"}}, nil
}

func sortedKeys(row oro.Map) []string {
	keys := make([]string, 0, len(row))
	for key := range row {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
