package oro

import "context"

// ApplyMode identifies the operation currently being extended.
type ApplyMode string

// ApplyStage identifies the extension stage for an operation.
type ApplyStage string

const (
	// ApplyRead is used while building a read query.
	ApplyRead ApplyMode = "read"
	// ApplyInsert is used while inserting rows.
	ApplyInsert ApplyMode = "insert"
	// ApplyUpdate is used while updating rows.
	ApplyUpdate ApplyMode = "update"
	// ApplyDelete is used while deleting rows.
	ApplyDelete ApplyMode = "delete"
	// ApplyRestore is used while restoring soft-deleted rows.
	ApplyRestore ApplyMode = "restore"
	// ApplyAfterFind is used after a model is found.
	ApplyAfterFind ApplyMode = "after_find"
	// ApplyAfterWrite is used after a write completes.
	ApplyAfterWrite ApplyMode = "after_write"
)

const (
	// ApplyStageSpec lets extensions alter the query spec.
	ApplyStageSpec ApplyStage = "spec"
	// ApplyStageValues lets extensions alter write values.
	ApplyStageValues ApplyStage = "values"
	// ApplyStageResult lets extensions observe write results.
	ApplyStageResult ApplyStage = "result"
)

// Apply is implemented by query extensions applied with ModelQuery.Apply.
type Apply interface {
	ApplyOro(*ApplyContext) error
}

// ApplyFinalizer can run after all Apply hooks on the same query.
type ApplyFinalizer interface {
	AfterApplyOro(*ApplyContext) error
}

// ApplyFunc adapts a function to Apply.
type ApplyFunc func(*ApplyContext) error

// ApplyOro calls fn.
func (fn ApplyFunc) ApplyOro(ctx *ApplyContext) error {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

// ApplyContext exposes query/write state to Apply extensions.
type ApplyContext struct {
	Context context.Context
	DB      *DB
	Schema  *ModelSchema
	Spec    *QuerySpec
	Values  Map
	Model   any
	Mode    ApplyMode
	Stage   ApplyStage
	State   Map
	Rows    int64

	selectHidden *[]string
}

// IsQueryMode reports whether the context is at the query-spec stage.
func (ctx *ApplyContext) IsQueryMode() bool {
	switch ctx.Mode {
	case ApplyRead, ApplyUpdate, ApplyDelete, ApplyRestore:
		return ctx.Stage == ApplyStageSpec
	default:
		return false
	}
}

// Where adds an AND condition to the current query spec.
func (ctx *ApplyContext) Where(field any, args ...any) error {
	if ctx == nil || ctx.Spec == nil {
		return &Error{Op: "apply.where", Kind: ErrInvalidArgument}
	}
	conditions, err := appendWhereCondition(ctx.Spec.Where, "and", field, args...)
	if err != nil {
		return err
	}
	ctx.Spec.Where = conditions
	return nil
}

// OrWhere adds an OR condition to the current query spec.
func (ctx *ApplyContext) OrWhere(field any, args ...any) error {
	if ctx == nil || ctx.Spec == nil {
		return &Error{Op: "apply.where", Kind: ErrInvalidArgument}
	}
	conditions, err := appendWhereCondition(ctx.Spec.Where, "or", field, args...)
	if err != nil {
		return err
	}
	ctx.Spec.Where = conditions
	return nil
}

// Select appends select expressions to the current query spec.
func (ctx *ApplyContext) Select(items ...any) error {
	if ctx == nil || ctx.Spec == nil {
		return &Error{Op: "apply.select", Kind: ErrInvalidArgument}
	}
	exprs, err := selectExprs(items)
	if err != nil {
		return err
	}
	ctx.Spec.Select = append(ctx.Spec.Select, exprs...)
	return nil
}

// SelectHidden includes hidden fields in model query selection.
func (ctx *ApplyContext) SelectHidden(fields ...string) {
	if ctx == nil || ctx.selectHidden == nil {
		return
	}
	*ctx.selectHidden = append(*ctx.selectHidden, fields...)
}

// OrderBy appends ascending order expressions.
func (ctx *ApplyContext) OrderBy(fields ...string) {
	if ctx == nil || ctx.Spec == nil {
		return
	}
	ctx.Spec.Order = append(ctx.Spec.Order, orderExprs(false, fields)...)
}

// OrderByDesc appends descending order expressions.
func (ctx *ApplyContext) OrderByDesc(fields ...string) {
	if ctx == nil || ctx.Spec == nil {
		return
	}
	ctx.Spec.Order = append(ctx.Spec.Order, orderExprs(true, fields)...)
}

// FirstRowColumns reads the first matching row with selected columns.
func (ctx *ApplyContext) FirstRowColumns(columns ...string) (Map, error) {
	rows, err := ctx.RowsColumns(1, columns...)
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	row := rows[0]
	if ctx.Schema == nil {
		return row, nil
	}
	normalized := Map{}
	for _, fieldName := range columns {
		field, ok := ctx.Schema.FieldByGo[fieldName]
		if !ok {
			continue
		}
		if value, exists := row[field.Column]; exists {
			normalized[fieldName] = value
		}
	}
	if len(normalized) == 0 {
		return row, nil
	}
	return normalized, nil
}

// FirstWhereColumns reads selected columns for the first row matching field=value.
func (ctx *ApplyContext) FirstWhereColumns(field string, value any, columns ...string) (Map, error) {
	if ctx == nil || ctx.Spec == nil {
		return nil, &Error{Op: "apply.rows", Kind: ErrInvalidArgument}
	}
	spec := cloneQuerySpec(*ctx.Spec)
	spec.Where = nil
	spec.Order = nil
	spec.Group = nil
	spec.Having = nil
	spec.Limit = nil
	spec.Offset = nil
	spec.With = nil
	spec.Cache = CacheSpec{}
	ctxClone := *ctx
	ctxClone.Spec = &spec
	if err := ctxClone.Where(field, value); err != nil {
		return nil, err
	}
	if ctx.Schema != nil {
		converted, err := convertModelConditions(ctx.Schema, spec.Where)
		if err != nil {
			return nil, err
		}
		spec.Where = converted
	}
	return ctxClone.FirstRowColumns(columns...)
}

// CountRows counts rows for the current query spec.
func (ctx *ApplyContext) CountRows() (int64, error) {
	if ctx == nil || ctx.Spec == nil || ctx.DB == nil {
		return 0, &Error{Op: "apply.count", Kind: ErrInvalidArgument}
	}
	spec := cloneQuerySpec(*ctx.Spec)
	spec.With = nil
	spec.Cache = CacheSpec{}
	countSpec, err := countQuerySpec(spec)
	if err != nil {
		return 0, err
	}
	row, err := queryFirstRowPrepared(ctx.Context, ctx.DB, countSpec)
	if err != nil || row == nil {
		return 0, err
	}
	return rowInt64(row, "total")
}

// RowsColumns reads rows from the current query spec with selected columns.
func (ctx *ApplyContext) RowsColumns(limit int, columns ...string) ([]Map, error) {
	if ctx == nil || ctx.Spec == nil || ctx.DB == nil {
		return nil, &Error{Op: "apply.rows", Kind: ErrInvalidArgument}
	}
	spec := cloneQuerySpec(*ctx.Spec)
	spec.Select = spec.Select[:0]
	for _, column := range columns {
		expr := column
		if ctx.Schema != nil {
			if field, ok := ctx.Schema.FieldByGo[column]; ok {
				expr = field.Column
			}
		}
		spec.Select = append(spec.Select, SelectExpr{Expr: expr})
	}
	spec.With = nil
	spec.Order = nil
	spec.Cache = CacheSpec{}
	if limit > 0 {
		spec.Limit = &limit
	}
	return queryRowsPrepared(ctx.Context, ctx.DB, spec)
}

// Apply attaches extension logic to a model query.
func (query *ModelQuery[T]) Apply(applies ...Apply) *ModelQuery[T] {
	if len(applies) == 0 {
		return query
	}
	clone := *query
	clone.applies = append(append([]Apply(nil), query.applies...), applies...)
	return &clone
}

func (query *ModelQuery[T]) hasApplies() bool {
	return len(query.applies) > 0
}

func applyModelApplies[T any](ctx context.Context, query *ModelQuery[T], mode ApplyMode, stage ApplyStage, schema *ModelSchema, spec *QuerySpec, values Map, model any, selectHidden *[]string) error {
	if query == nil || len(query.applies) == 0 {
		return nil
	}
	applyCtx := &ApplyContext{
		Context:      ctx,
		DB:           query.db,
		Schema:       schema,
		Spec:         spec,
		Values:       values,
		Model:        model,
		Mode:         mode,
		Stage:        stage,
		State:        Map{},
		selectHidden: selectHidden,
	}
	for _, apply := range query.applies {
		if apply == nil {
			continue
		}
		if err := apply.ApplyOro(applyCtx); err != nil {
			return err
		}
	}
	for _, apply := range query.applies {
		finalizer, ok := apply.(ApplyFinalizer)
		if !ok || apply == nil {
			continue
		}
		if err := finalizer.AfterApplyOro(applyCtx); err != nil {
			return err
		}
	}
	return nil
}

func applyModelAfterWrite[T any](ctx context.Context, query *ModelQuery[T], schema *ModelSchema, spec *QuerySpec, values Map, model any, rows int64) error {
	return applyModelAfterWriteWithState(ctx, query, schema, spec, values, model, rows, nil)
}

func applyModelAfterWriteWithState[T any](ctx context.Context, query *ModelQuery[T], schema *ModelSchema, spec *QuerySpec, values Map, model any, rows int64, state Map) error {
	if query == nil || len(query.applies) == 0 {
		return nil
	}
	if state == nil {
		state = Map{}
	}
	applyDB := query.db
	if spec != nil {
		applyDB = withSpecConnection(query.db, *spec)
	}
	applyCtx := &ApplyContext{
		Context: ctx,
		DB:      applyDB,
		Schema:  schema,
		Spec:    spec,
		Values:  values,
		Model:   model,
		Mode:    ApplyAfterWrite,
		Stage:   ApplyStageResult,
		State:   state,
		Rows:    rows,
	}
	for _, apply := range query.applies {
		if apply == nil {
			continue
		}
		if err := apply.ApplyOro(applyCtx); err != nil {
			return err
		}
	}
	for _, apply := range query.applies {
		finalizer, ok := apply.(ApplyFinalizer)
		if !ok || apply == nil {
			continue
		}
		if err := finalizer.AfterApplyOro(applyCtx); err != nil {
			return err
		}
	}
	return nil
}
