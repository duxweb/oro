package oro

import "testing"

func TestDiffTableSpecCreatesTable(t *testing.T) {
	changes := diffTableSpec(nil, TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
		},
	})
	if len(changes) != 1 || changes[0].Kind != SchemaCreateTable {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecAddsColumnAndIndex(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "integer", Primary: true},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
			{ColumnName: "code", Type: "string"},
		},
		Indexes: []IndexSpec{
			{Name: "idx_products_code", Fields: []string{"code"}},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 2 {
		t.Fatalf("unexpected changes %#v", changes)
	}
	if changes[0].Kind != SchemaAddColumn || changes[0].Column.ColumnName != "code" {
		t.Fatalf("unexpected add column change %#v", changes[0])
	}
	if changes[1].Kind != SchemaCreateIndex || changes[1].Index.Name != "idx_products_code" {
		t.Fatalf("unexpected index change %#v", changes[1])
	}
}

func TestDiffTableSpecDetectsUnsafeIndexChange(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "integer", Primary: true},
			{ColumnName: "code", Type: "text"},
		},
		Indexes: []IndexSpec{
			{Name: "idx_products_code", Fields: []string{"code"}},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
			{ColumnName: "code", Type: "string"},
		},
		Indexes: []IndexSpec{
			{Name: "idx_products_code", Fields: []string{"code"}, Unique: true},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 1 || changes[0].Kind != SchemaUnsafeChange {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecDetectsUnsafeFullTextIndexChange(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "integer", Primary: true},
			{ColumnName: "code", Type: "text"},
		},
		Indexes: []IndexSpec{
			{Name: "idx_products_code", Fields: []string{"code"}},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
			{ColumnName: "code", Type: "string"},
		},
		Indexes: []IndexSpec{
			{Name: "idx_products_code", Fields: []string{"code"}, FullText: true},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 1 || changes[0].Kind != SchemaUnsafeChange {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecDetectsRemovedColumn(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "integer", Primary: true},
			{ColumnName: "old_code", Type: "text"},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 1 || changes[0].Kind != SchemaUnsafeChange || changes[0].Current.ColumnName != "old_code" {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecDetectsUnsafeColumnTypeChange(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "integer", Primary: true},
			{ColumnName: "code", Type: "text"},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
			{ColumnName: "code", Type: "integer"},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 1 || changes[0].Kind != SchemaUnsafeChange || changes[0].Column.ColumnName != "code" {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecAllowsCompatibleColumnType(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "integer", Primary: true},
			{ColumnName: "code", Type: "text", Nullable: false},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
			{ColumnName: "code", Type: "string", Nullable: true},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 0 {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecAllowsDriverColumnTypeVariants(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "bigint unsigned", Primary: true},
			{ColumnName: "code", Type: "varchar(255)", Nullable: true},
			{ColumnName: "price", Type: "int unsigned", Nullable: true},
			{ColumnName: "stock", Type: "int unsigned", Nullable: true},
			{ColumnName: "meta", Type: "json", Nullable: true},
			{ColumnName: "created_at", Type: "datetime", Nullable: true},
			{ColumnName: "updated_at", Type: "timestamp without time zone", Nullable: true},
			{ColumnName: "deleted_at", Type: "timestamp without time zone", Nullable: true},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
			{ColumnName: "code", Type: "string", Nullable: true},
			{ColumnName: "price", Type: "uint", Nullable: true},
			{ColumnName: "stock", Type: "oro.Null[uint]", Nullable: true},
			{ColumnName: "meta", Type: "oro.JSONRaw", Nullable: true},
			{ColumnName: "created_at", Type: "time.Time", Nullable: true},
			{ColumnName: "updated_at", Type: "time.Time", Nullable: true},
			{ColumnName: "deleted_at", Type: "oro.Null[time.Time]", Nullable: true},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 0 {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecAllowsLogicalTypesFromSnapshot(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "active", Type: "tinyint(1)", Nullable: true},
			{ColumnName: "meta", Type: "jsonb", Nullable: true},
			{ColumnName: "tags", Type: "json", Nullable: true},
			{ColumnName: "location", Type: "point", Nullable: true},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "active", Type: "bool", Nullable: true},
			{ColumnName: "meta", Type: "json", Nullable: true},
			{ColumnName: "tags", Type: "string_array", Nullable: true},
			{ColumnName: "location", Type: "point", Nullable: true},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 0 {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecUsesSnapshotLogicalTypeForExistingColumns(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "Meta", ColumnName: "meta", Type: "longtext", Nullable: true},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "Meta", ColumnName: "meta", Type: "json", Nullable: true},
		},
	}
	snapshot := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "Meta", ColumnName: "meta", Type: "json", Nullable: true},
		},
	}

	changes := diffTableSpecWithSnapshot(current, target, snapshot)
	if len(changes) != 0 {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecDetectsNullableTightening(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "integer", Primary: true},
			{ColumnName: "code", Type: "text", Nullable: true},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{ColumnName: "id", Type: "uint64", Primary: true},
			{ColumnName: "code", Type: "string", Nullable: false},
		},
	}

	changes := diffTableSpec(current, target)
	if len(changes) != 1 || changes[0].Kind != SchemaUnsafeChange || changes[0].Column.ColumnName != "code" {
		t.Fatalf("unexpected changes %#v", changes)
	}
}

func TestDiffTableSpecDetectsRenameFromSnapshot(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "ID", ColumnName: "id", Type: "integer", Primary: true},
			{FieldName: "Code", ColumnName: "old_code", Type: "text"},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "ID", ColumnName: "id", Type: "uint64", Primary: true},
			{FieldName: "Code", ColumnName: "code", Type: "string"},
		},
	}
	snapshot := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "ID", ColumnName: "id", Type: "uint64", Primary: true},
			{FieldName: "Code", ColumnName: "old_code", Type: "string"},
		},
	}

	changes := diffTableSpecWithSnapshot(current, target, snapshot)
	if len(changes) != 1 || changes[0].Kind != SchemaRenameColumn {
		t.Fatalf("unexpected changes %#v", changes)
	}
	if changes[0].Current.ColumnName != "old_code" || changes[0].Column.ColumnName != "code" {
		t.Fatalf("unexpected rename change %#v", changes[0])
	}
}

func TestDiffTableSpecRejectsAmbiguousRenameFromSnapshot(t *testing.T) {
	current := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "Code", ColumnName: "old_code", Type: "text"},
			{FieldName: "Name", ColumnName: "old_name", Type: "text"},
		},
	}
	target := TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "Code", ColumnName: "code", Type: "string"},
			{FieldName: "Name", ColumnName: "name", Type: "string"},
		},
	}
	snapshot := &TableSpec{
		Name: "products",
		Columns: []ColumnSpec{
			{FieldName: "Code", ColumnName: "old_code", Type: "string"},
			{FieldName: "Name", ColumnName: "old_name", Type: "string"},
		},
	}

	changes := diffTableSpecWithSnapshot(current, target, snapshot)
	if len(changes) != 1 || changes[0].Kind != SchemaUnsafeChange {
		t.Fatalf("unexpected changes %#v", changes)
	}
}
