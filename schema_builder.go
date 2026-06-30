package oro

// SchemaBuilder builds a model schema from a Define method.
type SchemaBuilder struct {
	table        string
	connection   string
	shardGroup   string
	shardFields  []string
	tenantFields []string
	noTenant     bool
	fields       map[string]*FieldBuilder
	indexes      []indexBuilder
}

// NewSchemaBuilder creates an empty schema builder.
func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{fields: map[string]*FieldBuilder{}}
}

// Table sets the database table name for the model.
func (builder *SchemaBuilder) Table(name string) {
	builder.table = name
}

// Connection sets the named connection used by the model.
func (builder *SchemaBuilder) Connection(name string) {
	builder.connection = name
}

// Shard marks the model as sharded by group and key fields.
func (builder *SchemaBuilder) Shard(group string, fields ...string) {
	builder.shardGroup = group
	builder.shardFields = append([]string(nil), fields...)
}

// Tenant marks model fields used by the tenant extension.
func (builder *SchemaBuilder) Tenant(fields ...string) {
	builder.tenantFields = append([]string(nil), fields...)
	builder.noTenant = false
}

// NoTenant disables inherited tenant handling for the model.
func (builder *SchemaBuilder) NoTenant() {
	builder.tenantFields = nil
	builder.noTenant = true
}

// Field returns the builder for a Go struct field.
func (builder *SchemaBuilder) Field(name string) *FieldBuilder {
	field := builder.fields[name]
	if field == nil {
		field = &FieldBuilder{name: name}
		builder.fields[name] = field
	}
	return field
}

// Index adds a non-unique index across fields.
func (builder *SchemaBuilder) Index(name string, fields ...string) {
	builder.addIndex(indexBuilder{Name: name, Fields: fields})
}

// Unique adds a unique index across fields.
func (builder *SchemaBuilder) Unique(name string, fields ...string) {
	builder.addIndex(indexBuilder{Name: name, Fields: fields, Unique: true})
}

// FullText adds a full-text index across fields.
func (builder *SchemaBuilder) FullText(name string, fields ...string) {
	builder.addIndex(indexBuilder{Name: name, Fields: fields, FullText: true})
}

func (builder *SchemaBuilder) addIndex(index indexBuilder) {
	for _, existing := range builder.indexes {
		if existing.Name == index.Name && existing.Unique == index.Unique && existing.FullText == index.FullText && stringSlicesEqual(existing.Fields, index.Fields) {
			return
		}
	}
	builder.indexes = append(builder.indexes, index)
}

type indexBuilder struct {
	Name     string
	Fields   []string
	Unique   bool
	FullText bool
}

func stringSlicesEqual(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

const defaultIndexMarker = "__oro_default__"

// FieldBuilder builds database metadata for one model field.
type FieldBuilder struct {
	name         string
	column       string
	fieldTyp     string
	typeConflict bool
	size         int
	sizeSet      bool
	precision    int
	scale        int
	defaultVal   *DefaultSpec
	enumValues   []string
	comment      string
	nullable     *bool
	primary      bool
	ignore       bool
	virtual      bool
	hidden       bool
	optimistic   bool
	softDelete   bool
	index        string
	unique       string
	fullText     string
}

// Column sets the database column name for the field.
func (field *FieldBuilder) Column(name string) *FieldBuilder {
	field.column = name
	return field
}

// String marks the field as a string column.
func (field *FieldBuilder) String() *FieldBuilder {
	field.setType("string")
	return field
}

// Text marks the field as a text column.
func (field *FieldBuilder) Text() *FieldBuilder {
	field.setType("text")
	return field
}

// Bool marks the field as a bool column.
func (field *FieldBuilder) Bool() *FieldBuilder {
	field.setType("bool")
	return field
}

// Int marks the field as a signed integer column.
func (field *FieldBuilder) Int() *FieldBuilder {
	field.setType("int")
	return field
}

// BigInt marks the field as a signed 64-bit integer column.
func (field *FieldBuilder) BigInt() *FieldBuilder {
	field.setType("int64")
	return field
}

// Uint marks the field as an unsigned integer column.
func (field *FieldBuilder) Uint() *FieldBuilder {
	field.setType("uint")
	return field
}

// UnsignedInt marks the field as an unsigned integer column.
func (field *FieldBuilder) UnsignedInt() *FieldBuilder {
	field.setType("uint")
	return field
}

// UnsignedBigInt marks the field as an unsigned 64-bit integer column.
func (field *FieldBuilder) UnsignedBigInt() *FieldBuilder {
	field.setType("uint64")
	return field
}

// Decimal marks the field as a fixed precision decimal column.
func (field *FieldBuilder) Decimal(precision int, scale int) *FieldBuilder {
	field.setType("decimal")
	field.precision = precision
	field.scale = scale
	return field
}

// Float marks the field as a single-precision floating point column.
func (field *FieldBuilder) Float() *FieldBuilder {
	field.setType("float")
	return field
}

// Double marks the field as a double-precision floating point column.
func (field *FieldBuilder) Double() *FieldBuilder {
	field.setType("double")
	return field
}

// Binary marks the field as a binary column.
func (field *FieldBuilder) Binary() *FieldBuilder {
	field.setType("binary")
	return field
}

// JSON marks the field as a JSON column.
func (field *FieldBuilder) JSON() *FieldBuilder {
	field.setType("json")
	return field
}

// UUID marks the field as a UUID column.
func (field *FieldBuilder) UUID() *FieldBuilder {
	field.setType("uuid")
	return field
}

// Timestamp marks the field as a timestamp column.
func (field *FieldBuilder) Timestamp() *FieldBuilder {
	field.setType("time.Time")
	return field
}

// Date marks the field as a date column.
func (field *FieldBuilder) Date() *FieldBuilder {
	field.setType("date")
	return field
}

// Time marks the field as a time-of-day column.
func (field *FieldBuilder) Time() *FieldBuilder {
	field.setType("time")
	return field
}

// Enum marks the field as an enum column with allowed values.
func (field *FieldBuilder) Enum(values ...string) *FieldBuilder {
	field.setType("enum")
	field.enumValues = append([]string(nil), values...)
	return field
}

// Email marks the field as an email string column.
func (field *FieldBuilder) Email() *FieldBuilder {
	field.setType("email")
	return field
}

// URL marks the field as a URL string column.
func (field *FieldBuilder) URL() *FieldBuilder {
	field.setType("url")
	return field
}

// IP marks the field as an IP string column.
func (field *FieldBuilder) IP() *FieldBuilder {
	field.setType("ip")
	return field
}

// MAC marks the field as a MAC address string column.
func (field *FieldBuilder) MAC() *FieldBuilder {
	field.setType("mac")
	return field
}

// Phone marks the field as a phone string column.
func (field *FieldBuilder) Phone() *FieldBuilder {
	field.setType("phone")
	return field
}

// Slug marks the field as a slug string column.
func (field *FieldBuilder) Slug() *FieldBuilder {
	field.setType("slug")
	return field
}

// Color marks the field as a color string column.
func (field *FieldBuilder) Color() *FieldBuilder {
	field.setType("color")
	return field
}

// StringArray marks the field as an array of strings.
func (field *FieldBuilder) StringArray() *FieldBuilder {
	field.setType("string_array")
	return field
}

// IntArray marks the field as an array of integers.
func (field *FieldBuilder) IntArray() *FieldBuilder {
	field.setType("int_array")
	return field
}

// Point marks the field as a point/geometric coordinate column.
func (field *FieldBuilder) Point() *FieldBuilder {
	field.setType("point")
	return field
}

func (field *FieldBuilder) setType(fieldType string) {
	if field.fieldTyp != "" && field.fieldTyp != fieldType {
		field.typeConflict = true
	}
	field.fieldTyp = fieldType
}

// Size sets the field size or length.
func (field *FieldBuilder) Size(size int) *FieldBuilder {
	field.size = size
	field.sizeSet = true
	return field
}

// NotNull marks the field as not nullable.
func (field *FieldBuilder) NotNull() *FieldBuilder {
	nullable := false
	field.nullable = &nullable
	return field
}

// Nullable marks the field as nullable.
func (field *FieldBuilder) Nullable() *FieldBuilder {
	nullable := true
	field.nullable = &nullable
	return field
}

// Default sets a literal default value.
func (field *FieldBuilder) Default(value any) *FieldBuilder {
	field.defaultVal = &DefaultSpec{Value: value}
	return field
}

// DefaultExpr sets a database expression default value.
func (field *FieldBuilder) DefaultExpr(expr string) *FieldBuilder {
	field.defaultVal = &DefaultSpec{Expr: expr}
	return field
}

// Comment sets the field comment.
func (field *FieldBuilder) Comment(comment string) *FieldBuilder {
	field.comment = comment
	return field
}

// Primary marks the field as a primary key.
func (field *FieldBuilder) Primary() *FieldBuilder {
	field.primary = true
	return field
}

// Ignore excludes the field from schema parsing and persistence.
func (field *FieldBuilder) Ignore() *FieldBuilder {
	field.ignore = true
	return field
}

// Virtual marks the field as not backed by a database column.
func (field *FieldBuilder) Virtual() *FieldBuilder {
	field.virtual = true
	return field
}

// Hidden excludes the field from default SELECT lists and serialization helpers.
func (field *FieldBuilder) Hidden() *FieldBuilder {
	field.hidden = true
	return field
}

// OptimisticLock marks the field as an optimistic locking version column.
func (field *FieldBuilder) OptimisticLock() *FieldBuilder {
	field.optimistic = true
	return field
}

// SoftDelete marks the field as the soft-delete timestamp column.
func (field *FieldBuilder) SoftDelete() *FieldBuilder {
	field.softDelete = true
	field.nullable = boolPtr(true)
	if field.fieldTyp == "" {
		field.setType("time.Time")
	}
	return field
}

// Index adds an index for this field.
func (field *FieldBuilder) Index(name ...string) *FieldBuilder {
	if len(name) > 0 {
		field.index = name[0]
	} else {
		field.index = defaultIndexMarker
	}
	return field
}

func boolPtr(value bool) *bool {
	return &value
}

// Unique adds a unique index for this field.
func (field *FieldBuilder) Unique(name ...string) *FieldBuilder {
	if len(name) > 0 {
		field.unique = name[0]
	} else {
		field.unique = defaultIndexMarker
	}
	return field
}

// FullText adds a full-text index for this field.
func (field *FieldBuilder) FullText(name ...string) *FieldBuilder {
	if len(name) > 0 {
		field.fullText = name[0]
	} else {
		field.fullText = defaultIndexMarker
	}
	return field
}
