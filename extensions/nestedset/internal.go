package nestedset

import (
	"context"
	"reflect"

	"github.com/duxweb/oro"
)

const (
	childLast = iota + 1
	childFirst
	siblingBefore
	siblingAfter
)

const (
	moveRoot = iota + 1
	moveLastChild
	moveFirstChild
	moveBefore
	moveAfter
)

type treeRow struct {
	ID       uint64
	ParentID oro.Null[uint64]
	Lft      int
	Rgt      int
	Depth    int
}

type moveUpdate struct {
	ID           uint64
	ParentID     oro.Null[uint64]
	Lft          int
	Rgt          int
	Depth        int
	UpdateParent bool
}

func resolveConfig(options []Option) Config {
	config := Config{
		ParentField: "ParentID",
		LeftField:   "Lft",
		RightField:  "Rgt",
		DepthField:  "Depth",
	}
	for _, option := range options {
		if option != nil {
			option.applyNestedSetOption(&config)
		}
	}
	if config.ParentField == "" {
		config.ParentField = "ParentID"
	}
	if config.LeftField == "" {
		config.LeftField = "Lft"
	}
	if config.RightField == "" {
		config.RightField = "Rgt"
	}
	if config.DepthField == "" {
		config.DepthField = "Depth"
	}
	return config
}

func defaultLeftColumn(field string) string {
	if field == "Lft" {
		return "_lft"
	}
	return oro.Snake(field)
}

func defaultRightColumn(field string) string {
	if field == "Rgt" {
		return "_rgt"
	}
	return oro.Snake(field)
}

func (tree *Tree[T]) query() *oro.ModelQuery[T] {
	query := tree.db.Use[T]()
	for field, value := range tree.config.Scope {
		query = query.Where(field, value)
	}
	return query
}

func (tree *Tree[T]) write(ctx context.Context, fn func(txTree *Tree[T]) error) error {
	if tree.db == nil {
		return &oro.Error{Op: "nestedset", Kind: oro.ErrInvalidArgument}
	}
	return tree.db.Transaction(ctx, func(tx *oro.DB) error {
		txTree := *tree
		txTree.db = tx
		return fn(&txTree)
	})
}

func (tree *Tree[T]) createAtNode(ctx context.Context, targetID any, model *T, mode int, options ...oro.WriteOption) (*T, error) {
	var created *T
	err := tree.write(ctx, func(txTree *Tree[T]) error {
		target, err := txTree.nodeForUpdate(ctx, targetID)
		if err != nil || target == nil {
			return err
		}
		insertAt := target.Rgt
		parentID := oro.NullOf(target.ID)
		depth := target.Depth + 1
		switch mode {
		case childFirst:
			insertAt = target.Lft + 1
		case siblingBefore:
			insertAt = target.Lft
			parentID = target.ParentID
			depth = target.Depth
		case siblingAfter:
			insertAt = target.Rgt + 1
			parentID = target.ParentID
			depth = target.Depth
		}
		if err := txTree.makeGap(ctx, insertAt, 2); err != nil {
			return err
		}
		if err := txTree.prepareModel(model, parentIDValue(parentID), insertAt, insertAt+1, depth); err != nil {
			return err
		}
		created, err = txTree.query().Create(ctx, model, options...)
		return err
	})
	return created, err
}

func (tree *Tree[T]) makeGap(ctx context.Context, from int, width int) error {
	if _, err := tree.query().Where(tree.config.RightField, ">=", from).Update(ctx, oro.Map{tree.config.RightField: oro.Increment(width)}); err != nil {
		return err
	}
	if _, err := tree.query().Where(tree.config.LeftField, ">=", from).Update(ctx, oro.Map{tree.config.LeftField: oro.Increment(width)}); err != nil {
		return err
	}
	return nil
}

func (tree *Tree[T]) maxRight(ctx context.Context) (int, error) {
	value, err := tree.query().Max[int](ctx, tree.config.RightField)
	if err != nil || !value.Valid {
		return 0, err
	}
	return value.Value, nil
}

func (tree *Tree[T]) node(ctx context.Context, id any) (*treeRow, error) {
	model, err := tree.query().Where(primaryField(tree.db), id).First(ctx)
	if err != nil || model == nil {
		return nil, err
	}
	return tree.rowFromModel(model)
}

func (tree *Tree[T]) nodeForUpdate(ctx context.Context, id any) (*treeRow, error) {
	model, err := tree.query().Where(primaryField(tree.db), id).LockForUpdate().First(ctx)
	if err != nil || model == nil {
		return nil, err
	}
	return tree.rowFromModel(model)
}

func (tree *Tree[T]) rowFromModel(model *T) (*treeRow, error) {
	id, err := modelID(model)
	if err != nil {
		return nil, err
	}
	parentID, err := fieldNullUint64(model, tree.config.ParentField)
	if err != nil {
		return nil, err
	}
	left, err := fieldInt(model, tree.config.LeftField)
	if err != nil {
		return nil, err
	}
	right, err := fieldInt(model, tree.config.RightField)
	if err != nil {
		return nil, err
	}
	depth, err := fieldInt(model, tree.config.DepthField)
	if err != nil {
		return nil, err
	}
	return &treeRow{ID: id, ParentID: parentID, Lft: left, Rgt: right, Depth: depth}, nil
}

func (tree *Tree[T]) prepareModel(model *T, parent any, left int, right int, depth int) error {
	if model == nil {
		return &oro.Error{Op: "nestedset.create", Kind: oro.ErrInvalidArgument}
	}
	if err := setFieldValue(model, tree.config.ParentField, parent); err != nil {
		return err
	}
	if err := setFieldValue(model, tree.config.LeftField, left); err != nil {
		return err
	}
	if err := setFieldValue(model, tree.config.RightField, right); err != nil {
		return err
	}
	if err := setFieldValue(model, tree.config.DepthField, depth); err != nil {
		return err
	}
	for field, value := range tree.config.Scope {
		if err := setFieldValue(model, field, value); err != nil {
			return err
		}
	}
	return nil
}

func (tree *Tree[T]) updateValues(model *T) (oro.Map, error) {
	if model == nil {
		return nil, &oro.Error{Op: "nestedset.update", Kind: oro.ErrInvalidArgument}
	}
	values := oro.Map{}
	value := modelValue(model)
	for index := 0; index < value.NumField(); index++ {
		structField := value.Type().Field(index)
		if !structField.IsExported() || structField.Anonymous {
			continue
		}
		fieldValue := value.Field(index)
		if isZero(fieldValue) {
			continue
		}
		values[structField.Name] = fieldValue.Interface()
	}
	return values, nil
}

func (tree *Tree[T]) cleanUpdateValues(values oro.Map) oro.Map {
	copied := copyMap(values)
	delete(copied, tree.config.ParentField)
	delete(copied, tree.config.LeftField)
	delete(copied, tree.config.RightField)
	delete(copied, tree.config.DepthField)
	for field := range tree.config.Scope {
		delete(copied, field)
	}
	return copied
}

func (tree *Tree[T]) hasTreeCoordinates(model *T) bool {
	if model == nil {
		return false
	}
	left, err := namedField(model, tree.config.LeftField)
	if err != nil || isZero(left) {
		return false
	}
	right, err := namedField(model, tree.config.RightField)
	if err != nil || isZero(right) {
		return false
	}
	return true
}

func sameParent(left oro.Null[uint64], right oro.Null[uint64]) bool {
	if !left.Valid && !right.Valid {
		return true
	}
	if left.Valid != right.Valid {
		return false
	}
	return left.Value == right.Value
}

func (tree *Tree[T]) move(ctx context.Context, nodeID any, targetID any, mode int) error {
	return tree.write(ctx, func(txTree *Tree[T]) error {
		node, err := txTree.nodeForUpdate(ctx, nodeID)
		if err != nil || node == nil {
			return err
		}
		var target *treeRow
		if mode != moveRoot {
			target, err = txTree.nodeForUpdate(ctx, targetID)
			if err != nil || target == nil {
				return err
			}
			if target.Lft >= node.Lft && target.Rgt <= node.Rgt {
				return &oro.Error{Op: "nestedset.move", Kind: oro.ErrInvalidArgument}
			}
		}

		width := node.Rgt - node.Lft + 1
		newParent := oro.NullZero[uint64]()
		newDepth := 0
		insertAt := 0

		switch mode {
		case moveRoot:
			maxRight, err := txTree.maxRight(ctx)
			if err != nil {
				return err
			}
			insertAt = maxRight + 1
			newParent = oro.NullZero[uint64]()
			newDepth = 0
		case moveLastChild:
			insertAt = target.Rgt
			newParent = oro.NullOf(target.ID)
			newDepth = target.Depth + 1
		case moveFirstChild:
			insertAt = target.Lft + 1
			newParent = oro.NullOf(target.ID)
			newDepth = target.Depth + 1
		case moveBefore:
			insertAt = target.Lft
			newParent = target.ParentID
			newDepth = target.Depth
		case moveAfter:
			insertAt = target.Rgt + 1
			newParent = target.ParentID
			newDepth = target.Depth
		}

		if insertAt > node.Rgt {
			insertAt -= width
		}
		if insertAt == node.Lft {
			return nil
		}
		return txTree.moveSubtree(ctx, node, insertAt, newParent, newDepth)
	})
}

func (tree *Tree[T]) moveSubtree(ctx context.Context, node *treeRow, insertAt int, newParent oro.Null[uint64], newDepth int) error {
	models, err := tree.query().Where(tree.config.LeftField, ">=", node.Lft).Where(tree.config.RightField, "<=", node.Rgt).OrderBy(tree.config.LeftField).LockForUpdate().Get(ctx)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return nil
	}
	updates := make([]moveUpdate, 0, len(models))
	depthDelta := newDepth - node.Depth
	for _, model := range models {
		row, err := tree.rowFromModel(model)
		if err != nil {
			return err
		}
		update := moveUpdate{
			ID:    row.ID,
			Lft:   insertAt + (row.Lft - node.Lft),
			Rgt:   insertAt + (row.Rgt - node.Lft),
			Depth: row.Depth + depthDelta,
		}
		if row.ID == node.ID {
			update.ParentID = newParent
			update.UpdateParent = true
		}
		updates = append(updates, update)
	}
	width := node.Rgt - node.Lft + 1
	if _, err := tree.query().Where(tree.config.LeftField, ">", node.Rgt).Update(ctx, oro.Map{tree.config.LeftField: oro.Decrement(width)}); err != nil {
		return err
	}
	if _, err := tree.query().Where(tree.config.RightField, ">", node.Rgt).Update(ctx, oro.Map{tree.config.RightField: oro.Decrement(width)}); err != nil {
		return err
	}
	if _, err := tree.query().Where(tree.config.LeftField, ">=", insertAt).Update(ctx, oro.Map{tree.config.LeftField: oro.Increment(width)}); err != nil {
		return err
	}
	if _, err := tree.query().Where(tree.config.RightField, ">=", insertAt).Update(ctx, oro.Map{tree.config.RightField: oro.Increment(width)}); err != nil {
		return err
	}
	for _, update := range updates {
		values := oro.Map{
			tree.config.LeftField:  update.Lft,
			tree.config.RightField: update.Rgt,
			tree.config.DepthField: update.Depth,
		}
		if update.UpdateParent {
			values[tree.config.ParentField] = parentIDValue(update.ParentID)
		}
		if _, err := tree.query().Where(primaryField(tree.db), update.ID).Update(ctx, values); err != nil {
			return err
		}
	}
	return nil
}

func (tree *Tree[T]) rebuildModels(ctx context.Context, models []*T) error {
	return tree.write(ctx, func(txTree *Tree[T]) error {
		return txTree.rebuildModelsOnTx(ctx, models)
	})
}

func (tree *Tree[T]) rebuildModelsOnTx(ctx context.Context, models []*T) error {
	children := map[uint64][]*T{}
	roots := []*T{}
	for _, model := range models {
		if model == nil {
			continue
		}
		row, err := tree.rowFromModel(model)
		if err != nil {
			return err
		}
		if row.ParentID.Valid {
			children[row.ParentID.Value] = append(children[row.ParentID.Value], model)
		} else {
			roots = append(roots, model)
		}
	}
	counter := 1
	var assign func(model *T, depth int) error
	assign = func(model *T, depthValue int) error {
		id, err := modelID(model)
		if err != nil {
			return err
		}
		leftValue := counter
		counter++
		for _, child := range children[id] {
			if err := assign(child, depthValue+1); err != nil {
				return err
			}
		}
		rightValue := counter
		counter++
		_, err = tree.query().Where(primaryField(tree.db), id).Update(ctx, oro.Map{
			tree.config.LeftField:  leftValue,
			tree.config.RightField: rightValue,
			tree.config.DepthField: depthValue,
		})
		return err
	}
	for _, root := range roots {
		if err := assign(root, 0); err != nil {
			return err
		}
	}
	return nil
}

func (tree *Tree[T]) persistTreeNodes(ctx context.Context, nodes []*Node[T], parent oro.Null[uint64], ordered *[]*T) error {
	for _, node := range nodes {
		if node == nil || node.Model == nil {
			continue
		}
		if err := setFieldValue(node.Model, tree.config.ParentField, parentIDValue(parent)); err != nil {
			return err
		}
		id, err := modelID(node.Model)
		if err != nil {
			return err
		}
		if id == 0 {
			created, err := tree.query().Create(ctx, node.Model)
			if err != nil {
				return err
			}
			node.Model = created
			id, err = modelID(node.Model)
			if err != nil {
				return err
			}
		} else {
			if _, err := tree.query().Where(primaryField(tree.db), id).Update(ctx, oro.Map{tree.config.ParentField: parentIDValue(parent)}); err != nil {
				return err
			}
		}
		*ordered = append(*ordered, node.Model)
		if err := tree.persistTreeNodes(ctx, node.Children, oro.NullOf(id), ordered); err != nil {
			return err
		}
	}
	return nil
}

func buildNodes[T any](config Config, models []*T) ([]*Node[T], error) {
	roots := []*Node[T]{}
	stack := []*Node[T]{}
	baseDepth := 0
	baseSet := false
	for _, model := range models {
		depth, err := fieldInt(model, config.DepthField)
		if err != nil {
			return nil, err
		}
		if !baseSet {
			baseDepth = depth
			baseSet = true
		}
		depth -= baseDepth
		if depth < 0 {
			depth = 0
		}
		node := &Node[T]{Model: model, Depth: depth}
		for len(stack) > depth {
			stack = stack[:len(stack)-1]
		}
		if depth <= 0 || len(stack) == 0 {
			roots = append(roots, node)
		} else {
			parent := stack[len(stack)-1]
			parent.Children = append(parent.Children, node)
		}
		stack = append(stack, node)
	}
	return roots, nil
}

func (tree *Tree[T]) descendants(ctx context.Context, nodeID any, includeSelf bool) (*treeRow, []*T, error) {
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return nil, []*T{}, err
	}
	leftOp := ">"
	rightOp := "<"
	if includeSelf {
		leftOp = ">="
		rightOp = "<="
	}
	models, err := tree.query().
		Where(tree.config.LeftField, leftOp, node.Lft).
		Where(tree.config.RightField, rightOp, node.Rgt).
		OrderBy(tree.config.LeftField).
		Get(ctx)
	return node, models, err
}

func (tree *Tree[T]) descendantsWithinDepth(ctx context.Context, nodeID any, maxDepth int, includeSelf bool) (*treeRow, []*T, error) {
	if maxDepth < 0 {
		return nil, []*T{}, &oro.Error{Op: "nestedset.descendants", Kind: oro.ErrInvalidArgument, Field: "depth"}
	}
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return nil, []*T{}, err
	}
	minDepth := node.Depth + 1
	leftOp := ">"
	rightOp := "<"
	if includeSelf {
		minDepth = node.Depth
		leftOp = ">="
		rightOp = "<="
	}
	models, err := tree.query().
		Where(tree.config.LeftField, leftOp, node.Lft).
		Where(tree.config.RightField, rightOp, node.Rgt).
		Where(tree.config.DepthField, ">=", minDepth).
		Where(tree.config.DepthField, "<=", node.Depth+maxDepth).
		OrderBy(tree.config.LeftField).
		Get(ctx)
	return node, models, err
}

func (tree *Tree[T]) descendantsAtDepth(ctx context.Context, nodeID any, depth int, includeSelf bool) (*treeRow, []*T, error) {
	if depth < 0 {
		return nil, []*T{}, &oro.Error{Op: "nestedset.descendants", Kind: oro.ErrInvalidArgument, Field: "depth"}
	}
	node, err := tree.node(ctx, nodeID)
	if err != nil || node == nil {
		return nil, []*T{}, err
	}
	if depth == 0 && !includeSelf {
		return node, []*T{}, nil
	}
	leftOp := ">"
	rightOp := "<"
	if includeSelf {
		leftOp = ">="
		rightOp = "<="
	}
	models, err := tree.query().
		Where(tree.config.LeftField, leftOp, node.Lft).
		Where(tree.config.RightField, rightOp, node.Rgt).
		Where(tree.config.DepthField, node.Depth+depth).
		OrderBy(tree.config.LeftField).
		Get(ctx)
	return node, models, err
}

func (tree *Tree[T]) relativeNodes(models []*T, baseDepth int) ([]*RelativeNode[T], error) {
	nodes := make([]*RelativeNode[T], 0, len(models))
	for _, model := range models {
		depth, err := fieldInt(model, tree.config.DepthField)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, &RelativeNode[T]{
			Model: model,
			Depth: depth - baseDepth,
		})
	}
	return nodes, nil
}

func primaryField(db *oro.DB) string {
	return "ID"
}

func modelID(model any) (uint64, error) {
	value := modelValue(model)
	field := value.FieldByName("ID")
	if !field.IsValid() && value.FieldByName("Model").IsValid() {
		field = value.FieldByName("Model").FieldByName("ID")
	}
	if !field.IsValid() {
		return 0, &oro.Error{Op: "nestedset", Kind: oro.ErrUnknownField, Field: "ID"}
	}
	return uint64FromValue(field)
}

func fieldInt(model any, name string) (int, error) {
	field, err := namedField(model, name)
	if err != nil {
		return 0, err
	}
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return 0, nil
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int(field.Int()), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return int(field.Uint()), nil
	default:
		return 0, &oro.Error{Op: "nestedset", Kind: oro.ErrInvalidArgument, Field: name}
	}
}

func fieldNullUint64(model any, name string) (oro.Null[uint64], error) {
	field, err := namedField(model, name)
	if err != nil {
		return oro.NullZero[uint64](), err
	}
	if field.Type() == reflect.TypeOf(oro.Null[uint64]{}) {
		value := field.Interface().(oro.Null[uint64])
		return value, nil
	}
	if isZero(field) {
		return oro.NullZero[uint64](), nil
	}
	value, err := uint64FromValue(field)
	if err != nil {
		return oro.NullZero[uint64](), err
	}
	return oro.NullOf(value), nil
}

func setFieldValue(model any, name string, value any) error {
	field, err := namedField(model, name)
	if err != nil {
		return err
	}
	if !field.CanSet() {
		return &oro.Error{Op: "nestedset", Kind: oro.ErrInvalidArgument, Field: name}
	}
	if field.Type() == reflect.TypeOf(oro.Null[uint64]{}) {
		if value == nil {
			field.Set(reflect.ValueOf(oro.NullZero[uint64]()))
			return nil
		}
		uintValue, err := uint64FromAny(value)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(oro.NullOf(uintValue)))
		return nil
	}
	if value == nil {
		field.Set(reflect.Zero(field.Type()))
		return nil
	}
	valueReflect := reflect.ValueOf(value)
	if valueReflect.Type().AssignableTo(field.Type()) {
		field.Set(valueReflect)
		return nil
	}
	if valueReflect.Type().ConvertibleTo(field.Type()) {
		field.Set(valueReflect.Convert(field.Type()))
		return nil
	}
	return &oro.Error{Op: "nestedset", Kind: oro.ErrInvalidArgument, Field: name}
}

func namedField(model any, name string) (reflect.Value, error) {
	value := modelValue(model)
	field := value.FieldByName(name)
	if field.IsValid() {
		return field, nil
	}
	for index := 0; index < value.NumField(); index++ {
		structField := value.Type().Field(index)
		if !structField.Anonymous {
			continue
		}
		embedded := value.Field(index)
		for embedded.Kind() == reflect.Pointer {
			if embedded.IsNil() {
				break
			}
			embedded = embedded.Elem()
		}
		if embedded.IsValid() && embedded.Kind() == reflect.Struct {
			field = embedded.FieldByName(name)
			if field.IsValid() {
				return field, nil
			}
		}
	}
	return reflect.Value{}, &oro.Error{Op: "nestedset", Kind: oro.ErrUnknownField, Field: name}
}

func modelValue(model any) reflect.Value {
	value := reflect.ValueOf(model)
	for value.Kind() == reflect.Pointer {
		value = value.Elem()
	}
	return value
}

func uint64FromValue(value reflect.Value) (uint64, error) {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0, nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return value.Uint(), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intValue := value.Int()
		if intValue < 0 {
			return 0, &oro.Error{Op: "nestedset", Kind: oro.ErrInvalidArgument}
		}
		return uint64(intValue), nil
	default:
		return 0, &oro.Error{Op: "nestedset", Kind: oro.ErrInvalidArgument}
	}
}

func uint64FromAny(value any) (uint64, error) {
	if value == nil {
		return 0, nil
	}
	return uint64FromValue(reflect.ValueOf(value))
}

func isZero(value reflect.Value) bool {
	return value.IsZero()
}

func parentIDValue(value oro.Null[uint64]) any {
	if !value.Valid {
		return nil
	}
	return value.Value
}

func copyMap(values oro.Map) oro.Map {
	if len(values) == 0 {
		return nil
	}
	copied := oro.Map{}
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func schemaColumn(schema *oro.ModelSchema, field string) string {
	if schema != nil {
		if item, ok := schema.FieldByGo[field]; ok {
			return item.Column
		}
	}
	return oro.Snake(field)
}
