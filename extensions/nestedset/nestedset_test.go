package nestedset_test

import (
	"context"
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/driver/sqlite"
	"github.com/duxweb/oro/extensions/nestedset"
	_ "modernc.org/sqlite"
)

type category struct {
	oro.Model
	nestedset.NodeFields

	TenantID uint64
	Name     string
}

func (category) Define(s *oro.SchemaBuilder) {
	s.Table("nested_categories")
	s.Field("TenantID").Uint().Default(0).Index()
	nestedset.Define(s)
	s.Field("Name").String().Size(120)
}

func openNestedSetDB(t *testing.T) (*oro.DB, context.Context) {
	t.Helper()
	db, err := oro.Open(oro.Config{
		Connections: map[string]oro.ConnectionConfig{
			"default": {Driver: sqlite.Open(":memory:")},
		},
		Extensions: []oro.Extension{nestedset.Extension()},
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	if err := db.Register(category{}); err != nil {
		t.Fatalf("register: %v", err)
	}
	ctx := context.Background()
	if err := db.Sync(ctx); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return db, ctx
}

func TestNestedSetCreateAndRead(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root, err := tree.CreateRoot(ctx, &category{Name: "root"})
	if err != nil {
		t.Fatalf("create root: %v", err)
	}
	first, err := tree.CreateChild(ctx, root.ID, &category{Name: "first"})
	if err != nil {
		t.Fatalf("create child: %v", err)
	}
	second, err := tree.CreateAfter(ctx, first.ID, &category{Name: "second"})
	if err != nil {
		t.Fatalf("create sibling: %v", err)
	}
	grand, err := tree.CreateFirstChild(ctx, second.ID, &category{Name: "grand"})
	if err != nil {
		t.Fatalf("create grandchild: %v", err)
	}

	assertOrder(t, ctx, tree, []string{"root", "first", "second", "grand"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root":   {1, 8, 0},
		"first":  {2, 3, 1},
		"second": {4, 7, 1},
		"grand":  {5, 6, 2},
	})

	ancestors, err := tree.AncestorsAndSelf(ctx, grand.ID)
	if err != nil {
		t.Fatalf("ancestors: %v", err)
	}
	assertNames(t, ancestors, []string{"root", "second", "grand"})

	descendants, err := tree.Descendants(ctx, root.ID)
	if err != nil {
		t.Fatalf("descendants: %v", err)
	}
	assertNames(t, descendants, []string{"first", "second", "grand"})

	nodes, err := tree.Tree(ctx)
	if err != nil {
		t.Fatalf("tree: %v", err)
	}
	if len(nodes) != 1 || nodes[0].Model.Name != "root" || len(nodes[0].Children) != 2 || len(nodes[0].Children[1].Children) != 1 {
		t.Fatalf("unexpected nested tree: %#v", nodes)
	}

	valid, err := tree.Check(ctx)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !valid.Valid {
		t.Fatalf("tree should be valid: %#v", valid)
	}
}

func TestNestedSetCreateAndUpdateFromParentID(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root, err := tree.Create(ctx, &category{Name: "root"})
	if err != nil {
		t.Fatalf("create root from parent id: %v", err)
	}
	a, err := tree.Create(ctx, &category{Name: "a", NodeFields: nestedset.NodeFields{ParentID: oro.NullOf(root.ID)}})
	if err != nil {
		t.Fatalf("create child a from parent id: %v", err)
	}
	b, err := tree.Create(ctx, &category{Name: "b", NodeFields: nestedset.NodeFields{ParentID: oro.NullOf(root.ID)}})
	if err != nil {
		t.Fatalf("create child b from parent id: %v", err)
	}
	c, err := tree.Create(ctx, &category{Name: "c", NodeFields: nestedset.NodeFields{ParentID: oro.NullOf(a.ID)}})
	if err != nil {
		t.Fatalf("create child c from parent id: %v", err)
	}

	assertOrder(t, ctx, tree, []string{"root", "a", "c", "b"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 8, 0},
		"a":    {2, 5, 1},
		"c":    {3, 4, 2},
		"b":    {6, 7, 1},
	})

	b.Name = "b2"
	b.ParentID = oro.NullOf(c.ID)
	updated, err := tree.Update(ctx, b)
	if err != nil {
		t.Fatalf("update parent id: %v", err)
	}
	if updated == nil || updated.Name != "b2" || !updated.ParentID.Valid || updated.ParentID.Value != c.ID {
		t.Fatalf("unexpected updated node %#v", updated)
	}

	assertOrder(t, ctx, tree, []string{"root", "a", "c", "b2"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 8, 0},
		"a":    {2, 7, 1},
		"c":    {3, 6, 2},
		"b2":   {4, 5, 3},
	})

	c.ParentID = oro.NullZero[uint64]()
	if _, err := tree.Update(ctx, c); err != nil {
		t.Fatalf("move to root from parent id: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "a", "c", "b2"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 4, 0},
		"a":    {2, 3, 1},
		"c":    {5, 8, 0},
		"b2":   {6, 7, 1},
	})
}

func TestNestedSetMoveAndDelete(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root, _ := tree.CreateRoot(ctx, &category{Name: "root"})
	a, _ := tree.CreateChild(ctx, root.ID, &category{Name: "a"})
	b, _ := tree.CreateChild(ctx, root.ID, &category{Name: "b"})
	c, _ := tree.CreateChild(ctx, b.ID, &category{Name: "c"})

	if err := tree.MoveBefore(ctx, b.ID, a.ID); err != nil {
		t.Fatalf("move before: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "b", "c", "a"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 8, 0},
		"b":    {2, 5, 1},
		"c":    {3, 4, 2},
		"a":    {6, 7, 1},
	})

	if err := tree.MoveToChildOf(ctx, a.ID, c.ID); err != nil {
		t.Fatalf("move to child: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "b", "c", "a"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 8, 0},
		"b":    {2, 7, 1},
		"c":    {3, 6, 2},
		"a":    {4, 5, 3},
	})

	if err := tree.MoveToRoot(ctx, c.ID); err != nil {
		t.Fatalf("move to root: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "b", "c", "a"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 4, 0},
		"b":    {2, 3, 1},
		"c":    {5, 8, 0},
		"a":    {6, 7, 1},
	})

	deleted, err := tree.Delete(ctx, c.ID)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	assertOrder(t, ctx, tree, []string{"root", "b"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 4, 0},
		"b":    {2, 3, 1},
	})
}

func TestNestedSetScopeIsolation(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tenantOne := nestedset.Use[category](db, nestedset.Scope(oro.Map{"TenantID": uint64(1)}))
	tenantTwo := nestedset.Use[category](db, nestedset.Scope(oro.Map{"TenantID": uint64(2)}))

	rootOne, err := tenantOne.CreateRoot(ctx, &category{Name: "one-root"})
	if err != nil {
		t.Fatalf("create tenant one root: %v", err)
	}
	if _, err := tenantOne.CreateChild(ctx, rootOne.ID, &category{Name: "one-child"}); err != nil {
		t.Fatalf("create tenant one child: %v", err)
	}
	rootTwo, err := tenantTwo.CreateRoot(ctx, &category{Name: "two-root"})
	if err != nil {
		t.Fatalf("create tenant two root: %v", err)
	}
	if rootTwo.TenantID != 2 {
		t.Fatalf("scope should be written to model, got %d", rootTwo.TenantID)
	}

	assertBounds(t, ctx, tenantOne, map[string][3]int{
		"one-root":  {1, 4, 0},
		"one-child": {2, 3, 1},
	})
	assertBounds(t, ctx, tenantTwo, map[string][3]int{
		"two-root": {1, 2, 0},
	})
}

func TestNestedSetRebuild(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root, _ := db.Use[category]().Create(ctx, &category{Name: "root"})
	a, _ := db.Use[category]().Create(ctx, &category{Name: "a", NodeFields: nestedset.NodeFields{ParentID: oro.NullOf(root.ID)}})
	_, _ = db.Use[category]().Create(ctx, &category{Name: "b", NodeFields: nestedset.NodeFields{ParentID: oro.NullOf(root.ID)}})
	_, _ = db.Use[category]().Create(ctx, &category{Name: "c", NodeFields: nestedset.NodeFields{ParentID: oro.NullOf(a.ID)}})

	if err := tree.Rebuild(ctx); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "a", "c", "b"})
	result, err := tree.Check(ctx)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !result.Valid {
		t.Fatalf("rebuilt tree invalid: %#v", result)
	}
}

func assertOrder(t *testing.T, ctx context.Context, tree *nestedset.Tree[category], names []string) {
	t.Helper()
	models, err := tree.All(ctx)
	if err != nil {
		t.Fatalf("all: %v", err)
	}
	assertNames(t, models, names)
}

func assertNames(t *testing.T, models []*category, names []string) {
	t.Helper()
	if len(models) != len(names) {
		t.Fatalf("len = %d, want %d (%v)", len(models), len(names), names)
	}
	for index, model := range models {
		if model.Name != names[index] {
			t.Fatalf("models[%d] = %s, want %s", index, model.Name, names[index])
		}
	}
}

func assertBounds(t *testing.T, ctx context.Context, tree *nestedset.Tree[category], expected map[string][3]int) {
	t.Helper()
	models, err := tree.All(ctx)
	if err != nil {
		t.Fatalf("all bounds: %v", err)
	}
	if len(models) != len(expected) {
		t.Fatalf("model len = %d, want %d", len(models), len(expected))
	}
	for _, model := range models {
		bounds, ok := expected[model.Name]
		if !ok {
			t.Fatalf("unexpected model %s", model.Name)
		}
		if model.Lft != bounds[0] || model.Rgt != bounds[1] || model.Depth != bounds[2] {
			t.Fatalf("%s bounds = (%d,%d,%d), want (%d,%d,%d)", model.Name, model.Lft, model.Rgt, model.Depth, bounds[0], bounds[1], bounds[2])
		}
	}
}

func TestNestedSetRelationsPredicatesAndQuery(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root, _ := tree.CreateRoot(ctx, &category{Name: "root"})
	a, _ := tree.CreateChild(ctx, root.ID, &category{Name: "a"})
	b, _ := tree.CreateChild(ctx, root.ID, &category{Name: "b"})
	c, _ := tree.CreateChild(ctx, b.ID, &category{Name: "c"})

	parent, err := tree.Parent(ctx, c.ID)
	if err != nil || parent == nil || parent.Name != "b" {
		t.Fatalf("parent = %#v, err=%v", parent, err)
	}
	children, err := tree.Children(ctx, root.ID)
	if err != nil {
		t.Fatalf("children: %v", err)
	}
	assertNames(t, children, []string{"a", "b"})
	firstChild, _ := tree.FirstChild(ctx, root.ID)
	lastChild, _ := tree.LastChild(ctx, root.ID)
	if firstChild.Name != "a" || lastChild.Name != "b" {
		t.Fatalf("first/last children = %s/%s", firstChild.Name, lastChild.Name)
	}
	siblings, err := tree.Siblings(ctx, a.ID)
	if err != nil {
		t.Fatalf("siblings: %v", err)
	}
	assertNames(t, siblings, []string{"b"})
	prev, _ := tree.PrevSibling(ctx, b.ID)
	next, _ := tree.NextSibling(ctx, a.ID)
	if prev.Name != "a" || next.Name != "b" {
		t.Fatalf("prev/next = %s/%s", prev.Name, next.Name)
	}

	isRoot, _ := tree.IsRoot(ctx, root.ID)
	isLeaf, _ := tree.IsLeaf(ctx, a.ID)
	isAncestor, _ := tree.IsAncestorOf(ctx, root.ID, c.ID)
	isDescendant, _ := tree.IsDescendantOf(ctx, c.ID, root.ID)
	if !isRoot || !isLeaf || !isAncestor || !isDescendant {
		t.Fatalf("unexpected predicates: root=%v leaf=%v ancestor=%v descendant=%v", isRoot, isLeaf, isAncestor, isDescendant)
	}

	deep, err := tree.Query().WhereDepthGte(1).DefaultOrder().Get(ctx)
	if err != nil {
		t.Fatalf("query depth: %v", err)
	}
	assertNames(t, deep, []string{"a", "b", "c"})
	descQuery, err := tree.Query().DescendantsOf(ctx, root.ID)
	if err != nil {
		t.Fatalf("desc query: %v", err)
	}
	desc, err := descQuery.DefaultOrder().Get(ctx)
	if err != nil {
		t.Fatalf("desc get: %v", err)
	}
	assertNames(t, desc, []string{"a", "b", "c"})
	nested, err := descQuery.ToTree(ctx)
	if err != nil {
		t.Fatalf("to tree: %v", err)
	}
	if len(nested) != 2 || nested[1].Model.Name != "b" || len(nested[1].Children) != 1 {
		t.Fatalf("unexpected query tree: %#v", nested)
	}
}

func TestNestedSetMoveUpDownAndInvalidMove(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root, _ := tree.CreateRoot(ctx, &category{Name: "root"})
	a, _ := tree.CreateChild(ctx, root.ID, &category{Name: "a"})
	b, _ := tree.CreateChild(ctx, root.ID, &category{Name: "b"})
	c, _ := tree.CreateChild(ctx, root.ID, &category{Name: "c"})

	if err := tree.MoveUp(ctx, c.ID); err != nil {
		t.Fatalf("move up: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "a", "c", "b"})
	if err := tree.MoveDown(ctx, a.ID); err != nil {
		t.Fatalf("move down: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "c", "a", "b"})
	if err := tree.MoveToChildOf(ctx, root.ID, b.ID); err == nil {
		t.Fatal("expected invalid descendant move")
	}
}

func TestNestedSetRebuildTreePersistsPayload(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root := &nestedset.Node[category]{
		Model: &category{Name: "root"},
		Children: []*nestedset.Node[category]{
			{Model: &category{Name: "a"}},
			{Model: &category{Name: "b"}, Children: []*nestedset.Node[category]{{Model: &category{Name: "c"}}}},
		},
	}
	if err := tree.RebuildTree(ctx, []*nestedset.Node[category]{root}); err != nil {
		t.Fatalf("rebuild tree: %v", err)
	}
	assertOrder(t, ctx, tree, []string{"root", "a", "b", "c"})
	assertBounds(t, ctx, tree, map[string][3]int{
		"root": {1, 8, 0},
		"a":    {2, 3, 1},
		"b":    {4, 7, 1},
		"c":    {5, 6, 2},
	})
	if root.Model.ID == 0 || root.Children[0].Model.ID == 0 || root.Children[1].Children[0].Model.ID == 0 {
		t.Fatalf("rebuild tree should write IDs back to payload")
	}
}

func TestNestedSetCheckDetectsBrokenTree(t *testing.T) {
	db, ctx := openNestedSetDB(t)
	tree := nestedset.Use[category](db)

	root, _ := tree.CreateRoot(ctx, &category{Name: "root"})
	child, _ := tree.CreateChild(ctx, root.ID, &category{Name: "child"})
	if _, err := db.Use[category]().Where("ID", child.ID).Update(ctx, oro.Map{"Lft": 1}); err != nil {
		t.Fatalf("break tree: %v", err)
	}
	result, err := tree.Check(ctx)
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.Valid {
		t.Fatalf("expected invalid tree: %#v", result)
	}
}
