package drivererr

import (
	"errors"
	"reflect"
)

func UintField(err error, name string) (uint64, bool) {
	for current := range walk(err) {
		value := reflect.ValueOf(current)
		if !value.IsValid() {
			continue
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				continue
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			continue
		}
		field := value.FieldByName(name)
		if !field.IsValid() {
			continue
		}
		switch field.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return field.Uint(), true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			number := field.Int()
			if number >= 0 {
				return uint64(number), true
			}
		}
	}
	return 0, false
}

func StringField(err error, name string) (string, bool) {
	for current := range walk(err) {
		value := reflect.ValueOf(current)
		if !value.IsValid() {
			continue
		}
		if value.Kind() == reflect.Pointer {
			if value.IsNil() {
				continue
			}
			value = value.Elem()
		}
		if value.Kind() != reflect.Struct {
			continue
		}
		field := value.FieldByName(name)
		if !field.IsValid() || field.Kind() != reflect.String {
			continue
		}
		return field.String(), true
	}
	return "", false
}

func StringMethod(err error, name string) (string, bool) {
	for current := range walk(err) {
		value := reflect.ValueOf(current)
		if !value.IsValid() {
			continue
		}
		method := value.MethodByName(name)
		if !method.IsValid() {
			continue
		}
		methodType := method.Type()
		if methodType.NumIn() != 0 || methodType.NumOut() != 1 || methodType.Out(0).Kind() != reflect.String {
			continue
		}
		return method.Call(nil)[0].String(), true
	}
	return "", false
}

func walk(err error) func(func(error) bool) {
	return func(yield func(error) bool) {
		var visit func(error) bool
		visit = func(current error) bool {
			if current == nil {
				return true
			}
			if !yield(current) {
				return false
			}
			if wrapped := errors.Unwrap(current); wrapped != nil {
				return visit(wrapped)
			}
			if multi, ok := current.(interface{ Unwrap() []error }); ok {
				for _, item := range multi.Unwrap() {
					if !visit(item) {
						return false
					}
				}
			}
			return true
		}
		visit(err)
	}
}
