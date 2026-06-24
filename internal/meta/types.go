package meta

import (
	"reflect"

	"github.com/duxweb/oro/internal/queryast"
)

type RelationKind string

const (
	RelationBelongsTo         RelationKind = "belongs_to"
	RelationHasOne            RelationKind = "has_one"
	RelationHasMany           RelationKind = "has_many"
	RelationManyToMany        RelationKind = "many_to_many"
	RelationDynamicBelongsTo  RelationKind = "dynamic_belongs_to"
	RelationDynamicHasMany    RelationKind = "dynamic_has_many"
	RelationDynamicManyToMany RelationKind = "dynamic_many_to_many"
)

type RelationSchema struct {
	Kind             RelationKind
	Name             string
	SourceModel      string
	SourceTable      string
	TargetModel      string
	ForeignKey       string
	ReferenceKey     string
	Through          string
	SourceForeignKey string
	TargetForeignKey string
	IDField          string
	TypeField        string
	TypeValue        string
	SourceTypeField  string
	SourceTypeValue  string
	JSONName         string
}

type ModelSchema struct {
	Name           string
	Table          string
	Connection     string
	ShardGroup     string
	ShardFields    []string
	TenantFields   []string
	NoTenant       bool
	Fields         []FieldSchema
	FieldByGo      map[string]FieldSchema
	FieldByDB      map[string]FieldSchema
	Primary        []string
	PrimaryColumns []string
	Indexes        []IndexSpec
	Relations      []RelationSchema
	DefaultSelect  []string
	DefaultExprs   []queryast.SelectExpr
	InsertFields   []FieldSchema
	ModelIndex     []int
	Type           reflect.Type
}

type FieldSchema struct {
	Name       string
	Column     string
	Type       string
	Index      []int
	Size       int
	SizeSet    bool
	Precision  int
	Scale      int
	Default    *DefaultSpec
	EnumValues []string
	Comment    string
	Primary    bool
	Nullable   bool
	Ignore     bool
	Virtual    bool
	Hidden     bool
	Optimistic bool
	AutoCreate bool
	AutoUpdate bool
	SoftDelete bool
}

type TableSpec struct {
	Name    string
	Columns []ColumnSpec
	Indexes []IndexSpec
}

type ColumnSpec struct {
	FieldName  string
	ColumnName string
	Type       string
	Size       int
	SizeSet    bool
	Precision  int
	Scale      int
	Default    *DefaultSpec
	EnumValues []string
	Comment    string
	Primary    bool
	Nullable   bool
}

type DefaultSpec struct {
	Value any
	Expr  string
}

type IndexSpec struct {
	Name     string
	Fields   []string
	Unique   bool
	FullText bool
}

type SchemaChangeKind int

const (
	SchemaCreateTable SchemaChangeKind = iota + 1
	SchemaAddColumn
	SchemaCreateIndex
	SchemaUnsafeChange
	SchemaRenameColumn
)

type SchemaChange struct {
	Kind    SchemaChangeKind
	Table   TableSpec
	Column  ColumnSpec
	Current ColumnSpec
	Index   IndexSpec
}
