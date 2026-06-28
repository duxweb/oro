package translation

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/duxweb/oro"
	"github.com/duxweb/oro/internal/reflectx"
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
	return &Query[T]{query: db.Use[T](), config: dbConfig(db, options...)}
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

func LocaleFromContext(ctx context.Context) (string, bool) {
	state, ok := stateFromContext(ctx)
	return state.locale, ok && state.locale != ""
}

type Apply struct {
	config   Config
	locale   string
	fallback string
	values   Values
	where    []transWhere
}

type transWhere struct {
	field string
	op    string
	value any
}

type applyState struct {
	config   Config
	locale   string
	fallback string
	values   Values
	where    []transWhere
	done     map[oro.ApplyStage]bool
}

const applyStateKey = "translation"

func Configured(options ...Option) Apply {
	return Apply{config: resolveConfig(options)}
}

func Locale(locale string) Apply {
	return Apply{locale: locale}
}

func Fallback(locale string) Apply {
	return Apply{fallback: locale}
}

func Write(values Values) Apply {
	return Apply{values: values}
}

func WhereTrans(field string, value any) Apply {
	return Apply{where: []transWhere{{field: field, op: "=", value: value}}}
}

func WhereTransLike(field string, value any) Apply {
	return Apply{where: []transWhere{{field: field, op: "like", value: value}}}
}

func (apply Apply) ApplyOro(ctx *oro.ApplyContext) error {
	if ctx == nil {
		return &oro.Error{Op: "translation.apply", Kind: oro.ErrInvalidArgument}
	}
	state := translationApplyState(ctx)
	state.config = mergeConfig(state.config, apply.config)
	if apply.locale != "" {
		state.locale = apply.locale
	}
	if apply.fallback != "" {
		state.fallback = apply.fallback
	}
	if len(apply.values) > 0 {
		if state.values == nil {
			state.values = Values{}
		}
		mergeTranslations(state.values, apply.values)
	}
	state.where = append(state.where, apply.where...)
	return nil
}

func (apply Apply) AfterApplyOro(ctx *oro.ApplyContext) error {
	if ctx == nil {
		return &oro.Error{Op: "translation.apply", Kind: oro.ErrInvalidArgument}
	}
	state := translationApplyState(ctx)
	if state.done[ctx.Stage] {
		return nil
	}
	state.done[ctx.Stage] = true
	switch ctx.Stage {
	case oro.ApplyStageSpec:
		return apply.applySpec(ctx, state)
	case oro.ApplyStageValues:
		return apply.applyValues(ctx, state)
	case oro.ApplyStageResult:
		return apply.applyResult(ctx, state)
	default:
		return nil
	}
}

func (apply Apply) applySpec(ctx *oro.ApplyContext, state *applyState) error {
	if ctx.Mode != oro.ApplyRead && ctx.Mode != oro.ApplyUpdate && ctx.Mode != oro.ApplyDelete && ctx.Mode != oro.ApplyRestore {
		return nil
	}
	if len(state.where) == 0 && ctx.Mode != oro.ApplyRead {
		return nil
	}
	if ctx.Mode == oro.ApplyRead {
		ctx.SelectHidden(state.config.field())
	}
	locale := state.effectiveLocale(ctx.Context)
	for _, condition := range state.where {
		if !isTranslatedField(state.config, condition.field) {
			return &oro.Error{Op: "translation.where", Kind: oro.ErrUnknownField, Field: condition.field}
		}
		if locale == "" {
			return &oro.Error{Op: "translation.where", Kind: oro.ErrInvalidArgument, Field: condition.field}
		}
		jsonPath := oro.JSON(state.config.field()).Path(locale, condition.field)
		if condition.op == "like" {
			if err := ctx.Where(jsonPath.Like(condition.value)); err != nil {
				return err
			}
			continue
		}
		if err := ctx.Where(jsonPath.Eq(condition.value)); err != nil {
			return err
		}
	}
	return nil
}

func (apply Apply) applyValues(ctx *oro.ApplyContext, state *applyState) error {
	switch ctx.Mode {
	case oro.ApplyInsert:
		if ctx.Model == nil {
			return nil
		}
		return prepareModelForWrite(ctx.Context, ctx.Model, state.config, state.effectiveLocale(ctx.Context), state.values)
	case oro.ApplyUpdate:
		mapped, hasTranslations, err := prepareMapForUpdate(ctx, state.config, state.effectiveLocale(ctx.Context), state.values)
		if err != nil {
			return err
		}
		if !hasTranslations && len(state.values) == 0 {
			return nil
		}
		for key := range ctx.Values {
			delete(ctx.Values, key)
		}
		for key, value := range mapped {
			ctx.Values[key] = value
		}
	}
	return nil
}

func (apply Apply) applyResult(ctx *oro.ApplyContext, state *applyState) error {
	if ctx.Mode != oro.ApplyAfterFind || ctx.Model == nil {
		return nil
	}
	translations, err := translationsFromModel(ctx.Model, state.config.field())
	if err != nil || len(translations) == 0 {
		return err
	}
	return applyTranslations(ctx.Model, translations, state.effectiveLocale(ctx.Context), state.effectiveFallback(ctx.Context), state.config.Fields)
}

func translationApplyState(ctx *oro.ApplyContext) *applyState {
	if ctx.State == nil {
		ctx.State = oro.Map{}
	}
	if state, ok := ctx.State[applyStateKey].(*applyState); ok {
		return state
	}
	state := &applyState{config: dbConfig(ctx.DB), done: map[oro.ApplyStage]bool{}}
	ctx.State[applyStateKey] = state
	return state
}

func (state *applyState) effectiveLocale(ctx context.Context) string {
	if state.locale != "" {
		return state.locale
	}
	if contextState, ok := stateFromContext(ctx); ok && contextState.locale != "" {
		return contextState.locale
	}
	return state.config.DefaultLocale
}

func (state *applyState) effectiveFallback(ctx context.Context) string {
	if state.fallback != "" {
		return state.fallback
	}
	if contextState, ok := stateFromContext(ctx); ok && contextState.fallback != "" {
		return contextState.fallback
	}
	return state.config.FallbackLocale
}

type Query[T any] struct {
	query    *oro.ModelQuery[T]
	config   Config
	locale   string
	fallback string
}

func (query *Query[T]) apply() Apply {
	return Apply{config: query.config, locale: query.locale, fallback: query.fallback}
}

func (query *Query[T]) base() *oro.ModelQuery[T] {
	return query.query.Apply(query.apply())
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
	return query.clone(query.query.Where(field, args...))
}

func (query *Query[T]) OrWhere(field any, args ...any) *Query[T] {
	return query.clone(query.query.OrWhere(field, args...))
}

func (query *Query[T]) WhereGroup(fn func(w *oro.WhereBuilder)) *Query[T] {
	return query.clone(query.query.WhereGroup(fn))
}

func (query *Query[T]) OrWhereGroup(fn func(w *oro.WhereBuilder)) *Query[T] {
	return query.clone(query.query.OrWhereGroup(fn))
}

func (query *Query[T]) WhereWhen(condition bool, fn func(w *oro.WhereBuilder)) *Query[T] {
	return query.clone(query.query.WhereWhen(condition, fn))
}

func (query *Query[T]) WhereRaw(sql string, args ...any) *Query[T] {
	return query.clone(query.query.WhereRaw(sql, args...))
}

func (query *Query[T]) WhereTrans(field string, value any) *Query[T] {
	return query.clone(query.query.Apply(WhereTrans(field, value)))
}

func (query *Query[T]) WhereTransLike(field string, value any) *Query[T] {
	return query.clone(query.query.Apply(WhereTransLike(field, value)))
}

func (query *Query[T]) OrderBy(fields ...string) *Query[T] {
	return query.clone(query.query.OrderBy(fields...))
}

func (query *Query[T]) OrderByDesc(fields ...string) *Query[T] {
	return query.clone(query.query.OrderByDesc(fields...))
}

func (query *Query[T]) Limit(limit int) *Query[T] {
	return query.clone(query.query.Limit(limit))
}

func (query *Query[T]) Offset(offset int) *Query[T] {
	return query.clone(query.query.Offset(offset))
}

func (query *Query[T]) With(relation any, callbacks ...func(*oro.RelationQuery)) *Query[T] {
	return query.clone(query.query.With(relation, callbacks...))
}

func (query *Query[T]) First(ctx context.Context) (*T, error) {
	return query.base().First(ctx)
}

func (query *Query[T]) Find(ctx context.Context, id any) (*T, error) {
	return query.base().Find(ctx, id)
}

func (query *Query[T]) Get(ctx context.Context) ([]*T, error) {
	return query.base().Get(ctx)
}

func (query *Query[T]) Count(ctx context.Context) (int64, error) {
	return query.base().Count(ctx)
}

func (query *Query[T]) Exists(ctx context.Context) (bool, error) {
	return query.base().Exists(ctx)
}

func (query *Query[T]) Create(ctx context.Context, model *T, values ...Values) (*T, error) {
	applies := []oro.Apply{query.apply()}
	if len(values) > 0 {
		applies = append(applies, Write(values[0]))
	}
	return query.query.Apply(applies...).Create(ctx, model)
}

func (query *Query[T]) Update(ctx context.Context, values oro.Map, transValues ...Values) (int64, error) {
	applies := []oro.Apply{query.apply()}
	if len(transValues) > 0 {
		applies = append(applies, Write(transValues[0]))
	}
	return query.query.Apply(applies...).Update(ctx, values)
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

func dbConfig(db *oro.DB, options ...Option) Config {
	config := resolveConfig(options)
	if configured, ok := configFromDB(db); ok {
		config = mergeConfig(configured, config)
	}
	return config
}

func prepareModelForWrite(ctx context.Context, model any, config Config, locale string, values Values) error {
	translations, err := translationsFromModel(model, config.field())
	if err != nil {
		return err
	}
	if translations == nil {
		translations = Values{}
	}
	if len(values) > 0 {
		filtered := filterValues(config, values)
		mergeTranslations(translations, filtered)
		if err := applyOriginalFallbackForCreate(ctx, model, config, locale, filtered); err != nil {
			return err
		}
	} else {
		row, err := translationValuesFromModel(model, config)
		if err != nil {
			return err
		}
		if len(row) > 0 {
			if locale == "" {
				return &oro.Error{Op: "translation.create", Kind: oro.ErrInvalidArgument, Field: "locale"}
			}
			translations[locale] = row
		}
	}
	return setTranslations(model, config.field(), translations)
}

func applyOriginalFallbackForCreate(ctx context.Context, model any, config Config, locale string, values Values) error {
	if locale == "" {
		locale = config.DefaultLocale
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

func prepareMapForUpdate(ctx *oro.ApplyContext, config Config, locale string, transValues Values) (oro.Map, bool, error) {
	mapped := copyMap(ctx.Values)
	if len(mapped) == 0 && len(transValues) == 0 {
		return mapped, false, nil
	}
	row := oro.Map{}
	for key, value := range mapped {
		if isTranslatedField(config, key) {
			row[key] = value
			delete(mapped, key)
		}
	}
	translations := Values{}
	if len(row) > 0 {
		if locale == "" {
			return nil, false, &oro.Error{Op: "translation.update", Kind: oro.ErrInvalidArgument, Field: "locale"}
		}
		translations[locale] = row
	}
	filtered := filterValues(config, transValues)
	mergeTranslations(translations, filtered)
	if len(translations) == 0 {
		return mapped, false, nil
	}
	if ctx.DB == nil || ctx.Spec == nil {
		return nil, false, &oro.Error{Op: "translation.update", Kind: oro.ErrInvalidArgument}
	}
	if count, err := ctx.CountRows(); err != nil || count != 1 {
		if err != nil {
			return nil, false, err
		}
		return nil, false, &oro.Error{Op: "translation.update", Kind: oro.ErrUnsupported}
	}
	current, err := ctx.FirstRowColumns(config.field())
	if err != nil {
		return nil, false, err
	}
	existing, err := normalizeTranslations(current[config.field()])
	if err != nil {
		return nil, false, err
	}
	if existing == nil {
		existing = Values{}
	}
	mergeTranslations(existing, translations)
	syncOriginalValues(config, mapped, translations)
	mapped[config.field()] = mustMarshalRaw(existing)
	return mapped, true, nil
}

func isTranslatedField(config Config, field string) bool {
	if field == "" || field == config.field() {
		return false
	}
	for _, item := range config.Fields {
		if item == field {
			return true
		}
	}
	return false
}

func filterValues(config Config, values Values) Values {
	if len(values) == 0 {
		return nil
	}
	out := Values{}
	for locale, row := range values {
		for field, value := range row {
			if !isTranslatedField(config, field) {
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

func translationValuesFromModel(model any, config Config) (oro.Map, error) {
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
		if !field.IsExported() || field.Anonymous || !isTranslatedField(config, field.Name) {
			continue
		}
		fieldValue, ok := reflectx.FieldByIndex(structValue, field.Index)
		if !ok {
			continue
		}
		if !fieldValue.IsValid() || !fieldValue.CanInterface() || fieldValue.IsZero() {
			continue
		}
		row[field.Name] = fieldValue.Interface()
	}
	return row, nil
}

func syncOriginalValues(config Config, mapped oro.Map, values Values) {
	locale := config.DefaultLocale
	if locale == "" {
		return
	}
	row := values[locale]
	for field, value := range row {
		if !isTranslatedField(config, field) {
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
			fieldValue, ok := reflectx.FieldByIndexAlloc(structValue, field.Index)
			if !ok {
				continue
			}
			if err := assignFieldValue(fieldValue, value); err != nil {
				return err
			}
			continue
		}
		if fallback != "" {
			if value, ok := translatedValue(translations, fallback, field.Name); ok {
				fieldValue, ok := reflectx.FieldByIndexAlloc(structValue, field.Index)
				if !ok {
					continue
				}
				if err := assignFieldValue(fieldValue, value); err != nil {
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
