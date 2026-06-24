package oro

import "strings"

type tableNameResolver struct {
	prefix string
}

type tableNameContext struct {
	resolver *tableNameResolver
	tables   map[string]string
}

func newTableNameResolver(config Config) *tableNameResolver {
	return &tableNameResolver{prefix: config.TablePrefix}
}

func tableNames(db *DB) *tableNameResolver {
	if db == nil || db.runtime == nil || db.runtime.tableNames == nil {
		return newTableNameResolver(Config{})
	}
	return db.runtime.tableNames
}

func (resolver *tableNameResolver) Physical(logical string) string {
	if resolver == nil || resolver.prefix == "" || logical == "" {
		return logical
	}
	if strings.HasPrefix(logical, resolver.prefix) {
		return logical
	}
	parts := strings.Split(logical, ".")
	name := parts[len(parts)-1]
	if strings.HasPrefix(name, resolver.prefix) {
		return logical
	}
	parts[len(parts)-1] = resolver.prefix + name
	return strings.Join(parts, ".")
}

func (resolver *tableNameResolver) Snapshot() string {
	return resolver.Physical(schemaSnapshotTable)
}

func (resolver *tableNameResolver) Qualifier(spec QuerySpec, logicalTable string) string {
	if spec.Alias != "" {
		return spec.Alias
	}
	return resolver.Physical(logicalTable)
}

func (resolver *tableNameResolver) ApplyQuery(spec *QuerySpec) {
	if spec == nil {
		return
	}
	if resolver == nil || resolver.prefix == "" {
		return
	}
	context := resolver.queryContext(*spec)
	spec.Table = resolver.Physical(spec.Table)
	resolver.applySource(&spec.From)
	for index := range spec.Select {
		resolver.applySelectExpr(context, &spec.Select[index])
	}
	resolver.applyConditions(context, spec.Where)
	resolver.applyConditions(context, spec.Having)
	for index := range spec.Joins {
		resolver.applyJoinConditions(context, spec.Joins[index].Conditions)
		spec.Joins[index].Table = resolver.Physical(spec.Joins[index].Table)
		resolver.applySource(&spec.Joins[index].Source)
	}
	for index := range spec.Group {
		spec.Group[index] = context.identifier(spec.Group[index])
	}
	for index := range spec.Order {
		if !spec.Order[index].Raw {
			spec.Order[index].Expr = context.identifier(spec.Order[index].Expr)
		}
	}
}

func (resolver *tableNameResolver) ApplyWrite(spec *WriteSpec) {
	if spec == nil {
		return
	}
	if resolver == nil || resolver.prefix == "" {
		return
	}
	resolver.ApplyQuery(&spec.QuerySpec)
	spec.Table = resolver.Physical(spec.Table)
}

func (resolver *tableNameResolver) ApplySource(source *SourceAST) {
	if resolver == nil || resolver.prefix == "" {
		return
	}
	resolver.applySource(source)
}

func (resolver *tableNameResolver) ApplySelect(ast *SelectAST) {
	if resolver == nil || resolver.prefix == "" {
		return
	}
	resolver.applySelect(ast)
}

func (resolver *tableNameResolver) ApplyTableSpec(table *TableSpec) {
	if table == nil {
		return
	}
	table.Name = resolver.Physical(table.Name)
	for index := range table.Indexes {
		table.Indexes[index].Name = resolver.Physical(table.Indexes[index].Name)
	}
}

func (resolver *tableNameResolver) queryContext(spec QuerySpec) tableNameContext {
	context := tableNameContext{resolver: resolver, tables: map[string]string{}}
	context.addTable(spec.Table, spec.Alias)
	context.addSource(spec.From)
	for _, join := range spec.Joins {
		alias := join.Alias
		if alias == "" {
			alias = join.Source.Alias
		}
		context.addTable(join.Table, alias)
		context.addSource(join.Source)
	}
	return context
}

func (resolver *tableNameResolver) selectContext(ast SelectAST) tableNameContext {
	context := tableNameContext{resolver: resolver, tables: map[string]string{}}
	context.addTable(ast.Table, ast.Alias)
	context.addSource(ast.From)
	for _, join := range ast.Joins {
		alias := join.Alias
		if alias == "" {
			alias = join.Source.Alias
		}
		context.addTable(join.Table, alias)
		context.addSource(join.Source)
	}
	return context
}

func (context tableNameContext) addSource(source SourceAST) {
	context.addTable(source.Table, source.Alias)
}

func (context tableNameContext) addTable(logical string, alias string) {
	if context.resolver == nil || context.resolver.prefix == "" || logical == "" || alias != "" {
		return
	}
	physical := context.resolver.Physical(logical)
	if physical == logical {
		return
	}
	context.tables[logical] = physical
	logicalName := lastTableNamePart(logical)
	physicalName := lastTableNamePart(physical)
	if logicalName != "" && physicalName != "" && logicalName != logical {
		context.tables[logicalName] = physicalName
	}
}

func (context tableNameContext) identifier(identifier string) string {
	if context.resolver == nil || context.resolver.prefix == "" || identifier == "" || len(context.tables) == 0 {
		return identifier
	}
	parts := strings.Split(identifier, ".")
	if len(parts) < 2 {
		return identifier
	}
	qualifier := strings.Join(parts[:len(parts)-1], ".")
	replacement, ok := context.tables[qualifier]
	if !ok {
		return identifier
	}
	return replacement + "." + parts[len(parts)-1]
}

func (resolver *tableNameResolver) applySource(source *SourceAST) {
	if source == nil {
		return
	}
	source.Table = resolver.Physical(source.Table)
	if source.Query != nil {
		resolver.applySelect(source.Query)
	}
}

func (resolver *tableNameResolver) applySelect(ast *SelectAST) {
	if ast == nil {
		return
	}
	context := resolver.selectContext(*ast)
	ast.Table = resolver.Physical(ast.Table)
	resolver.applySource(&ast.From)
	for index := range ast.Select {
		resolver.applySelectExpr(context, &ast.Select[index])
	}
	resolver.applyConditions(context, ast.Where)
	resolver.applyConditions(context, ast.Having)
	for index := range ast.Joins {
		resolver.applyJoinConditions(context, ast.Joins[index].Conditions)
		ast.Joins[index].Table = resolver.Physical(ast.Joins[index].Table)
		resolver.applySource(&ast.Joins[index].Source)
	}
	for index := range ast.Group {
		ast.Group[index] = context.identifier(ast.Group[index])
	}
	for index := range ast.Order {
		if !ast.Order[index].Raw {
			ast.Order[index].Expr = context.identifier(ast.Order[index].Expr)
		}
	}
}

func (resolver *tableNameResolver) applySelectExpr(context tableNameContext, item *SelectExpr) {
	if item == nil {
		return
	}
	if item.Source != nil {
		resolver.applySource(item.Source)
		return
	}
	switch item.Expr {
	case "__oro_relation_exists__":
		if len(item.Args) == 1 {
			switch source := item.Args[0].(type) {
			case SourceAST:
				resolver.applySource(&source)
				item.Args[0] = source
			case *SourceAST:
				resolver.applySource(source)
			}
		}
	case "__oro_aggregate__":
		if len(item.Args) > 0 {
			if expr, ok := item.Args[0].(AggregateExpr); ok {
				expr.Field = context.identifier(expr.Field)
				item.Args[0] = expr
			}
		}
	case "__oro_fulltext_score__":
		if len(item.Args) > 0 {
			if expr, ok := item.Args[0].(FullTextExpr); ok {
				item.Args[0] = context.fullText(expr)
			}
		}
	default:
		if !item.Raw {
			item.Expr = context.identifier(item.Expr)
		}
	}
}

func (resolver *tableNameResolver) applyConditions(context tableNameContext, conditions []Condition) {
	for index := range conditions {
		resolver.applyConditions(context, conditions[index].Conditions)
		op := strings.ToLower(strings.TrimSpace(conditions[index].Op))
		if op != "raw" {
			conditions[index].Field = context.identifier(conditions[index].Field)
		}
		switch value := conditions[index].Value.(type) {
		case ColumnCondition:
			value.Right = context.identifier(value.Right)
			conditions[index].Value = value
		case CountCondition:
			if value.Source != nil {
				resolver.applySource(value.Source)
				conditions[index].Value = value
			}
		case JSONCondition:
			value.Field = context.identifier(value.Field)
			conditions[index].Value = value
		case FullTextExpr:
			conditions[index].Value = context.fullText(value)
		case *SourceAST:
			resolver.applySource(value)
		case SourceAST:
			resolver.applySource(&value)
			conditions[index].Value = value
		}
	}
}

func (resolver *tableNameResolver) applyJoinConditions(context tableNameContext, conditions []JoinCondition) {
	for index := range conditions {
		if len(conditions[index].Group) > 0 {
			resolver.applyJoinConditions(context, conditions[index].Group)
			continue
		}
		conditions[index].Left = context.identifier(conditions[index].Left)
		if conditions[index].Column {
			conditions[index].Right = context.identifier(conditions[index].Right)
		}
	}
}

func (context tableNameContext) fullText(expr FullTextExpr) FullTextExpr {
	for index := range expr.Fields {
		expr.Fields[index] = context.identifier(expr.Fields[index])
	}
	return expr
}

func lastTableNamePart(value string) string {
	if value == "" {
		return ""
	}
	parts := strings.Split(value, ".")
	return parts[len(parts)-1]
}
