package shardstrategy

import (
	"context"
	"fmt"
	"hash/fnv"

	internaltypes "github.com/duxweb/oro/internal/types"
)

type Strategy interface {
	Pick(ctx context.Context, values internaltypes.Map, shards []string) (string, error)
}

type Func func(ctx context.Context, values internaltypes.Map, shards []string) (string, error)

func (fn Func) Pick(ctx context.Context, values internaltypes.Map, shards []string) (string, error) {
	return fn(ctx, values, shards)
}

type Range struct {
	Min        int64
	Max        int64
	Connection string
}

type ErrorSet struct {
	Required error
	NotFound error
}

type Mod struct {
	Field  string
	Errors ErrorSet
}

func (strategy Mod) Pick(ctx context.Context, values internaltypes.Map, shards []string) (string, error) {
	value, ok := values[strategy.Field]
	if !ok || value == nil {
		return "", strategy.Errors.Required
	}
	if len(shards) == 0 {
		return "", strategy.Errors.NotFound
	}
	integer, err := Uint(value, strategy.Errors.NotFound)
	if err != nil {
		return "", err
	}
	return shards[int(integer%uint64(len(shards)))], nil
}

type Hash struct {
	Field  string
	Errors ErrorSet
}

func (strategy Hash) Pick(ctx context.Context, values internaltypes.Map, shards []string) (string, error) {
	value, ok := values[strategy.Field]
	if !ok || value == nil {
		return "", strategy.Errors.Required
	}
	if len(shards) == 0 {
		return "", strategy.Errors.NotFound
	}
	hasher := fnv.New64a()
	_, _ = hasher.Write([]byte(fmt.Sprint(value)))
	return shards[int(hasher.Sum64()%uint64(len(shards)))], nil
}

type RangeStrategy struct {
	Field  string
	Ranges []Range
	Errors ErrorSet
}

func (strategy RangeStrategy) Pick(ctx context.Context, values internaltypes.Map, shards []string) (string, error) {
	value, ok := values[strategy.Field]
	if !ok || value == nil {
		return "", strategy.Errors.Required
	}
	integer, err := Int(value, strategy.Errors.NotFound)
	if err != nil {
		return "", err
	}
	allowed := map[string]bool{}
	for _, shard := range shards {
		allowed[shard] = true
	}
	for _, item := range strategy.Ranges {
		if integer < item.Min || integer > item.Max {
			continue
		}
		if !allowed[item.Connection] {
			return "", strategy.Errors.NotFound
		}
		return item.Connection, nil
	}
	return "", strategy.Errors.NotFound
}

func Uint(value any, notFound error) (uint64, error) {
	switch typed := value.(type) {
	case uint:
		return uint64(typed), nil
	case uint8:
		return uint64(typed), nil
	case uint16:
		return uint64(typed), nil
	case uint32:
		return uint64(typed), nil
	case uint64:
		return typed, nil
	case int:
		if typed < 0 {
			return 0, notFound
		}
		return uint64(typed), nil
	case int8:
		if typed < 0 {
			return 0, notFound
		}
		return uint64(typed), nil
	case int16:
		if typed < 0 {
			return 0, notFound
		}
		return uint64(typed), nil
	case int32:
		if typed < 0 {
			return 0, notFound
		}
		return uint64(typed), nil
	case int64:
		if typed < 0 {
			return 0, notFound
		}
		return uint64(typed), nil
	}
	return 0, notFound
}

func Int(value any, notFound error) (int64, error) {
	switch typed := value.(type) {
	case int:
		return int64(typed), nil
	case int8:
		return int64(typed), nil
	case int16:
		return int64(typed), nil
	case int32:
		return int64(typed), nil
	case int64:
		return typed, nil
	case uint:
		if uint64(typed) > uint64(^uint64(0)>>1) {
			return 0, notFound
		}
		return int64(typed), nil
	case uint8:
		return int64(typed), nil
	case uint16:
		return int64(typed), nil
	case uint32:
		return int64(typed), nil
	case uint64:
		if typed > uint64(^uint64(0)>>1) {
			return 0, notFound
		}
		return int64(typed), nil
	}
	return 0, notFound
}
