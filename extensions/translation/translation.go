package translation

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/duxweb/oro"
)

const extensionName = "translation"

const defaultFieldName = "Translations"
const defaultColumnName = "translations"

type Fields struct {
	Translations oro.JSONRaw
}

func (Fields) OroEmbeddedFields() {}

func (Fields) DefineOroFields(s *oro.SchemaBuilder) {
	s.Field(defaultFieldName).Column(defaultColumnName).JSON().Nullable().Hidden()
}

type Values map[string]oro.Map

type Config struct {
	DefaultLocale  string
	FallbackLocale string
	FieldName      string
	Fields         []string
}

type Option interface {
	applyTranslationOption(*Config)
}

type optionFunc func(*Config)

func (fn optionFunc) applyTranslationOption(config *Config) {
	fn(config)
}

func DefaultLocale(locale string) Option {
	return optionFunc(func(config *Config) {
		config.DefaultLocale = locale
	})
}

func FallbackLocale(locale string) Option {
	return optionFunc(func(config *Config) {
		config.FallbackLocale = locale
	})
}

func FieldName(field string) Option {
	return optionFunc(func(config *Config) {
		config.FieldName = field
	})
}

func TranslatedFields(fields ...string) Option {
	return optionFunc(func(config *Config) {
		config.Fields = append([]string(nil), fields...)
	})
}

type extension struct {
	config Config
}

func Extension(options ...Option) oro.Extension {
	return extension{config: resolveConfig(options)}
}

func (extension extension) Name() string {
	return extensionName
}

func (extension extension) Install(db *oro.DB) error {
	return nil
}

func (extension extension) State() any {
	return extension.config
}

func Use[T any](db *oro.DB, options ...Option) *Query[T] {
	config := resolveConfig(options)
	if configured, ok := configFromDB(db); ok {
		config = mergeConfig(configured, config)
	}
	return &Query[T]{db: db, config: config}
}

func WithLocale(ctx context.Context, locale string) context.Context {
	current, _ := stateFromContext(ctx)
	current.locale = locale
	return context.WithValue(ctx, contextKey{}, current)
}

func WithFallback(ctx context.Context, fallback string) context.Context {
	current, _ := stateFromContext(ctx)
	current.fallback = fallback
	return context.WithValue(ctx, contextKey{}, current)
}

func Locale(ctx context.Context) (string, bool) {
	state, ok := stateFromContext(ctx)
	return state.locale, ok && state.locale != ""
}

type Query[T any] struct {
	db       *oro.DB
	query    *oro.ModelQuery[T]
	config   Config
	locale   string
	fallback string
}

func (query *Query[T]) base() *oro.ModelQuery[T] {
	if query.query != nil {
		return query.query
	}
	return query.db.Use[T]()
}

func (query *Query[T]) clone(next *oro.ModelQuery[T]) *Query[T] {
	clone := *query
	clone.query = next
	return &clone
}

func (query *Query[T]) Locale(locale string) *Query[T] {
	clone := *query
	clone.locale = locale
	return &clone
}

func (query *Query[T]) Fallback(locale string) *Query[T] {
	clone := *query
	clone.fallback = locale
	return &clone
}

func (query *Query[T]) Where(field any, args ...any) *Query[T] {
	return query.clone(query.base().Where(field, args...))
}

func (query *Query[T]) OrWhere(field any, args ...any) *Query[T] {
	return query.clone(query.base().OrWhere(field, args...))
}

func (query *Query[T]) WhereGroup(fn func(w *oro.WhereBuilder)) *Query[T] {
	return query.clone(query.base().WhereGroup(fn))
}

func (query *Query[T]) OrWhereGroup(fn func(w *oro.WhereBuilder)) *Query[T] {
	return query.clone(query.base().OrWhereGroup(fn))
}

func (query *Query[T]) WhereWhen(condition bool, fn func(w *oro.WhereBuilder)) *Query[T] {
	return query.clone(query.base().WhereWhen(condition, fn))
}

func (query *Query[T]) WhereRaw(sql string, args ...any) *Query[T] {
	return query.clone(query.base().WhereRaw(sql, args...))
}

func (query *Query[T]) WhereTrans(field string, value any) *Query[T] {
	return query.transCondition(field, "=", value)
}

func (query *Query[T]) WhereTransLike(field string, value any) *Query[T] {
	clone := *query
	clone.query = clone.base().Where(oro.Condition{Op: "invalid", Value: &oro.Error{Op: "translation.where", Kind: oro.ErrUnsupported, Field: field}})
	return &clone
}

func (query *Query[T]) transCondition(field string, op string, value any) *Query[T] {
	if !query.isTranslatedField(field) {
		clone := *query
		clone.query = clone.base().Where(oro.Condition{Op: "invalid", Value: &oro.Error{Op: "translation.where", Kind: oro.ErrUnknownField, Field: field}})
		return &clone
	}
	locale := query.effectiveLocale(context.Background())
	if locale == "" {
		clone := *query
		clone.query = clone.base().Where(oro.Condition{Op: "invalid", Value: &oro.Error{Op: "translation.where", Kind: oro.ErrInvalidArgument, Field: field}})
		return &clone
	}
	condition := oro.JSON(query.config.field()).Path(locale, field)
	_ = op
	return query.clone(query.base().Where(condition.Eq(value)))
}

func (query *Query[T]) OrderBy(fields ...string) *Query[T] {
	return query.clone(query.base().OrderBy(fields...))
}

func (query *Query[T]) OrderByDesc(fields ...string) *Query[T] {
	return query.clone(query.base().OrderByDesc(fields...))
}

func (query *Query[T]) Limit(limit int) *Query[T] {
	return query.clone(query.base().Limit(limit))
}

func (query *Query[T]) Offset(offset int) *Query[T] {
	return query.clone(query.base().Offset(offset))
}

func (query *Query[T]) With(relation any, callbacks ...func(*oro.RelationQuery)) *Query[T] {
	return query.clone(query.base().With(relation, callbacks...))
}

func (query *Query[T]) First(ctx context.Context) (*T, error) {
	model, err := query.withTranslationField().First(ctx)
	if err != nil || model == nil {
		return model, err
	}
	return model, query.applyModel(ctx, model)
}

func (query *Query[T]) Find(ctx context.Context, id any) (*T, error) {
	model, err := query.withTranslationField().Find(ctx, id)
	if err != nil || model == nil {
		return model, err
	}
	return model, query.applyModel(ctx, model)
}

func (query *Query[T]) Get(ctx context.Context) ([]*T, error) {
	models, err := query.withTranslationField().Get(ctx)
	if err != nil {
		return nil, err
	}
	for _, model := range models {
		if err := query.applyModel(ctx, model); err != nil {
			return nil, err
		}
	}
	return models, nil
}

func (query *Query[T]) Count(ctx context.Context) (int64, error) {
	return query.base().Count(ctx)
}

func (query *Query[T]) Exists(ctx context.Context) (bool, error) {
	return query.base().Exists(ctx)
}

func (query *Query[T]) Create(ctx context.Context, model *T, values ...Values) (*T, error) {
	if model == nil {
		return nil, &oro.Error{Op: "translation.create", Kind: oro.ErrInvalidArgument}
	}
	if err := query.prepareModelForWrite(ctx, model, values...); err != nil {
		return nil, err
	}
	created, err := query.base().Create(ctx, model)
	if err != nil || created == nil {
		return created, err
	}
	return created, query.applyModel(ctx, created)
}

func (query *Query[T]) Update(ctx context.Context, values oro.Map, transValues ...Values) (int64, error) {
	mapped, hasTranslations, err := query.prepareMapForUpdate(ctx, values)
	if err != nil {
		return 0, err
	}
	if hasTranslations || len(transValues) > 0 {
		if query.canAffectMany(ctx) {
			return 0, &oro.Error{Op: "translation.update", Kind: oro.ErrUnsupported}
		}
		merged, err := query.mergeExistingTranslations(ctx, mapped, transValues...)
		if err != nil {
			return 0, err
		}
		mapped = merged
	}
	return query.base().Update(ctx, mapped)
}

func (query *Query[T]) Delete(ctx context.Context) (int64, error) {
	return query.base().Delete(ctx)
}

func (query *Query[T]) ForceDelete(ctx context.Context) (int64, error) {
	return query.base().ForceDelete(ctx)
}

func (query *Query[T]) Restore(ctx context.Context) (int64, error) {
	return query.base().Restore(ctx)
}

func (query *Query[T]) applyModel(ctx context.Context, model *T) error {
	translations, err := translationsFromModel(model, query.config.field())
	if err != nil || len(translations) == 0 {
		return err
	}
	locale := query.effectiveLocale(ctx)
	fallback := query.effectiveFallback(ctx)
	return applyTranslations(model, translations, locale, fallback, query.config.Fields)
}

func (query *Query[T]) prepareModelForWrite(ctx context.Context, model *T, values ...Values) error {
	translations, err := translationsFromModel(model, query.config.field())
	if err != nil {
		return err
	}
	if translations == nil {
		translations = Values{}
	}
	if len(values) > 0 {
		filtered := query.filterValues(values[0])
		mergeTranslations(translations, filtered)
		if err := query.applyOriginalFallbackForCreate(ctx, model, filtered); err != nil {
			return err
		}
	} else {
		row, err := query.translationValuesFromModel(model)
		if err != nil {
			return err
		}
		if len(row) > 0 {
			locale := query.effectiveLocale(ctx)
			if locale == "" {
				return &oro.Error{Op: "translation.create", Kind: oro.ErrInvalidArgument, Field: "locale"}
			}
			translations[locale] = row
		}
	}
	return setTranslations(model, query.config.field(), translations)
}

func (query *Query[T]) applyOriginalFallbackForCreate(ctx context.Context, model *T, values Values) error {
	locale := query.config.DefaultLocale
	if locale == "" {
		locale = query.effectiveLocale(ctx)
		if locale == "" {
			locale = firstLocale(values)
		}
	}
	row := values[locale]
	if len(row) == 0 {
		return nil
	}
	for field, value := range row {
		current, ok := modelFieldValue(model, field)
		if ok && !isZeroAny(current) {
			continue
		}
		if err := setModelFieldValue(model, field, value); err != nil {
			return err
		}
	}
	return nil
}

func (query *Query[T]) prepareMapForUpdate(ctx context.Context, values oro.Map) (oro.Map, bool, error) {
	mapped := copyMap(values)
	if len(mapped) == 0 {
		return mapped, false, nil
	}
	row := oro.Map{}
	for key, value := range mapped {
		if query.isTranslatedField(key) {
			row[key] = value
			delete(mapped, key)
		}
	}
	if len(row) == 0 {
		return mapped, false, nil
	}
	locale := query.effectiveLocale(ctx)
	if locale == "" {
		return nil, false, &oro.Error{Op: "translation.update", Kind: oro.ErrInvalidArgument, Field: "locale"}
	}
	translations := Values{}
	if len(row) > 0 {
		translations[locale] = row
	}
	query.syncOriginalValues(mapped, translations)
	if len(translations) > 0 {
		mapped[query.config.field()] = mustMarshalRaw(translations)
	}
	return mapped, len(translations) > 0, nil
}

func (query *Query[T]) mergeExistingTranslations(ctx context.Context, mapped oro.Map, transValues ...Values) (oro.Map, error) {
	model, err := query.withTranslationField().First(ctx)
	if err != nil || model == nil {
		return mapped, err
	}
	translations, err := translationsFromModel(model, query.config.field())
	if err != nil {
		return nil, err
	}
	if translations == nil {
		translations = Values{}
	}
	if raw, ok := mapped[query.config.field()]; ok {
		next, err := normalizeTranslations(raw)
		if err != nil {
			return nil, err
		}
		mergeTranslations(translations, next)
		query.syncOriginalValues(mapped, next)
		delete(mapped, query.config.field())
	}
	for _, values := range transValues {
		filtered := query.filterValues(values)
		mergeTranslations(translations, filtered)
		query.syncOriginalValues(mapped, filtered)
	}
	mapped[query.config.field()] = mustMarshalRaw(translations)
	return mapped, nil
}

func (query *Query[T]) withTranslationField() *oro.ModelQuery[T] {
	modelQuery := query.base()
	schema, err := oro.SchemaOf[T](query.db)
	if err != nil {
		return modelQuery.SelectHidden(query.config.field())
	}
	selects := make([]any, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		if field.Hidden || field.Ignore || field.Virtual {
			continue
		}
		selects = append(selects, field.Name)
	}
	if len(selects) > 0 {
		modelQuery = modelQuery.Select(selects...)
	}
	return modelQuery.SelectHidden(query.config.field())
}

func (query *Query[T]) effectiveLocale(ctx context.Context) string {
	if query.locale != "" {
		return query.locale
	}
	if state, ok := stateFromContext(ctx); ok && state.locale != "" {
		return state.locale
	}
	return query.config.DefaultLocale
}

func (query *Query[T]) effectiveFallback(ctx context.Context) string {
	if query.fallback != "" {
		return query.fallback
	}
	if state, ok := stateFromContext(ctx); ok && state.fallback != "" {
		return state.fallback
	}
	return query.config.FallbackLocale
}

func (query *Query[T]) isTranslatedField(field string) bool {
	if field == "" || field == query.config.field() {
		return false
	}
	for _, item := range query.config.Fields {
		if item == field {
			return true
		}
	}
	return false
}

func (query *Query[T]) filterValues(values Values) Values {
	if len(values) == 0 {
		return nil
	}
	out := Values{}
	for locale, row := range values {
		for field, value := range row {
			if !query.isTranslatedField(field) {
				continue
			}
			if out[locale] == nil {
				out[locale] = oro.Map{}
			}
			out[locale][field] = value
		}
	}
	return out
}

func (query *Query[T]) canAffectMany(ctx context.Context) bool {
	count, err := query.base().Count(ctx)
	return err != nil || count != 1
}

func (query *Query[T]) translationValuesFromModel(model any) (oro.Map, error) {
	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() || modelValue.Kind() != reflect.Pointer || modelValue.IsNil() {
		return nil, &oro.Error{Op: "translation", Kind: oro.ErrInvalidArgument}
	}
	structValue := modelValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return nil, &oro.Error{Op: "translation", Kind: oro.ErrInvalidArgument}
	}
	row := oro.Map{}
	for _, field := range reflect.VisibleFields(structValue.Type()) {
		if !field.IsExported() || field.Anonymous || !query.isTranslatedField(field.Name) {
			continue
		}
		fieldValue := structValue.FieldByIndex(field.Index)
		if !fieldValue.IsValid() || !fieldValue.CanInterface() || fieldValue.IsZero() {
			continue
		}
		row[field.Name] = fieldValue.Interface()
	}
	return row, nil
}

func (query *Query[T]) syncOriginalValues(mapped oro.Map, values Values) {
	locale := query.config.DefaultLocale
	if locale == "" {
		return
	}
	row := values[locale]
	for field, value := range row {
		if !query.isTranslatedField(field) {
			continue
		}
		if _, exists := mapped[field]; !exists {
			mapped[field] = value
		}
	}
}

type Translator[T any] struct {
	model    *T
	locale   string
	fallback string
	field    string
}

func Translate[T any](model *T, locale string, fallback ...string) *Translator[T] {
	fb := ""
	if len(fallback) > 0 {
		fb = fallback[0]
	}
	return &Translator[T]{model: model, locale: locale, fallback: fb, field: defaultFieldName}
}

func (translator *Translator[T]) Get(field string) any {
	translations, err := translationsFromModel(translator.model, translator.field)
	if err != nil {
		return nil
	}
	if value, ok := translatedValue(translations, translator.locale, field); ok {
		return value
	}
	if translator.fallback != "" {
		if value, ok := translatedValue(translations, translator.fallback, field); ok {
			return value
		}
	}
	value, _ := modelFieldValue(translator.model, field)
	return value
}

func (translator *Translator[T]) String(field string) string {
	value := translator.Get(field)
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func (translator *Translator[T]) Set(field string, value any) error {
	translations, err := translationsFromModel(translator.model, translator.field)
	if err != nil {
		return err
	}
	if translations == nil {
		translations = Values{}
	}
	row := translations[translator.locale]
	if row == nil {
		row = oro.Map{}
		translations[translator.locale] = row
	}
	row[field] = value
	return setTranslations(translator.model, translator.field, translations)
}

func resolveConfig(options []Option) Config {
	config := Config{DefaultLocale: "", FallbackLocale: "", FieldName: defaultFieldName}
	for _, option := range options {
		if option != nil {
			option.applyTranslationOption(&config)
		}
	}
	return config
}

func mergeConfig(base Config, override Config) Config {
	if override.DefaultLocale != "" {
		base.DefaultLocale = override.DefaultLocale
	}
	if override.FallbackLocale != "" {
		base.FallbackLocale = override.FallbackLocale
	}
	if override.FieldName != "" && override.FieldName != defaultFieldName {
		base.FieldName = override.FieldName
	}
	if len(override.Fields) > 0 {
		base.Fields = append([]string(nil), override.Fields...)
	}
	return base
}

func configFromDB(db *oro.DB) (Config, bool) {
	if db == nil {
		return Config{}, false
	}
	value, ok := db.ExtensionState(extensionName)
	if !ok {
		return Config{}, false
	}
	config, ok := value.(Config)
	return config, ok
}

func (config Config) field() string {
	if config.FieldName != "" {
		return config.FieldName
	}
	return defaultFieldName
}

type contextKey struct{}

type state struct {
	locale   string
	fallback string
}

func stateFromContext(ctx context.Context) (state, bool) {
	value, ok := ctx.Value(contextKey{}).(state)
	return value, ok
}

func translationsFromModel(model any, field string) (Values, error) {
	value, ok := modelFieldValue(model, field)
	if !ok || value == nil {
		return nil, nil
	}
	return normalizeTranslations(value)
}

func applyTranslations(model any, translations Values, locale string, fallback string, fields []string) error {
	if locale == "" {
		return nil
	}
	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() || modelValue.Kind() != reflect.Pointer || modelValue.IsNil() {
		return &oro.Error{Op: "translation", Kind: oro.ErrInvalidArgument}
	}
	structValue := modelValue.Elem()
	if structValue.Kind() != reflect.Struct {
		return &oro.Error{Op: "translation", Kind: oro.ErrInvalidArgument}
	}
	for _, field := range reflect.VisibleFields(structValue.Type()) {
		if !field.IsExported() || field.Anonymous || !isConfiguredField(fields, field.Name) {
			continue
		}
		if value, ok := translatedValue(translations, locale, field.Name); ok {
			if err := assignFieldValue(structValue.FieldByIndex(field.Index), value); err != nil {
				return err
			}
			continue
		}
		if fallback != "" {
			if value, ok := translatedValue(translations, fallback, field.Name); ok {
				if err := assignFieldValue(structValue.FieldByIndex(field.Index), value); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func translatedValue(values Values, locale string, field string) (any, bool) {
	if locale == "" || len(values) == 0 {
		return nil, false
	}
	row, ok := values[locale]
	if !ok || row == nil {
		return nil, false
	}
	value, ok := row[field]
	if !ok || value == nil {
		return nil, false
	}
	return value, true
}

func firstLocale(values Values) string {
	for locale := range values {
		return locale
	}
	return ""
}

func isZeroAny(value any) bool {
	if value == nil {
		return true
	}
	reflectValue := reflect.ValueOf(value)
	return !reflectValue.IsValid() || reflectValue.IsZero()
}

func isConfiguredField(fields []string, field string) bool {
	for _, item := range fields {
		if item == field {
			return true
		}
	}
	return false
}

func normalizeTranslations(value any) (Values, error) {
	switch typed := value.(type) {
	case Values:
		return typed, nil
	case oro.Map:
		return valuesFromMap(typed), nil
	case oro.JSONRaw:
		return valuesFromJSON([]byte(typed))
	case []byte:
		return valuesFromJSON(typed)
	case string:
		return valuesFromJSON([]byte(typed))
	default:
		return nil, nil
	}
}

func valuesFromJSON(payload []byte) (Values, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	values := Values{}
	if err := json.Unmarshal(payload, &values); err != nil {
		return nil, err
	}
	return values, nil
}

func valuesFromMap(values oro.Map) Values {
	out := Values{}
	for locale, raw := range values {
		switch typed := raw.(type) {
		case oro.Map:
			out[locale] = typed
		case map[string]any:
			out[locale] = oro.Map(typed)
		}
	}
	return out
}

func setTranslations(model any, field string, values Values) error {
	payload, err := json.Marshal(values)
	if err != nil {
		return err
	}
	return setModelFieldValue(model, field, oro.JSONRaw(payload))
}

func mustMarshalRaw(values Values) oro.JSONRaw {
	payload, _ := json.Marshal(values)
	return oro.JSONRaw(payload)
}

func mergeTranslations(target Values, source Values) {
	for locale, row := range source {
		if target[locale] == nil {
			target[locale] = oro.Map{}
		}
		for field, value := range row {
			target[locale][field] = value
		}
	}
}

func modelFieldValue(model any, field string) (any, bool) {
	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() {
		return nil, false
	}
	for modelValue.Kind() == reflect.Pointer {
		if modelValue.IsNil() {
			return nil, false
		}
		modelValue = modelValue.Elem()
	}
	if modelValue.Kind() != reflect.Struct {
		return nil, false
	}
	fieldValue := modelValue.FieldByName(field)
	if !fieldValue.IsValid() || !fieldValue.CanInterface() {
		return nil, false
	}
	return fieldValue.Interface(), true
}

func setModelFieldValue(model any, field string, value any) error {
	modelValue := reflect.ValueOf(model)
	if !modelValue.IsValid() || modelValue.Kind() != reflect.Pointer || modelValue.IsNil() {
		return &oro.Error{Op: "translation", Kind: oro.ErrInvalidArgument}
	}
	structValue := modelValue.Elem()
	fieldValue := structValue.FieldByName(field)
	return assignFieldValue(fieldValue, value)
}

func assignFieldValue(field reflect.Value, value any) error {
	if !field.IsValid() || !field.CanSet() || value == nil {
		return nil
	}
	if raw, ok := value.(json.Number); ok {
		value = raw.String()
	}
	valueReflect := reflect.ValueOf(value)
	if valueReflect.IsValid() && valueReflect.Type().AssignableTo(field.Type()) {
		field.Set(valueReflect)
		return nil
	}
	if field.Kind() == reflect.String {
		field.SetString(toString(value))
		return nil
	}
	if valueReflect.IsValid() && valueReflect.Type().ConvertibleTo(field.Type()) {
		field.Set(valueReflect.Convert(field.Type()))
	}
	return nil
}

func toString(value any) string {
	if value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func copyMap(values oro.Map) oro.Map {
	if len(values) == 0 {
		return oro.Map{}
	}
	out := make(oro.Map, len(values))
	for key, value := range values {
		out[key] = value
	}
	return out
}

func isBaseField(field string) bool {
	switch field {
	case "ID", "CreatedAt", "UpdatedAt", "DeletedAt", defaultFieldName:
		return true
	default:
		return false
	}
}
