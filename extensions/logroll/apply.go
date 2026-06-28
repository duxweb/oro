package logroll

import (
	"sync/atomic"

	"github.com/duxweb/oro"
)

type Apply struct {
	config Config
	writes atomic.Int64
}

func Roll(options ...Option) *Apply {
	return &Apply{config: resolveConfig(options)}
}

func (apply *Apply) ApplyOro(ctx *oro.ApplyContext) error {
	if apply == nil || ctx == nil || ctx.Mode != oro.ApplyAfterWrite || ctx.Stage != oro.ApplyStageResult {
		return nil
	}
	if len(apply.config.Policies) == 0 || ctx.DB == nil || ctx.Schema == nil {
		return nil
	}
	if apply.config.Every > 1 && apply.writes.Add(1)%apply.config.Every != 0 {
		return nil
	}
	_, err := cleanupWithSchema(ctx.Context, ctx.DB, ctx.Schema, apply.config)
	return err
}
