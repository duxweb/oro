package nestedset

import (
	"context"
	"fmt"

	"github.com/duxweb/oro"
)

const extensionName = "nestedset"

type NodeFields struct {
	ParentID oro.Null[uint64]
	Lft      int
	Rgt      int
	Depth    int
}

func (NodeFields) OroEmbeddedFields() {}

func (NodeFields) DefineOroFields(s *oro.SchemaBuilder) {
	Define(s)
}

type Config struct {
	ParentField string
	LeftField   string
	RightField  string
	DepthField  string
	Scope       oro.Map
}

type Option interface {
	applyNestedSetOption(*Config)
}

type optionFunc func(*Config)

func (fn optionFunc) applyNestedSetOption(config *Config) {
	fn(config)
}

func Scope(values oro.Map) Option {
	return optionFunc(func(config *Config) {
		config.Scope = copyMap(values)
	})
}

func FieldNames(parent string, left string, right string, depth string) Option {
	return optionFunc(func(config *Config) {
		config.ParentField = parent
		config.LeftField = left
		config.RightField = right
		config.DepthField = depth
	})
}

type extension struct{}

func Extension(options ...Option) oro.Extension {
	return extension{}
}

func (extension) Name() string {
	return extensionName
}

func (extension) Install(db *oro.DB) error {
	return nil
}

func Define(s *oro.SchemaBuilder, options ...Option) {
	config := resolveConfig(options)
	s.Field(config.ParentField).Uint().Nullable().Index()
	s.Field(config.LeftField).Int().Column(defaultLeftColumn(config.LeftField)).Default(0).Index()
	s.Field(config.RightField).Int().Column(defaultRightColumn(config.RightField)).Default(0).Index()
	s.Field(config.DepthField).Int().Default(0).Index()
	s.Index("", config.LeftField, config.RightField)
}

type Tree[T any] struct {
	db     *oro.DB
	config Config
}

type Node[T any] struct {
	Model    *T
	Depth    int
	Children []*Node[T]
}

type RelativeNode[T any] struct {
	Model *T
	Depth int
}

func Use[T any](db *oro.DB, options ...Option) *Tree[T] {
	return &Tree[T]{db: db, config: resolveConfig(options)}
}

func (tree *Tree[T]) Create(ctx context.Context, model *T, options ...oro.WriteOption) (*T, error) {
	if model == nil {
		return nil, &oro.Error{Op: "nestedset.create", Kind: oro.ErrInvalidArgument}
	}
	parentID, err := fieldNullUint64(model, tree.config.ParentField)
	if err != nil {
		return nil, err
	}
	if !parentID.Valid {
		return tree.createRoot(ctx, model, options...)
	}
	return tree.createAtNode(ctx, parentID.Value, model, childLast, options...)
}

func (tree *Tree[T]) CreateRoot(ctx context.Context, model *T) (*T, error) {
	return tree.createRoot(ctx, model)
}

func (tree *Tree[T]) createRoot(ctx context.Context, model *T, options ...oro.WriteOption) (*T, error) {
	var created *T
	err := tree.write(ctx, func(txTree *Tree[T]) error {
		maxRight, err := txTree.maxRight(ctx)
		if err != nil {
			return err
		}
		if err := txTree.prepareModel(model, nil, maxRight+1, maxRight+2, 0); err != nil {
			return err
		}
		created, err = txTree.query().Create(ctx, model, options...)
		return err
	})
	return created, err
}

func (tree *Tree[T]) CreateChild(ctx context.Context, parentID any, model *T) (*T, error) {
	return tree.createAtNode(ctx, parentID, model, childLast)
}

func (tree *Tree[T]) CreateFirstChild(ctx context.Context, parentID any, model *T) (*T, error) {
	return tree.createAtNode(ctx, parentID, model, childFirst)
}

func (tree *Tree[T]) CreateBefore(ctx context.Context, targetID any, model *T) (*T, error) {
	return tree.createAtNode(ctx, targetID, model, siblingBefore)
}

func (tree *Tree[T]) CreateAfter(ctx context.Context, targetID any, model *T) (*T, error) {
	return tree.createAtNode(ctx, targetID, model, siblingAfter)
}

func (tree *Tree[T]) Update(ctx context.Context, model *T, options ...oro.WriteOption) (*T, error) {
	if model == nil {
		return nil, &oro.Error{Op: "nestedset.update", Kind: oro.ErrInvalidArgument}
	}
	id, err := modelID(model)
	if err != nil {
		return nil, err
	}
	if id == 0 {
		return nil, &oro.Error{Op: "nestedset.update", Kind: oro.ErrInvalidArgument, Field: primaryField(tree.db)}
	}
	err = tree.write(ctx, func(txTree *Tree[T]) error {
		current, err := txTree.nodeForUpdate(ctx, id)
		if err != nil || current == nil {
			return err
		}
		nextParent, err := fieldNullUint64(model, txTree.config.ParentField)
		if err != nil {
			return err
		}
		if !nextParent.Valid && !txTree.hasTreeCoordinates(model) {
			nextParent = current.ParentID
		}
		values, err := txTree.updateValues(model)
		if err != nil {
			return err
		}
		values = txTree.cleanUpdateValues(values)
		if len(values) > 0 {
			if _, err := txTree.query().Where(primaryField(txTree.db), id).Update(ctx, values, options...); err != nil {
				return err
			}
		}
		if sameParent(current.ParentID, nextParent) {
			return nil
		}
		if nextParent.Valid {
			return txTree.move(ctx, id, nextParent.Value, moveLastChild)
		}
		return txTree.move(ctx, id, nil, moveRoot)
	})
	if err != nil {
		return nil, err
	}
	return tree.query().Find(ctx, id)
}

func (tree *Tree[T]) UpdateValues(ctx context.Context, nodeID any, values oro.Map, options ...oro.WriteOption) (*T, error) {
	if values == nil {
		values = oro.Map{}
	}
	err := tree.write(ctx, func(txTree *Tree[T]) error {
		current, err := txTree.nodeForUpdate(ctx, nodeID)
		if err != nil || current == nil {
			return err
		}
		nextParent := current.ParentID
		if value, ok := values[txTree.config.ParentField]; ok {
			parent, err := uint64FromAny(value)
			if err != nil {
				return err
			}
			if value == nil {
				nextParent = oro.NullZero[uint64]()
			} else {
				nextParent = oro.NullOf(parent)
			}
		}
		cleanedValues := txTree.cleanUpdateValues(values)
		if len(cleanedValues) > 0 {
			if _, err := txTree.query().Where(primaryField(txTree.db), nodeID).Update(ctx, cleanedValues, options...); err != nil {
				return err
			}
		}
		if sameParent(current.ParentID, nextParent) {
			return nil
		}
		if nextParent.Valid {
			return txTree.move(ctx, nodeID, nextParent.Value, moveLastChild)
		}
		return txTree.move(ctx, nodeID, nil, moveRoot)
	})
	if err != nil {
		return nil, err
	}
	return tree.query().Find(ctx, nodeID)
}

func (tree *Tree[T]) Roots(ctx context.Context) ([]*T, error) {
	return tree.query().Where(tree.config.ParentField, nil).OrderBy(tree.config.LeftField).Get(ctx)
}

func (tree *Tree[T]) All(ctx context.Context) ([]*T, error) {
	return tree.query().OrderBy(tree.config.LeftField).Get(ctx)
}

func (tree *Tree[T]) FlatTree(ctx context.Context) ([]*T, error) {
	return tree.All(ctx)
}

func (tree *Tree[T]) Query() *Query[T] {
	return &Query[T]{
		tree:  tree,
		query: tree.query(),
	}
}

func (tree *Tree[T]) Tree(ctx context.Context) ([]*Node[T], error) {
	models, err := tree.All(ctx)
	if err != nil {
		return nil, err
	}
	return buildNodes(tree.config, models)
}

func (tree *Tree[T]) Subtree(ctx context.Context, nodeID any) ([]*Node[T], error) {
	models, err := tree.FlatSubtree(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	return buildNodes(tree.config, models)
}

func (tree *Tree[T]) FlatSubtree(ctx context.Context, nodeID any) ([]*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return []*T{}, err
	}
	return tree.query().Where(tree.config.LeftField, ">=", node.Lft).Where(tree.config.RightField, "<=", node.Rgt).OrderBy(tree.config.LeftField).Get(ctx)
}

func (tree *Tree[T]) Parent(ctx context.Context, nodeID any) (*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil || !node.ParentID.Valid {
		return nil, err
	}
	return tree.query().Where(primaryField(tree.db), node.ParentID.Value).First(ctx)
}

func (tree *Tree[T]) Children(ctx context.Context, nodeID any) ([]*T, error) {
	return tree.query().Where(tree.config.ParentField, nodeID).OrderBy(tree.config.LeftField).Get(ctx)
}

func (tree *Tree[T]) FirstChild(ctx context.Context, nodeID any) (*T, error) {
	return tree.query().Where(tree.config.ParentField, nodeID).OrderBy(tree.config.LeftField).First(ctx)
}

func (tree *Tree[T]) LastChild(ctx context.Context, nodeID any) (*T, error) {
	return tree.query().Where(tree.config.ParentField, nodeID).OrderByDesc(tree.config.LeftField).First(ctx)
}

func (tree *Tree[T]) Siblings(ctx context.Context, nodeID any) ([]*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return []*T{}, err
	}
	query := tree.query().Where(primaryField(tree.db), "!=", node.ID).OrderBy(tree.config.LeftField)
	if node.ParentID.Valid {
		query = query.Where(tree.config.ParentField, node.ParentID.Value)
	} else {
		query = query.Where(tree.config.ParentField, nil)
	}
	return query.Get(ctx)
}

func (tree *Tree[T]) PrevSibling(ctx context.Context, nodeID any) (*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return nil, err
	}
	query := tree.query().Where(primaryField(tree.db), "!=", node.ID).Where(tree.config.RightField, "<", node.Lft).OrderByDesc(tree.config.RightField)
	if node.ParentID.Valid {
		query = query.Where(tree.config.ParentField, node.ParentID.Value)
	} else {
		query = query.Where(tree.config.ParentField, nil)
	}
	return query.First(ctx)
}

func (tree *Tree[T]) NextSibling(ctx context.Context, nodeID any) (*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return nil, err
	}
	query := tree.query().Where(primaryField(tree.db), "!=", node.ID).Where(tree.config.LeftField, ">", node.Rgt).OrderBy(tree.config.LeftField)
	if node.ParentID.Valid {
		query = query.Where(tree.config.ParentField, node.ParentID.Value)
	} else {
		query = query.Where(tree.config.ParentField, nil)
	}
	return query.First(ctx)
}

func (tree *Tree[T]) Ancestors(ctx context.Context, nodeID any) ([]*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return []*T{}, err
	}
	return tree.query().Where(tree.config.LeftField, "<", node.Lft).Where(tree.config.RightField, ">", node.Rgt).OrderBy(tree.config.LeftField).Get(ctx)
}

func (tree *Tree[T]) AncestorsAndSelf(ctx context.Context, nodeID any) ([]*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return []*T{}, err
	}
	return tree.query().Where(tree.config.LeftField, "<=", node.Lft).Where(tree.config.RightField, ">=", node.Rgt).OrderBy(tree.config.LeftField).Get(ctx)
}

func (tree *Tree[T]) Descendants(ctx context.Context, nodeID any) ([]*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return []*T{}, err
	}
	return tree.query().Where(tree.config.LeftField, ">", node.Lft).Where(tree.config.RightField, "<", node.Rgt).OrderBy(tree.config.LeftField).Get(ctx)
}

func (tree *Tree[T]) DescendantsAndSelf(ctx context.Context, nodeID any) ([]*T, error) {
	return tree.FlatSubtree(ctx, nodeID)
}

func (tree *Tree[T]) DescendantsWithDepth(ctx context.Context, nodeID any) ([]*RelativeNode[T], error) {
	node, models, err := tree.descendants(ctx, nodeID, false)
	if err != nil {
		return nil, err
	}
	return tree.relativeNodes(models, node.Depth)
}

func (tree *Tree[T]) DescendantsAndSelfWithDepth(ctx context.Context, nodeID any) ([]*RelativeNode[T], error) {
	node, models, err := tree.descendants(ctx, nodeID, true)
	if err != nil {
		return nil, err
	}
	return tree.relativeNodes(models, node.Depth)
}

func (tree *Tree[T]) DescendantsWithinDepth(ctx context.Context, nodeID any, maxDepth int) ([]*T, error) {
	_, models, err := tree.descendantsWithinDepth(ctx, nodeID, maxDepth, false)
	return models, err
}

func (tree *Tree[T]) DescendantsAndSelfWithinDepth(ctx context.Context, nodeID any, maxDepth int) ([]*T, error) {
	_, models, err := tree.descendantsWithinDepth(ctx, nodeID, maxDepth, true)
	return models, err
}

func (tree *Tree[T]) DescendantsAtDepth(ctx context.Context, nodeID any, depth int) ([]*T, error) {
	_, models, err := tree.descendantsAtDepth(ctx, nodeID, depth, false)
	return models, err
}

func (tree *Tree[T]) DescendantsAndSelfAtDepth(ctx context.Context, nodeID any, depth int) ([]*T, error) {
	_, models, err := tree.descendantsAtDepth(ctx, nodeID, depth, true)
	return models, err
}

func (tree *Tree[T]) IsRoot(ctx context.Context, nodeID any) (bool, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return false, err
	}
	return !node.ParentID.Valid, nil
}

func (tree *Tree[T]) IsLeaf(ctx context.Context, nodeID any) (bool, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return false, err
	}
	return node.Rgt-node.Lft == 1, nil
}

func (tree *Tree[T]) IsAncestorOf(ctx context.Context, ancestorID any, nodeID any) (bool, error) {
	ancestor, err := tree.node(ctx, ancestorID)
	if err != nil || ancestor == nil {
		return false, err
	}
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return false, err
	}
	return ancestor.Lft < node.Lft && ancestor.Rgt > node.Rgt, nil
}

func (tree *Tree[T]) IsDescendantOf(ctx context.Context, nodeID any, ancestorID any) (bool, error) {
	return tree.IsAncestorOf(ctx, ancestorID, nodeID)
}

func (tree *Tree[T]) MoveToRoot(ctx context.Context, nodeID any) error {
	return tree.move(ctx, nodeID, nil, moveRoot)
}

func (tree *Tree[T]) MoveToChildOf(ctx context.Context, nodeID any, parentID any) error {
	return tree.move(ctx, nodeID, parentID, moveLastChild)
}

func (tree *Tree[T]) MoveToFirstChildOf(ctx context.Context, nodeID any, parentID any) error {
	return tree.move(ctx, nodeID, parentID, moveFirstChild)
}

func (tree *Tree[T]) MoveBefore(ctx context.Context, nodeID any, targetID any) error {
	return tree.move(ctx, nodeID, targetID, moveBefore)
}

func (tree *Tree[T]) MoveAfter(ctx context.Context, nodeID any, targetID any) error {
	return tree.move(ctx, nodeID, targetID, moveAfter)
}

func (tree *Tree[T]) MoveUp(ctx context.Context, nodeID any) error {
	prev, err := tree.PrevSibling(ctx, nodeID)
	if err != nil || prev == nil {
		return err
	}
	prevID, err := modelID(prev)
	if err != nil {
		return err
	}
	return tree.MoveBefore(ctx, nodeID, prevID)
}

func (tree *Tree[T]) MoveDown(ctx context.Context, nodeID any) error {
	next, err := tree.NextSibling(ctx, nodeID)
	if err != nil || next == nil {
		return err
	}
	nextID, err := modelID(next)
	if err != nil {
		return err
	}
	return tree.MoveAfter(ctx, nodeID, nextID)
}

func (tree *Tree[T]) Delete(ctx context.Context, nodeID any) (int64, error) {
	return tree.DeleteSubtree(ctx, nodeID)
}

func (tree *Tree[T]) DeleteSubtree(ctx context.Context, nodeID any) (int64, error) {
	var deleted int64
	err := tree.write(ctx, func(txTree *Tree[T]) error {
		node, err := txTree.nodeForUpdate(ctx, nodeID)
		if err != nil || node == nil {
			return err
		}
		width := node.Rgt - node.Lft + 1
		deleted, err = txTree.query().Where(txTree.config.LeftField, ">=", node.Lft).Where(txTree.config.RightField, "<=", node.Rgt).ForceDelete(ctx)
		if err != nil {
			return err
		}
		if _, err := txTree.query().Where(txTree.config.LeftField, ">", node.Rgt).Update(ctx, oro.Map{txTree.config.LeftField: oro.Decrement(width)}); err != nil {
			return err
		}
		if _, err := txTree.query().Where(txTree.config.RightField, ">", node.Rgt).Update(ctx, oro.Map{txTree.config.RightField: oro.Decrement(width)}); err != nil {
			return err
		}
		return nil
	})
	return deleted, err
}

type CheckResult struct {
	Valid       bool
	Errors      []string
	NodeCount   int
	RootCount   int
	MaxRight    int
	Missing     []int
	Duplicated  []int
	InvalidRows int
}

func (tree *Tree[T]) Check(ctx context.Context) (*CheckResult, error) {
	models, err := tree.All(ctx)
	if err != nil {
		return nil, err
	}
	result := &CheckResult{Valid: true, NodeCount: len(models)}
	seen := map[int]int{}
	ids := map[uint64]*treeRow{}
	for _, model := range models {
		row, err := tree.rowFromModel(model)
		if err != nil {
			return nil, err
		}
		ids[row.ID] = row
		if !row.ParentID.Valid {
			result.RootCount++
		}
		if row.Lft <= 0 || row.Rgt <= 0 || row.Lft >= row.Rgt || (row.Rgt-row.Lft)%2 == 0 {
			result.InvalidRows++
		}
		seen[row.Lft]++
		seen[row.Rgt]++
		if row.Rgt > result.MaxRight {
			result.MaxRight = row.Rgt
		}
	}
	for value := 1; value <= result.NodeCount*2; value++ {
		count := seen[value]
		if count == 0 {
			result.Missing = append(result.Missing, value)
		}
		if count > 1 {
			result.Duplicated = append(result.Duplicated, value)
		}
	}
	for _, row := range ids {
		if !row.ParentID.Valid {
			continue
		}
		parent, ok := ids[row.ParentID.Value]
		if !ok {
			result.Errors = append(result.Errors, fmt.Sprintf("node %d parent %d missing", row.ID, row.ParentID.Value))
			continue
		}
		if !(parent.Lft < row.Lft && parent.Rgt > row.Rgt && parent.Depth+1 == row.Depth) {
			result.Errors = append(result.Errors, fmt.Sprintf("node %d parent bounds invalid", row.ID))
		}
	}
	if result.NodeCount > 0 && result.MaxRight != result.NodeCount*2 {
		result.Errors = append(result.Errors, "max right does not match node count")
	}
	if result.InvalidRows > 0 || len(result.Missing) > 0 || len(result.Duplicated) > 0 || len(result.Errors) > 0 {
		result.Valid = false
	}
	return result, nil
}

func (tree *Tree[T]) Rebuild(ctx context.Context) error {
	models, err := tree.query().OrderBy(tree.config.ParentField).OrderBy(primaryField(tree.db)).Get(ctx)
	if err != nil {
		return err
	}
	return tree.rebuildModels(ctx, models)
}

func (tree *Tree[T]) RebuildTree(ctx context.Context, roots []*Node[T]) error {
	return tree.write(ctx, func(txTree *Tree[T]) error {
		ordered := make([]*T, 0)
		if err := txTree.persistTreeNodes(ctx, roots, oro.NullZero[uint64](), &ordered); err != nil {
			return err
		}
		return txTree.rebuildModelsOnTx(ctx, ordered)
	})
}
