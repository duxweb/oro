package oro

import (
	"encoding/json"
	"reflect"
	"strings"
)

func Serialize(value any, opts ...SerializeOption) any {
	options := applySerializeOptions(opts)
	return reflectSerializer{}.Serialize(value, options)
}

type SerializeOption interface {
	applySerializeOption(*SerializeOptions)
}

type SerializeOptions struct {
	ShowHidden bool
}

type serializeOptionFunc func(*SerializeOptions)

func (fn serializeOptionFunc) applySerializeOption(options *SerializeOptions) {
	fn(options)
}

func ShowHidden() SerializeOption {
	return serializeOptionFunc(func(options *SerializeOptions) {
		options.ShowHidden = true
	})
}

func applySerializeOptions(options []SerializeOption) SerializeOptions {
	resolved := SerializeOptions{}
	for _, option := range options {
		if option != nil {
			option.applySerializeOption(&resolved)
		}
	}
	return resolved
}

type reflectSerializer struct {
	rt *Runtime
}

func (serializer reflectSerializer) Serialize(value any, opts SerializeOptions) any {
	return serializer.serializeValue(reflect.ValueOf(value), opts, map[visitKey]struct{}{})
}

type visitKey struct {
	typ reflect.Type
	ptr uintptr
}

func (serializer reflectSerializer) serializeValue(value reflect.Value, opts SerializeOptions, seen map[visitKey]struct{}) any {
	if !value.IsValid() {
		return nil
	}
	if value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil
		}
		return serializer.serializeValue(value.Elem(), opts, seen)
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		key := visitKey{typ: value.Type(), ptr: value.Pointer()}
		if _, ok := seen[key]; ok {
			return nil
		}
		seen[key] = struct{}{}
		value = value.Elem()
	}

	if value.CanInterface() {
		if raw, ok := value.Interface().(JSONRaw); ok {
			return json.RawMessage(raw)
		}
		if value.Type() == timeType {
			return value.Interface()
		}
	}

	switch value.Kind() {
	case reflect.Struct:
		return serializer.serializeStruct(value, opts, seen)
	case reflect.Slice, reflect.Array:
		items := make([]any, 0, value.Len())
		for index := 0; index < value.Len(); index++ {
			items = append(items, serializer.serializeValue(value.Index(index), opts, seen))
		}
		return items
	case reflect.Map:
		return serializer.serializeMap(value, opts, seen)
	default:
		if value.CanInterface() {
			return value.Interface()
		}
		return nil
	}
}

func (serializer reflectSerializer) serializeStruct(value reflect.Value, opts SerializeOptions, seen map[visitKey]struct{}) any {
	if value.CanInterface() {
		if raw, ok := value.Interface().(JSONRaw); ok {
			return json.RawMessage(raw)
		}
		if nullValue, ok := serializeNull(value.Interface()); ok {
			return nullValue
		}
	}
	schema, _ := serializer.schemaForType(value.Type())
	output := Map{}
	for _, field := range reflect.VisibleFields(value.Type()) {
		if isBaseModelField(field.Name) {
			continue
		}
		if !field.IsExported() || field.Anonymous {
			if field.Anonymous && field.Type == reflect.TypeOf(Model{}) {
				serializer.serializeBaseModel(value.FieldByIndex(field.Index), output, opts, seen)
			} else if field.Anonymous && isFlattenableExtensionStruct(field.Type) {
				serializer.serializeEmbeddedFields(value.FieldByIndex(field.Index), output, opts, seen)
			}
			continue
		}
		name, skip := jsonFieldName(field)
		if skip {
			continue
		}
		if schema != nil {
			if fieldSchema, ok := schema.FieldByGo[field.Name]; ok && fieldSchema.Hidden && !opts.ShowHidden {
				continue
			}
		}
		fieldValue := value.FieldByIndex(field.Index)
		if isOmitEmpty(field) && fieldValue.IsZero() {
			continue
		}
		output[name] = serializer.serializeValue(fieldValue, opts, seen)
	}
	if schema != nil {
		serializer.serializeRelations(value, schema, output, opts, seen)
	}
	return output
}

func (serializer reflectSerializer) serializeBaseModel(value reflect.Value, output Map, opts SerializeOptions, seen map[visitKey]struct{}) {
	if !value.IsValid() || value.Type() != reflect.TypeOf(Model{}) {
		return
	}
	for _, field := range []struct {
		goName string
		json   string
	}{
		{goName: "ID", json: "id"},
		{goName: "CreatedAt", json: "created_at"},
		{goName: "UpdatedAt", json: "updated_at"},
	} {
		fieldValue := value.FieldByName(field.goName)
		if !fieldValue.IsValid() {
			continue
		}
		output[field.json] = serializer.serializeValue(fieldValue, opts, seen)
	}
}

func (serializer reflectSerializer) serializeEmbeddedFields(value reflect.Value, output Map, opts SerializeOptions, seen map[visitKey]struct{}) {
	if !value.IsValid() {
		return
	}
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return
	}
	for _, field := range reflect.VisibleFields(value.Type()) {
		if !field.IsExported() || field.Anonymous {
			continue
		}
		name, skip := jsonFieldName(field)
		if skip {
			continue
		}
		fieldValue := value.FieldByIndex(field.Index)
		if isOmitEmpty(field) && fieldValue.IsZero() {
			continue
		}
		output[name] = serializer.serializeValue(fieldValue, opts, seen)
	}
}

func (serializer reflectSerializer) serializeMap(value reflect.Value, opts SerializeOptions, seen map[visitKey]struct{}) any {
	output := Map{}
	iter := value.MapRange()
	for iter.Next() {
		key := iter.Key()
		if key.Kind() != reflect.String {
			continue
		}
		output[key.String()] = serializer.serializeValue(iter.Value(), opts, seen)
	}
	return output
}

func (serializer reflectSerializer) serializeRelations(value reflect.Value, schema *ModelSchema, output Map, opts SerializeOptions, seen map[visitKey]struct{}) {
	state := serializer.modelState(value)
	if state == nil || state.relations == nil {
		return
	}
	relations := map[string]RelationSchema{}
	for _, relation := range schema.Relations {
		relations[relation.Name] = relation
	}
	for name, loaded := range state.relations {
		if !loaded.loaded {
			continue
		}
		relation, ok := relations[name]
		if !ok {
			relation = RelationSchema{Name: name}
		}
		jsonName := relation.JSONName
		if jsonName == "" {
			jsonName = Snake(name)
		}
		if loaded.one != nil {
			output[jsonName] = serializer.serializeValue(reflect.ValueOf(loaded.one), opts, seen)
			continue
		}
		items := make([]any, 0, len(loaded.many))
		for _, item := range loaded.many {
			items = append(items, serializer.serializeValue(reflect.ValueOf(item), opts, seen))
		}
		if loaded.many != nil {
			output[jsonName] = items
		} else {
			output[jsonName] = nil
		}
	}
}

func (serializer reflectSerializer) modelState(value reflect.Value) *modelState {
	modelField := value.FieldByName("Model")
	if !modelField.IsValid() || !modelField.CanAddr() || modelField.Type() != reflect.TypeOf(Model{}) {
		return nil
	}
	return modelField.Addr().Interface().(*Model).relationState()
}

func (serializer reflectSerializer) schemaForType(typ reflect.Type) (*ModelSchema, bool) {
	for typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if serializer.rt != nil && serializer.rt.Registry != nil {
		if schema, ok := serializer.rt.Registry.GetType(typ); ok {
			return schema, true
		}
	}
	if definerType := reflect.TypeOf((*Definer)(nil)).Elem(); reflect.PointerTo(typ).Implements(definerType) || typ.Implements(definerType) {
		schema, err := schemaParser{}.Parse(reflect.New(typ).Interface())
		if err == nil {
			return schema, true
		}
	}
	return nil, false
}

func jsonFieldName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" {
		return "", true
	}
	if tag != "" {
		name := strings.Split(tag, ",")[0]
		if name != "" {
			return name, false
		}
	}
	return field.Name, false
}

func isOmitEmpty(field reflect.StructField) bool {
	tag := field.Tag.Get("json")
	if tag == "" {
		return false
	}
	for _, part := range strings.Split(tag, ",")[1:] {
		if part == "omitempty" {
			return true
		}
	}
	return false
}

func serializeNull(value any) (any, bool) {
	reflectValue := reflect.ValueOf(value)
	reflectType := reflectValue.Type()
	if !isNullStruct(reflectType) {
		return nil, false
	}
	valid := reflectValue.FieldByName("Valid")
	if !valid.Bool() {
		return nil, true
	}
	return reflectValue.FieldByName("Value").Interface(), true
}
