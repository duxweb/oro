package nestedset

import (
	"context"

	"github.com/duxweb/oro"
)

type Query[T any] struct {
	tree  *Tree[T]
	query *oro.ModelQuery[T]
}

func (query *Query[T]) Where(field any, args ...any) *Query[T] {
	clone := *query
	clone.query = clone.query.Where(field, args...)
	return &clone
}

func (query *Query[T]) Select(items ...any) *Query[T] {
	clone := *query
	clone.query = clone.query.Select(items...)
	return &clone
}

func (query *Query[T]) Limit(limit int) *Query[T] {
	clone := *query
	clone.query = clone.query.Limit(limit)
	return &clone
}

func (query *Query[T]) Offset(offset int) *Query[T] {
	clone := *query
	clone.query = clone.query.Offset(offset)
	return &clone
}

func (query *Query[T]) Roots() *Query[T] {
	clone := *query
	clone.query = clone.query.Where(query.tree.config.ParentField, nil)
	return &clone
}

func (query *Query[T]) AncestorsOf(ctx context.Context, nodeID any) (*Query[T], error) {
	node, err := query.tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return query.empty(), err
	}
	clone := *query
	clone.query = clone.query.Where(query.tree.config.LeftField, "<", node.Lft).Where(query.tree.config.RightField, ">", node.Rgt)
	return &clone, nil
}

func (query *Query[T]) AncestorsAndSelfOf(ctx context.Context, nodeID any) (*Query[T], error) {
	node, err := query.tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return query.empty(), err
	}
	clone := *query
	clone.query = clone.query.Where(query.tree.config.LeftField, "<=", node.Lft).Where(query.tree.config.RightField, ">=", node.Rgt)
	return &clone, nil
}

func (query *Query[T]) DescendantsOf(ctx context.Context, nodeID any) (*Query[T], error) {
	node, err := query.tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return query.empty(), err
	}
	clone := *query
	clone.query = clone.query.Where(query.tree.config.LeftField, ">", node.Lft).Where(query.tree.config.RightField, "<", node.Rgt)
	return &clone, nil
}

func (query *Query[T]) DescendantsWithinDepthOf(ctx context.Context, nodeID any, maxDepth int) (*Query[T], error) {
	if maxDepth < 0 {
		return query.empty(), &oro.Error{Op: "nestedset.descendants", Kind: oro.ErrInvalidArgument, Field: "depth"}
	}
	node, err := query.tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return query.empty(), err
	}
	clone := *query
	clone.query = clone.query.
		Where(query.tree.config.LeftField, ">", node.Lft).
		Where(query.tree.config.RightField, "<", node.Rgt).
		Where(query.tree.config.DepthField, "<=", node.Depth+maxDepth)
	return &clone, nil
}

func (query *Query[T]) DescendantsAtDepthOf(ctx context.Context, nodeID any, depth int) (*Query[T], error) {
	if depth < 0 {
		return query.empty(), &oro.Error{Op: "nestedset.descendants", Kind: oro.ErrInvalidArgument, Field: "depth"}
	}
	node, err := query.tree.node(ctx, nodeID)
	if err != nil || node == nil || depth == 0 {
		return query.empty(), err
	}
	clone := *query
	clone.query = clone.query.
		Where(query.tree.config.LeftField, ">", node.Lft).
		Where(query.tree.config.RightField, "<", node.Rgt).
		Where(query.tree.config.DepthField, node.Depth+depth)
	return &clone, nil
}

func (query *Query[T]) DescendantsAndSelfOf(ctx context.Context, nodeID any) (*Query[T], error) {
	node, err := query.tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return query.empty(), err
	}
	clone := *query
	clone.query = clone.query.Where(query.tree.config.LeftField, ">=", node.Lft).Where(query.tree.config.RightField, "<=", node.Rgt)
	return &clone, nil
}

func (query *Query[T]) SiblingsOf(ctx context.Context, nodeID any) (*Query[T], error) {
	node, err := query.tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return query.empty(), err
	}
	clone := *query
	clone.query = clone.query.Where(primaryField(query.tree.db), "!=", node.ID)
	if node.ParentID.Valid {
		clone.query = clone.query.Where(query.tree.config.ParentField, node.ParentID.Value)
	} else {
		clone.query = clone.query.Where(query.tree.config.ParentField, nil)
	}
	return &clone, nil
}

func (query *Query[T]) WhereDepth(depth int) *Query[T] {
	clone := *query
	clone.query = clone.query.Where(query.tree.config.DepthField, depth)
	return &clone
}

func (query *Query[T]) WhereDepthGte(depth int) *Query[T] {
	clone := *query
	clone.query = clone.query.Where(query.tree.config.DepthField, ">=", depth)
	return &clone
}

func (query *Query[T]) WhereDepthLte(depth int) *Query[T] {
	clone := *query
	clone.query = clone.query.Where(query.tree.config.DepthField, "<=", depth)
	return &clone
}

func (query *Query[T]) DefaultOrder() *Query[T] {
	clone := *query
	clone.query = clone.query.OrderBy(query.tree.config.LeftField)
	return &clone
}

func (query *Query[T]) Reversed() *Query[T] {
	clone := *query
	clone.query = clone.query.OrderByDesc(query.tree.config.LeftField)
	return &clone
}

func (query *Query[T]) Get(ctx context.Context) ([]*T, error) {
	return query.query.Get(ctx)
}

func (query *Query[T]) First(ctx context.Context) (*T, error) {
	return query.query.First(ctx)
}

func (query *Query[T]) Count(ctx context.Context) (int64, error) {
	return query.query.Count(ctx)
}

func (query *Query[T]) Exists(ctx context.Context) (bool, error) {
	return query.query.Exists(ctx)
}

func (query *Query[T]) ToTree(ctx context.Context) ([]*Node[T], error) {
	models, err := query.DefaultOrder().Get(ctx)
	if err != nil {
		return nil, err
	}
	return buildNodes(query.tree.config, models)
}

func (query *Query[T]) empty() *Query[T] {
	clone := *query
	clone.query = clone.query.Where(primaryField(query.tree.db), -1)
	return &clone
}
