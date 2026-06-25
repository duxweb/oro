package oro

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

func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{fields: map[string]*FieldBuilder{}}
}

func (builder *SchemaBuilder) Table(name string) {
	builder.table = name
}

func (builder *SchemaBuilder) Connection(name string) {
	builder.connection = name
}

func (builder *SchemaBuilder) Shard(group string, fields ...string) {
	builder.shardGroup = group
	builder.shardFields = append([]string(nil), fields...)
}

func (builder *SchemaBuilder) Tenant(fields ...string) {
	builder.tenantFields = append([]string(nil), fields...)
	builder.noTenant = false
}

func (builder *SchemaBuilder) NoTenant() {
	builder.tenantFields = nil
	builder.noTenant = true
}

func (builder *SchemaBuilder) Field(name string) *FieldBuilder {
	field := builder.fields[name]
	if field == nil {
		field = &FieldBuilder{name: name}
		builder.fields[name] = field
	}
	return field
}

func (builder *SchemaBuilder) Index(name string, fields ...string) {
	builder.addIndex(indexBuilder{Name: name, Fields: fields})
}

func (builder *SchemaBuilder) Unique(name string, fields ...string) {
	builder.addIndex(indexBuilder{Name: name, Fields: fields, Unique: true})
}

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

func (field *FieldBuilder) Column(name string) *FieldBuilder {
	field.column = name
	return field
}

func (field *FieldBuilder) String() *FieldBuilder {
	field.setType("string")
	return field
}

func (field *FieldBuilder) Text() *FieldBuilder {
	field.setType("text")
	return field
}

func (field *FieldBuilder) Bool() *FieldBuilder {
	field.setType("bool")
	return field
}

func (field *FieldBuilder) Int() *FieldBuilder {
	field.setType("int")
	return field
}

func (field *FieldBuilder) BigInt() *FieldBuilder {
	field.setType("int64")
	return field
}

func (field *FieldBuilder) Uint() *FieldBuilder {
	field.setType("uint")
	return field
}

func (field *FieldBuilder) UnsignedInt() *FieldBuilder {
	field.setType("uint")
	return field
}

func (field *FieldBuilder) UnsignedBigInt() *FieldBuilder {
	field.setType("uint64")
	return field
}

func (field *FieldBuilder) Decimal(precision int, scale int) *FieldBuilder {
	field.setType("decimal")
	field.precision = precision
	field.scale = scale
	return field
}

func (field *FieldBuilder) Float() *FieldBuilder {
	field.setType("float")
	return field
}

func (field *FieldBuilder) Double() *FieldBuilder {
	field.setType("double")
	return field
}

func (field *FieldBuilder) Binary() *FieldBuilder {
	field.setType("binary")
	return field
}

func (field *FieldBuilder) JSON() *FieldBuilder {
	field.setType("json")
	return field
}

func (field *FieldBuilder) UUID() *FieldBuilder {
	field.setType("uuid")
	return field
}

func (field *FieldBuilder) Timestamp() *FieldBuilder {
	field.setType("time.Time")
	return field
}

func (field *FieldBuilder) Date() *FieldBuilder {
	field.setType("date")
	return field
}

func (field *FieldBuilder) Time() *FieldBuilder {
	field.setType("time")
	return field
}

func (field *FieldBuilder) Enum(values ...string) *FieldBuilder {
	field.setType("enum")
	field.enumValues = append([]string(nil), values...)
	return field
}

func (field *FieldBuilder) Email() *FieldBuilder {
	field.setType("email")
	return field
}

func (field *FieldBuilder) URL() *FieldBuilder {
	field.setType("url")
	return field
}

func (field *FieldBuilder) IP() *FieldBuilder {
	field.setType("ip")
	return field
}

func (field *FieldBuilder) MAC() *FieldBuilder {
	field.setType("mac")
	return field
}

func (field *FieldBuilder) Phone() *FieldBuilder {
	field.setType("phone")
	return field
}

func (field *FieldBuilder) Slug() *FieldBuilder {
	field.setType("slug")
	return field
}

func (field *FieldBuilder) Color() *FieldBuilder {
	field.setType("color")
	return field
}

func (field *FieldBuilder) StringArray() *FieldBuilder {
	field.setType("string_array")
	return field
}

func (field *FieldBuilder) IntArray() *FieldBuilder {
	field.setType("int_array")
	return field
}

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

func (field *FieldBuilder) Size(size int) *FieldBuilder {
	field.size = size
	field.sizeSet = true
	return field
}

func (field *FieldBuilder) NotNull() *FieldBuilder {
	nullable := false
	field.nullable = &nullable
	return field
}

func (field *FieldBuilder) Nullable() *FieldBuilder {
	nullable := true
	field.nullable = &nullable
	return field
}

func (field *FieldBuilder) Default(value any) *FieldBuilder {
	field.defaultVal = &DefaultSpec{Value: value}
	return field
}

func (field *FieldBuilder) DefaultExpr(expr string) *FieldBuilder {
	field.defaultVal = &DefaultSpec{Expr: expr}
	return field
}

func (field *FieldBuilder) Comment(comment string) *FieldBuilder {
	field.comment = comment
	return field
}

func (field *FieldBuilder) Primary() *FieldBuilder {
	field.primary = true
	return field
}

func (field *FieldBuilder) Ignore() *FieldBuilder {
	field.ignore = true
	return field
}

func (field *FieldBuilder) Virtual() *FieldBuilder {
	field.virtual = true
	return field
}

func (field *FieldBuilder) Hidden() *FieldBuilder {
	field.hidden = true
	return field
}

func (field *FieldBuilder) OptimisticLock() *FieldBuilder {
	field.optimistic = true
	return field
}

func (field *FieldBuilder) SoftDelete() *FieldBuilder {
	field.softDelete = true
	field.nullable = boolPtr(true)
	if field.fieldTyp == "" {
		field.setType("time.Time")
	}
	return field
}

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

func (field *FieldBuilder) Unique(name ...string) *FieldBuilder {
	if len(name) > 0 {
		field.unique = name[0]
	} else {
		field.unique = defaultIndexMarker
	}
	return field
}

func (field *FieldBuilder) FullText(name ...string) *FieldBuilder {
	if len(name) > 0 {
		field.fullText = name[0]
	} else {
		field.fullText = defaultIndexMarker
	}
	return field
}
