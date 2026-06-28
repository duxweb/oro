package oro

import "context"

type ApplyMode string
type ApplyStage string

const (
	ApplyRead      ApplyMode = "read"
	ApplyInsert    ApplyMode = "insert"
	ApplyUpdate    ApplyMode = "update"
	ApplyDelete    ApplyMode = "delete"
	ApplyRestore   ApplyMode = "restore"
	ApplyAfterFind ApplyMode = "after_find"
)

const (
	ApplyStageSpec   ApplyStage = "spec"
	ApplyStageValues ApplyStage = "values"
	ApplyStageResult ApplyStage = "result"
)

type Apply interface {
	ApplyOro(*ApplyContext) error
}

type ApplyFinalizer interface {
	AfterApplyOro(*ApplyContext) error
}

type ApplyFunc func(*ApplyContext) error

func (fn ApplyFunc) ApplyOro(ctx *ApplyContext) error {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

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

	selectHidden *[]string
}

func (ctx *ApplyContext) IsQueryMode() bool {
	switch ctx.Mode {
	case ApplyRead, ApplyUpdate, ApplyDelete, ApplyRestore:
		return ctx.Stage == ApplyStageSpec
	default:
		return false
	}
}

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

func (ctx *ApplyContext) SelectHidden(fields ...string) {
	if ctx == nil || ctx.selectHidden == nil {
		return
	}
	*ctx.selectHidden = append(*ctx.selectHidden, fields...)
}

func (ctx *ApplyContext) OrderBy(fields ...string) {
	if ctx == nil || ctx.Spec == nil {
		return
	}
	ctx.Spec.Order = append(ctx.Spec.Order, orderExprs(false, fields)...)
}

func (ctx *ApplyContext) OrderByDesc(fields ...string) {
	if ctx == nil || ctx.Spec == nil {
		return
	}
	ctx.Spec.Order = append(ctx.Spec.Order, orderExprs(true, fields)...)
}

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
