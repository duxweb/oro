package nestedset_test

import (
	"testing"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/extensions/internal/exttest"
	"github.com/duxweb/oro/extensions/nestedset"
)

func TestNestedSetDriverMatrix(t *testing.T) {
	for _, testCase := range exttest.DriverCases() {
		t.Run(testCase.Name, func(t *testing.T) {
			db, ctx := exttest.Open(t, testCase, exttest.OpenOptions{
				Models:     []oro.Definer{category{}},
				Tables:     []string{"nested_categories"},
				Prefix:     "nestedset_matrix_",
				Extensions: []oro.Extension{nestedset.Extension()},
			})
			tree := nestedset.Use[category](db)

			root, err := tree.CreateRoot(ctx, &category{Name: "root"})
			if err != nil {
				t.Fatalf("create root: %v", err)
			}
			first, err := tree.CreateChild(ctx, root.ID, &category{Name: "first"})
			if err != nil {
				t.Fatalf("create first: %v", err)
			}
			second, err := tree.CreateAfter(ctx, first.ID, &category{Name: "second"})
			if err != nil {
				t.Fatalf("create second: %v", err)
			}
			grand, err := tree.CreateFirstChild(ctx, second.ID, &category{Name: "grand"})
			if err != nil {
				t.Fatalf("create grand: %v", err)
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

			if err := tree.MoveBefore(ctx, second.ID, first.ID); err != nil {
				t.Fatalf("move before: %v", err)
			}
			assertOrder(t, ctx, tree, []string{"root", "second", "grand", "first"})

			deleted, err := tree.Delete(ctx, second.ID)
			if err != nil {
				t.Fatalf("delete: %v", err)
			}
			if deleted != 2 {
				t.Fatalf("deleted = %d, want 2", deleted)
			}
			assertOrder(t, ctx, tree, []string{"root", "first"})
		})
	}
}
