package types

type Map map[string]any

type Null[T any] struct {
	Value T
	Valid bool
}

func NullOf[T any](value T) Null[T] {
	return Null[T]{Value: value, Valid: true}
}

func NullZero[T any]() Null[T] {
	return Null[T]{}
}

func (value Null[T]) IsNull() bool {
	return !value.Valid
}

type Decimal string

type JSONRaw []byte
