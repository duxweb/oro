package oro

import "context"

type Hook struct {
	DB           *DB
	Operation    string
	Values       Map
	RowsAffected int64
	SoftDelete   bool
}

type beforeCreateHook interface {
	BeforeCreate(context.Context, *Hook) error
}

type afterCreateHook interface {
	AfterCreate(context.Context, *Hook) error
}

type beforeUpdateHook interface {
	BeforeUpdate(context.Context, *Hook) error
}

type afterUpdateHook interface {
	AfterUpdate(context.Context, *Hook) error
}

type beforeDeleteHook interface {
	BeforeDelete(context.Context, *Hook) error
}

type afterDeleteHook interface {
	AfterDelete(context.Context, *Hook) error
}

type beforeRestoreHook interface {
	BeforeRestore(context.Context, *Hook) error
}

type afterRestoreHook interface {
	AfterRestore(context.Context, *Hook) error
}

type afterFindHook interface {
	AfterFind(context.Context, *Hook) error
}

func callBeforeCreate(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(beforeCreateHook); ok {
		if err := handler.BeforeCreate(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func hasCreateHooks(model any) bool {
	if model == nil {
		return false
	}
	_, before := model.(beforeCreateHook)
	_, after := model.(afterCreateHook)
	return before || after
}

func callAfterCreate(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(afterCreateHook); ok {
		if err := handler.AfterCreate(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func callBeforeUpdate(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(beforeUpdateHook); ok {
		if err := handler.BeforeUpdate(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func callAfterUpdate(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(afterUpdateHook); ok {
		if err := handler.AfterUpdate(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func callBeforeDelete(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(beforeDeleteHook); ok {
		if err := handler.BeforeDelete(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func callAfterDelete(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(afterDeleteHook); ok {
		if err := handler.AfterDelete(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func callBeforeRestore(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(beforeRestoreHook); ok {
		if err := handler.BeforeRestore(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func callAfterRestore(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(afterRestoreHook); ok {
		if err := handler.AfterRestore(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func callAfterFind(ctx context.Context, model any, hook *Hook) error {
	if handler, ok := model.(afterFindHook); ok {
		if err := handler.AfterFind(ctx, hook); err != nil {
			return wrapHookError(err)
		}
	}
	return nil
}

func wrapHookError(err error) error {
	if err == nil {
		return nil
	}
	return &Error{Op: "hook", Kind: ErrHook, Cause: err}
}
