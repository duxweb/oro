package oro

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/duxweb/oro/internal/fifocache"
)

type sqlCache struct {
	items *fifocache.Cache[string, string]
}

func newSQLCache(maxSize int) *sqlCache {
	if maxSize <= 0 {
		return nil
	}
	return &sqlCache{items: fifocache.New[string, string](maxSize, nil)}
}

func (cache *sqlCache) get(key string) (string, bool) {
	if cache == nil || cache.items == nil || key == "" {
		return "", false
	}
	return cache.items.Get(key)
}

func (cache *sqlCache) set(key string, sql string) {
	if cache == nil || cache.items == nil || key == "" || sql == "" {
		return
	}
	cache.items.Set(key, sql)
}

func compileSelectSQL(db *DB, conn *Connection, spec QuerySpec) (CompiledSQL, error) {
	statement, err := db.runtime.Planner.BuildSelect(spec)
	if err != nil {
		return CompiledSQL{}, err
	}
	return compileStatementSQL(db, conn, statement)
}

func compileInsertSQL(db *DB, conn *Connection, spec WriteSpec) (CompiledSQL, error) {
	statement, err := db.runtime.Planner.BuildInsert(spec)
	if err != nil {
		return CompiledSQL{}, err
	}
	return compileStatementSQL(db, conn, statement)
}

func compileUpsertSQL(db *DB, conn *Connection, spec WriteSpec) (CompiledSQL, error) {
	statement, err := db.runtime.Planner.BuildUpsert(spec)
	if err != nil {
		return CompiledSQL{}, err
	}
	return compileStatementSQL(db, conn, statement)
}

func compileUpdateSQL(db *DB, conn *Connection, spec WriteSpec) (CompiledSQL, error) {
	statement, err := db.runtime.Planner.BuildUpdate(spec)
	if err != nil {
		return CompiledSQL{}, err
	}
	return compileStatementSQL(db, conn, statement)
}

func compileDeleteSQL(db *DB, conn *Connection, spec WriteSpec) (CompiledSQL, error) {
	statement, err := db.runtime.Planner.BuildDelete(spec)
	if err != nil {
		return CompiledSQL{}, err
	}
	return compileStatementSQL(db, conn, statement)
}

func compileStatementSQL(db *DB, conn *Connection, statement Statement) (CompiledSQL, error) {
	if conn == nil {
		return CompiledSQL{}, &Error{Op: "compile", Kind: ErrInvalidArgument}
	}
	key, args, ok := lightweightSQLCacheKey(conn.Dialect.Name(), statement)
	if ok {
		if sql, hit := db.runtime.SQLCache.get(key); hit {
			return CompiledSQL{SQL: sql, Args: args}, nil
		}
	}
	compiled, err := conn.Dialect.Compile(statement)
	if err != nil {
		return CompiledSQL{}, err
	}
	if ok {
		db.runtime.SQLCache.set(key, compiled.SQL)
	}
	return compiled, nil
}

func lightweightSQLCacheKey(dialect string, statement Statement) (string, []any, bool) {
	switch stmt := statement.(type) {
	case SelectAST:
		return selectSQLCacheKey(dialect, stmt)
	case InsertAST:
		return insertSQLCacheKey(dialect, stmt)
	case UpdateAST:
		return updateSQLCacheKey(dialect, stmt)
	case DeleteAST:
		return deleteSQLCacheKey(dialect, stmt)
	default:
		return "", nil, false
	}
}

func selectSQLCacheKey(dialect string, stmt SelectAST) (string, []any, bool) {
	if stmt.From.Table != "" || stmt.From.Query != nil || stmt.From.Raw != nil || len(stmt.Joins) > 0 || len(stmt.Having) > 0 || len(stmt.Group) > 0 || stmt.Lock.Mode != LockNone {
		return "", nil, false
	}
	if hasRawSelectOrOrder(stmt.Select, stmt.Order) {
		return "", nil, false
	}
	whereArgs, whereShape, ok := simpleConditionsArgs(dialect, stmt.Where)
	if !ok {
		return "", nil, false
	}
	key := strings.Builder{}
	key.WriteString("s|")
	key.WriteString(dialect)
	key.WriteByte('|')
	key.WriteString(stmt.Table)
	key.WriteByte('|')
	key.WriteString(stmt.Alias)
	key.WriteString("|sel:")
	appendSelectShape(&key, stmt.Select)
	key.WriteString("|w:")
	key.WriteString(whereShape)
	key.WriteString("|o:")
	appendOrderShape(&key, stmt.Order)
	key.WriteString("|l:")
	appendIntPointerShape(&key, stmt.Limit)
	key.WriteString("|f:")
	appendIntPointerShape(&key, stmt.Offset)
	return key.String(), whereArgs, true
}

func insertSQLCacheKey(dialect string, stmt InsertAST) (string, []any, bool) {
	if len(stmt.Values) == 0 || len(stmt.Conflict.Columns) > 0 {
		return "", nil, false
	}
	row := stmt.Values[0]
	keys := sortedSQLMapKeys(row)
	args := make([]any, 0, len(keys)*len(stmt.Values))
	key := strings.Builder{}
	key.WriteString("i|")
	key.WriteString(dialect)
	key.WriteByte('|')
	key.WriteString(stmt.Table)
	key.WriteString("|r:")
	if stmt.Returning {
		key.WriteByte('1')
	} else {
		key.WriteByte('0')
	}
	key.WriteString("|n:")
	key.WriteString(strconv.Itoa(len(stmt.Values)))
	for _, column := range keys {
		key.WriteByte('|')
		key.WriteString(column)
	}
	for _, value := range stmt.Values {
		if len(value) != len(keys) {
			return "", nil, false
		}
		for _, column := range keys {
			item, ok := value[column]
			if !ok {
				return "", nil, false
			}
			args = append(args, item)
		}
	}
	return key.String(), args, true
}

func updateSQLCacheKey(dialect string, stmt UpdateAST) (string, []any, bool) {
	if len(stmt.Values) == 0 {
		return "", nil, false
	}
	keys := sortedSQLMapKeys(stmt.Values)
	args := make([]any, 0, len(keys)+len(stmt.Where))
	key := strings.Builder{}
	key.WriteString("u|")
	key.WriteString(dialect)
	key.WriteByte('|')
	key.WriteString(stmt.Table)
	key.WriteString("|set:")
	for _, column := range keys {
		valueArgs, valueShape, ok := simpleSetValueArgs(stmt.Values[column])
		if !ok {
			return "", nil, false
		}
		key.WriteByte('|')
		key.WriteString(column)
		key.WriteByte(':')
		key.WriteString(valueShape)
		args = append(args, valueArgs...)
	}
	whereArgs, whereShape, ok := simpleConditionsArgs(dialect, stmt.Where)
	if !ok {
		return "", nil, false
	}
	key.WriteString("|w:")
	key.WriteString(whereShape)
	args = append(args, whereArgs...)
	return key.String(), args, true
}

func deleteSQLCacheKey(dialect string, stmt DeleteAST) (string, []any, bool) {
	whereArgs, whereShape, ok := simpleConditionsArgs(dialect, stmt.Where)
	if !ok {
		return "", nil, false
	}
	key := "d|" + dialect + "|" + stmt.Table + "|w:" + whereShape
	return key, whereArgs, true
}

func hasRawSelectOrOrder(selects []SelectExpr, orders []OrderExpr) bool {
	for _, item := range selects {
		if item.Raw || item.Source != nil || item.Expr == "__oro_relation_exists__" || item.Expr == "__oro_fulltext_score__" {
			return true
		}
	}
	for _, item := range orders {
		if item.Raw || len(item.Args) > 0 {
			return true
		}
	}
	return false
}

func appendSelectShape(builder *strings.Builder, selects []SelectExpr) {
	for _, item := range selects {
		builder.WriteByte('|')
		builder.WriteString(item.Expr)
		builder.WriteByte(':')
		builder.WriteString(item.Alias)
	}
}

func appendOrderShape(builder *strings.Builder, orders []OrderExpr) {
	for _, item := range orders {
		builder.WriteByte('|')
		builder.WriteString(item.Expr)
		if item.Desc {
			builder.WriteString(":d")
		} else {
			builder.WriteString(":a")
		}
	}
}

func appendIntPointerShape(builder *strings.Builder, value *int) {
	if value == nil {
		builder.WriteByte('-')
		return
	}
	builder.WriteString("v:")
	builder.WriteString(intString(*value))
}

func simpleConditionsArgs(dialect string, conditions []Condition) ([]any, string, bool) {
	args := []any{}
	shape := strings.Builder{}
	for _, condition := range conditions {
		conditionArgs, conditionShape, ok := simpleConditionArgs(dialect, condition)
		if !ok {
			return nil, "", false
		}
		shape.WriteByte('[')
		shape.WriteString(condition.Bool)
		shape.WriteByte('|')
		shape.WriteString(conditionShape)
		shape.WriteByte(']')
		args = append(args, conditionArgs...)
	}
	return args, shape.String(), true
}

func simpleConditionArgs(dialect string, condition Condition) ([]any, string, bool) {
	op := strings.ToLower(strings.TrimSpace(condition.Op))
	switch op {
	case "", "invalid", "raw", "in", "exists", "count", "fulltext":
		return nil, "", false
	case "group", "not":
		args, shape, ok := simpleConditionsArgs(dialect, condition.Conditions)
		if !ok {
			return nil, "", false
		}
		return args, op + "{" + shape + "}", true
	case "column":
		columnCondition, ok := condition.Value.(ColumnCondition)
		if !ok {
			return nil, "", false
		}
		return nil, condition.Field + "|col|" + columnCondition.Op + "|" + columnCondition.Right, true
	case "json":
		jsonCondition, ok := condition.Value.(JSONCondition)
		if !ok {
			return nil, "", false
		}
		args, ok := simpleJSONArgs(dialect, jsonCondition)
		if !ok {
			return nil, "", false
		}
		return args, condition.Field + "|json|" + jsonCondition.Op + "|" + strings.Join(jsonCondition.Parts, "."), true
	case "in_values", "not_in_values", "between":
		values, ok := condition.Value.([]any)
		if !ok {
			return nil, "", false
		}
		return append([]any(nil), values...), condition.Field + "|" + op + "|" + intString(len(values)), true
	case "is null", "is not null":
		return nil, condition.Field + "|" + op, true
	default:
		if _, ok := condition.Value.(*SourceAST); ok {
			return nil, "", false
		}
		return []any{condition.Value}, condition.Field + "|" + condition.Op, true
	}
}

func simpleSetValueArgs(value any) ([]any, string, bool) {
	switch expr := value.(type) {
	case IncrementExpr:
		return []any{expr.Value}, "inc", true
	case DecrementExpr:
		return []any{expr.Value}, "dec", true
	case RawExpr:
		if expr.SQL == "" {
			return nil, "", false
		}
		return append([]any(nil), expr.Args...), "raw:" + expr.SQL + ":" + intString(len(expr.Args)), true
	default:
		return []any{value}, "value", true
	}
}

func simpleJSONArgs(dialect string, condition JSONCondition) ([]any, bool) {
	path := simpleJSONPath(dialect, condition.Parts)
	switch strings.ToLower(condition.Op) {
	case "=", "!=", "contains":
		value := condition.Value
		if dialect == "mysql" || dialect == "pgsql" {
			bytesValue, err := json.Marshal(condition.Value)
			if err != nil {
				return nil, false
			}
			value = string(bytesValue)
		}
		return []any{path, value}, true
	case "is null", "is not null", "exists":
		return []any{path}, true
	default:
		return nil, false
	}
}

func simpleJSONPath(dialect string, parts []string) string {
	if dialect == "pgsql" {
		if len(parts) == 0 {
			return "{}"
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	path := "$"
	for _, part := range parts {
		path += "." + part
	}
	return path
}

func sortedSQLMapKeys(values Map) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	index := len(buf)
	for value > 0 {
		index--
		buf[index] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		index--
		buf[index] = '-'
	}
	return string(buf[index:])
}
