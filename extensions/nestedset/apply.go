package nestedset

import "github.com/duxweb/oro"

type Apply struct {
	config Config
	scope  oro.Map
	ops    []applyOp
}

type applyOp struct {
	name  string
	args  []any
	order string
	desc  bool
}

func Configured(options ...Option) Apply {
	return Apply{config: resolveConfig(options)}
}

func ScopeApply(values oro.Map) Apply {
	return Apply{scope: copyMap(values)}
}

func Roots(options ...Option) Apply {
	return Apply{config: resolveConfig(options), ops: []applyOp{{name: "roots"}}}
}

func Depth(depth int, options ...Option) Apply {
	return Apply{config: resolveConfig(options), ops: []applyOp{{name: "depth", args: []any{depth}}}}
}

func DepthGte(depth int, options ...Option) Apply {
	return Apply{config: resolveConfig(options), ops: []applyOp{{name: "depth_gte", args: []any{depth}}}}
}

func DepthLte(depth int, options ...Option) Apply {
	return Apply{config: resolveConfig(options), ops: []applyOp{{name: "depth_lte", args: []any{depth}}}}
}

func DefaultOrder(options ...Option) Apply {
	config := resolveConfig(options)
	return Apply{config: config, ops: []applyOp{{name: "order", order: config.LeftField}}}
}

func Reversed(options ...Option) Apply {
	config := resolveConfig(options)
	return Apply{config: config, ops: []applyOp{{name: "order", order: config.LeftField, desc: true}}}
}

func AncestorsOf(node any, options ...Option) Apply {
	return boundsApply("ancestors", node, options...)
}

func AncestorsAndSelfOf(node any, options ...Option) Apply {
	return boundsApply("ancestors_self", node, options...)
}

func DescendantsOf(node any, options ...Option) Apply {
	return boundsApply("descendants", node, options...)
}

func DescendantsAndSelfOf(node any, options ...Option) Apply {
	return boundsApply("descendants_self", node, options...)
}

func DescendantsWithinDepthOf(node any, depth int, options ...Option) Apply {
	apply := boundsApply("descendants_depth_lte", node, options...)
	apply.ops[0].args = append(apply.ops[0].args, depth)
	return apply
}

func DescendantsAtDepthOf(node any, depth int, options ...Option) Apply {
	apply := boundsApply("descendants_depth_eq", node, options...)
	apply.ops[0].args = append(apply.ops[0].args, depth)
	return apply
}

func SiblingsOf(node any, options ...Option) Apply {
	return boundsApply("siblings", node, options...)
}

func boundsApply(name string, node any, options ...Option) Apply {
	return Apply{config: resolveConfig(options), ops: []applyOp{{name: name, args: []any{node}}}}
}

func (apply Apply) ApplyOro(ctx *oro.ApplyContext) error {
	if ctx == nil || ctx.Stage != oro.ApplyStageSpec || !ctx.IsQueryMode() {
		return nil
	}
	config := apply.config
	if config.ParentField == "" || config.LeftField == "" || config.RightField == "" || config.DepthField == "" {
		config = resolveConfig(nil)
	}
	for field, value := range config.Scope {
		if err := ctx.Where(field, value); err != nil {
			return err
		}
	}
	for field, value := range apply.scope {
		if err := ctx.Where(field, value); err != nil {
			return err
		}
	}
	for _, op := range apply.ops {
		if err := applyOpToContext(ctx, config, op); err != nil {
			return err
		}
	}
	return nil
}

func applyOpToContext(ctx *oro.ApplyContext, config Config, op applyOp) error {
	switch op.name {
	case "roots":
		return ctx.Where(oro.Field(config.ParentField).IsNull())
	case "depth":
		return ctx.Where(config.DepthField, op.args[0])
	case "depth_gte":
		return ctx.Where(config.DepthField, ">=", op.args[0])
	case "depth_lte":
		return ctx.Where(config.DepthField, "<=", op.args[0])
	case "order":
		if op.desc {
			ctx.OrderByDesc(op.order)
		} else {
			ctx.OrderBy(op.order)
		}
		return nil
	case "ancestors", "ancestors_self", "descendants", "descendants_self", "descendants_depth_lte", "descendants_depth_eq", "siblings":
		row, err := treeRowFromApplyNode(ctx, config, op.args[0])
		if err != nil {
			return err
		}
		return applyNodeBounds(ctx, config, op, row)
	default:
		return nil
	}
}

func applyNodeBounds(ctx *oro.ApplyContext, config Config, op applyOp, row *treeRow) error {
	switch op.name {
	case "ancestors":
		if err := ctx.Where(config.LeftField, "<", row.Lft); err != nil {
			return err
		}
		return ctx.Where(config.RightField, ">", row.Rgt)
	case "ancestors_self":
		if err := ctx.Where(config.LeftField, "<=", row.Lft); err != nil {
			return err
		}
		return ctx.Where(config.RightField, ">=", row.Rgt)
	case "descendants":
		if err := ctx.Where(config.LeftField, ">", row.Lft); err != nil {
			return err
		}
		return ctx.Where(config.RightField, "<", row.Rgt)
	case "descendants_self":
		if err := ctx.Where(config.LeftField, ">=", row.Lft); err != nil {
			return err
		}
		return ctx.Where(config.RightField, "<=", row.Rgt)
	case "descendants_depth_lte":
		depth, ok := op.args[1].(int)
		if !ok || depth < 0 {
			return &oro.Error{Op: "nestedset.descendants", Kind: oro.ErrInvalidArgument, Field: "depth"}
		}
		if err := ctx.Where(config.LeftField, ">", row.Lft); err != nil {
			return err
		}
		if err := ctx.Where(config.RightField, "<", row.Rgt); err != nil {
			return err
		}
		return ctx.Where(config.DepthField, "<=", row.Depth+depth)
	case "descendants_depth_eq":
		depth, ok := op.args[1].(int)
		if !ok || depth < 0 {
			return &oro.Error{Op: "nestedset.descendants", Kind: oro.ErrInvalidArgument, Field: "depth"}
		}
		if depth == 0 {
			return ctx.Where(primaryField(ctx.DB), -1)
		}
		if err := ctx.Where(config.LeftField, ">", row.Lft); err != nil {
			return err
		}
		if err := ctx.Where(config.RightField, "<", row.Rgt); err != nil {
			return err
		}
		return ctx.Where(config.DepthField, row.Depth+depth)
	case "siblings":
		if err := ctx.Where(primaryField(ctx.DB), "!=", row.ID); err != nil {
			return err
		}
		if row.ParentID.Valid {
			return ctx.Where(config.ParentField, row.ParentID.Value)
		}
		return ctx.Where(config.ParentField, nil)
	default:
		return nil
	}
}

func treeRowFromApplyNode(ctx *oro.ApplyContext, config Config, node any) (*treeRow, error) {
	switch typed := node.(type) {
	case *treeRow:
		return typed, nil
	case treeRow:
		return &typed, nil
	default:
		row, err := treeRowFromModelLike(node)
		if err != nil {
			return nil, err
		}
		if row.ID == 0 || ctx == nil {
			return row, nil
		}
		current, err := ctx.FirstWhereColumns(primaryField(ctx.DB), row.ID, primaryField(ctx.DB), config.ParentField, config.LeftField, config.RightField, config.DepthField)
		if err != nil || current == nil {
			return row, err
		}
		return treeRowFromMap(config, current, row)
	}
}

func treeRowFromMap(config Config, values oro.Map, fallback *treeRow) (*treeRow, error) {
	row := *fallback
	if value, ok := values[primaryField(nil)]; ok {
		id, err := uint64FromAny(value)
		if err != nil {
			return nil, err
		}
		row.ID = id
	}
	if value, ok := values[config.ParentField]; ok {
		if value == nil {
			row.ParentID = oro.NullZero[uint64]()
		} else {
			parent, err := uint64FromAny(value)
			if err != nil {
				return nil, err
			}
			row.ParentID = oro.NullOf(parent)
		}
	}
	if value, ok := values[config.LeftField]; ok {
		left, err := intFromAny(value)
		if err != nil {
			return nil, err
		}
		row.Lft = left
	}
	if value, ok := values[config.RightField]; ok {
		right, err := intFromAny(value)
		if err != nil {
			return nil, err
		}
		row.Rgt = right
	}
	if value, ok := values[config.DepthField]; ok {
		depth, err := intFromAny(value)
		if err != nil {
			return nil, err
		}
		row.Depth = depth
	}
	return &row, nil
}

func intFromAny(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case uint:
		return int(typed), nil
	case uint8:
		return int(typed), nil
	case uint16:
		return int(typed), nil
	case uint32:
		return int(typed), nil
	case uint64:
		return int(typed), nil
	default:
		return 0, &oro.Error{Op: "nestedset.apply", Kind: oro.ErrInvalidArgument}
	}
}

func treeRowFromModelLike(model any) (*treeRow, error) {
	id, err := modelID(model)
	if err != nil {
		return nil, err
	}
	parentID, err := fieldNullUint64(model, "ParentID")
	if err != nil {
		return nil, err
	}
	left, err := fieldInt(model, "Lft")
	if err != nil {
		return nil, err
	}
	right, err := fieldInt(model, "Rgt")
	if err != nil {
		return nil, err
	}
	depth, err := fieldInt(model, "Depth")
	if err != nil {
		return nil, err
	}
	return &treeRow{ID: id, ParentID: parentID, Lft: left, Rgt: right, Depth: depth}, nil
}
