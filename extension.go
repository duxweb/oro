package oro

import "context"

type Extension interface {
	Name() string
	Install(db *DB) error
}

type QueryExtension interface {
	Extension
	ApplyQuery(ctx context.Context, db *DB, spec *QuerySpec) error
}

type WriteExtension interface {
	Extension
	ApplyWrite(ctx context.Context, db *DB, spec *WriteSpec) error
}

type ConnectionExtension interface {
	Extension
	ApplyConnection(ctx context.Context, db *DB, spec *QuerySpec) error
}

type ShardValueExtension interface {
	Extension
	ShardValues(ctx context.Context, db *DB) (Map, bool, error)
}

type CacheKeyExtension interface {
	Extension
	CacheKeyValues(ctx context.Context, db *DB) (Map, bool, error)
}

type ExtensionFunc struct {
	ExtensionName string
	Fn            func(db *DB) error
}

func (extension ExtensionFunc) Name() string {
	return extension.ExtensionName
}

func (extension ExtensionFunc) Install(db *DB) error {
	if extension.Fn == nil {
		return nil
	}
	return extension.Fn(db)
}

func installExtensions(db *DB, extensions []Extension) error {
	installed := make(map[string]struct{}, len(extensions))
	for _, extension := range extensions {
		if extension == nil {
			return &Error{Op: "extension.install", Kind: ErrInvalidArgument}
		}
		name := extension.Name()
		if name == "" {
			return &Error{Op: "extension.install", Kind: ErrInvalidArgument}
		}
		if _, exists := installed[name]; exists {
			return &Error{Op: "extension.install", Kind: ErrConflict, Field: name}
		}
		if err := extension.Install(db); err != nil {
			return &Error{Op: "extension.install", Kind: ErrHook, Field: name, Cause: err}
		}
		installed[name] = struct{}{}
	}
	return nil
}

func cloneExtensionState(values map[string]any) map[string]any {
	cloned := make(map[string]any, len(values)+1)
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func applyConnectionExtensions(ctx context.Context, db *DB, spec *QuerySpec) error {
	if db == nil || db.runtime == nil {
		return nil
	}
	for _, extension := range db.runtime.Config.Extensions {
		hook, ok := extension.(ConnectionExtension)
		if !ok {
			continue
		}
		if err := hook.ApplyConnection(ctx, db, spec); err != nil {
			return err
		}
	}
	return nil
}

func applyQueryExtensions(ctx context.Context, db *DB, spec *QuerySpec) error {
	if db == nil || db.runtime == nil {
		return nil
	}
	for _, extension := range db.runtime.Config.Extensions {
		hook, ok := extension.(QueryExtension)
		if !ok {
			continue
		}
		if err := hook.ApplyQuery(ctx, db, spec); err != nil {
			return err
		}
	}
	return nil
}

func applyWriteExtensions(ctx context.Context, db *DB, spec *WriteSpec) error {
	if db == nil || db.runtime == nil {
		return nil
	}
	for _, extension := range db.runtime.Config.Extensions {
		hook, ok := extension.(WriteExtension)
		if !ok {
			continue
		}
		if err := hook.ApplyWrite(ctx, db, spec); err != nil {
			return err
		}
	}
	return nil
}

func hasWriteExtensions(db *DB) bool {
	if db == nil || db.runtime == nil {
		return false
	}
	for _, extension := range db.runtime.Config.Extensions {
		if _, ok := extension.(WriteExtension); ok {
			return true
		}
	}
	return false
}

func extensionShardValues(ctx context.Context, db *DB) (Map, error) {
	if db == nil || db.runtime == nil {
		return nil, nil
	}
	values := Map{}
	for _, extension := range db.runtime.Config.Extensions {
		hook, ok := extension.(ShardValueExtension)
		if !ok {
			continue
		}
		next, ok, err := hook.ShardValues(ctx, db)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		for key, value := range next {
			values[key] = value
		}
	}
	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
}

func extensionCacheKeyValues(ctx context.Context, db *DB) (Map, error) {
	if db == nil || db.runtime == nil {
		return nil, nil
	}
	values := Map{}
	for _, extension := range db.runtime.Config.Extensions {
		hook, ok := extension.(CacheKeyExtension)
		if !ok {
			continue
		}
		next, ok, err := hook.CacheKeyValues(ctx, db)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		values[extension.Name()] = next
	}
	if len(values) == 0 {
		return nil, nil
	}
	return values, nil
}
