package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/duxweb/oro"
)

const extensionName = "audit"

type AuditFields struct {
	CreatedBy oro.Null[uint64]
	UpdatedBy oro.Null[uint64]
	DeletedBy oro.Null[uint64]
}

func (AuditFields) OroEmbeddedFields() {}

func (AuditFields) DefineOroFields(s *oro.SchemaBuilder) {
	Define(s)
}

func Define(s *oro.SchemaBuilder) {
	s.Field("CreatedBy").Column("created_by").UnsignedBigInt().Nullable().Index()
	s.Field("UpdatedBy").Column("updated_by").UnsignedBigInt().Nullable().Index()
	s.Field("DeletedBy").Column("deleted_by").UnsignedBigInt().Nullable().Index()
}

type Log struct {
	oro.Model
	ActorID   oro.Null[uint64]
	ModelName string
	Table     string
	Operation string
	Rows      int64
	Values    oro.JSONRaw
}

func (Log) Define(s *oro.SchemaBuilder) {
	s.Table("oro_audit_logs")
	s.Field("ActorID").Column("actor_id").UnsignedBigInt().Nullable().Index()
	s.Field("ModelName").Column("model_name").String().Index()
	s.Field("Table").String().Index()
	s.Field("Operation").String().Index()
	s.Field("Rows").BigInt()
	s.Field("Values").JSON().Nullable()
}

type ActorResolver interface {
	ResolveActor(ctx context.Context) (uint64, bool, error)
}

type ActorResolverFunc func(ctx context.Context) (uint64, bool, error)

func (fn ActorResolverFunc) ResolveActor(ctx context.Context) (uint64, bool, error) {
	return fn(ctx)
}

type LogWriter interface {
	WriteAuditLog(ctx context.Context, db *oro.DB, entry Entry) error
}

type LogWriterFunc func(ctx context.Context, db *oro.DB, entry Entry) error

func (fn LogWriterFunc) WriteAuditLog(ctx context.Context, db *oro.DB, entry Entry) error {
	return fn(ctx, db, entry)
}

type Entry struct {
	ActorID   oro.Null[uint64]
	ModelName string
	Table     string
	Operation string
	Rows      int64
	Values    oro.Map
	At        time.Time
}

type Config struct {
	Resolver ActorResolver
	Writer   LogWriter
	Fields   FieldsConfig
	LogModel bool
}

type FieldsConfig struct {
	CreatedBy string
	UpdatedBy string
	DeletedBy string
}

type Option interface {
	applyAuditOption(*Config)
}

type optionFunc func(*Config)

func (fn optionFunc) applyAuditOption(config *Config) {
	fn(config)
}

func WithResolver(resolver ActorResolver) Option {
	return optionFunc(func(config *Config) {
		config.Resolver = resolver
	})
}

func WithWriter(writer LogWriter) Option {
	return optionFunc(func(config *Config) {
		config.Writer = writer
	})
}

func WithDefaultLogModel() Option {
	return optionFunc(func(config *Config) {
		config.Writer = DefaultLogWriter()
		config.LogModel = true
	})
}

func WithFields(fields FieldsConfig) Option {
	return optionFunc(func(config *Config) {
		config.Fields = fields
	})
}

type extension struct {
	config Config
}

func Extension(options ...Option) oro.Extension {
	config := Config{
		Fields: FieldsConfig{
			CreatedBy: "CreatedBy",
			UpdatedBy: "UpdatedBy",
			DeletedBy: "DeletedBy",
		},
	}
	for _, option := range options {
		if option != nil {
			option.applyAuditOption(&config)
		}
	}
	return extension{config: config}
}

func (extension extension) Name() string {
	return extensionName
}

func (extension extension) Install(db *oro.DB) error {
	return nil
}

func (extension extension) ApplyWrite(ctx context.Context, db *oro.DB, spec *oro.WriteSpec) error {
	if db == nil || spec == nil || spec.Model == nil || len(spec.Values) == 0 {
		return nil
	}
	actorID, ok, err := extension.actor(ctx)
	if err != nil || !ok {
		return err
	}
	for rowIndex := range spec.Values {
		switch spec.Operation {
		case "create":
			extension.setIfPresent(spec.Model, spec.Values[rowIndex], extension.config.Fields.CreatedBy, actorID)
			extension.setIfPresent(spec.Model, spec.Values[rowIndex], extension.config.Fields.UpdatedBy, actorID)
		case "update", "upsert", "restore":
			extension.setIfPresent(spec.Model, spec.Values[rowIndex], extension.config.Fields.UpdatedBy, actorID)
		case "delete":
			extension.setIfPresent(spec.Model, spec.Values[rowIndex], extension.config.Fields.DeletedBy, actorID)
			extension.setIfPresent(spec.Model, spec.Values[rowIndex], extension.config.Fields.UpdatedBy, actorID)
		}
	}
	return nil
}

func (extension extension) Events() map[oro.EventName]oro.EventHandler {
	return map[oro.EventName]oro.EventHandler{
		oro.AfterCreate:  extension.handleAfterWrite,
		oro.AfterUpdate:  extension.handleAfterWrite,
		oro.AfterDelete:  extension.handleAfterWrite,
		oro.AfterRestore: extension.handleAfterWrite,
	}
}

func (extension extension) handleAfterWrite(ctx context.Context, event *oro.Event) error {
	if extension.config.Writer == nil || event == nil || (extension.config.LogModel && event.ModelName == "Log" && event.Table == "oro_audit_logs") {
		return nil
	}
	actorID, ok, err := extension.actor(ctx)
	if err != nil {
		return err
	}
	entry := Entry{
		ModelName: event.ModelName,
		Table:     event.Table,
		Operation: event.Operation,
		Rows:      event.RowsAffected,
		Values:    visibleValues(event.Schema, event.Values),
		At:        time.Now(),
	}
	if ok {
		entry.ActorID = oro.NullOf(actorID)
	}
	return extension.config.Writer.WriteAuditLog(ctx, event.DB, entry)
}

func (extension extension) actor(ctx context.Context) (uint64, bool, error) {
	if state, ok := stateFromContext(ctx); ok {
		return state.actorID, true, nil
	}
	if extension.config.Resolver == nil {
		return 0, false, nil
	}
	return extension.config.Resolver.ResolveActor(ctx)
}

func (extension extension) setIfPresent(schema *oro.ModelSchema, values oro.Map, fieldName string, actorID uint64) {
	if fieldName == "" {
		return
	}
	field, ok := schema.FieldByGo[fieldName]
	if !ok {
		return
	}
	values[field.Column] = actorID
}

type contextKey struct{}

type state struct {
	actorID uint64
}

func WithActor(ctx context.Context, actorID uint64) context.Context {
	return context.WithValue(ctx, contextKey{}, state{actorID: actorID})
}

func Actor(ctx context.Context) (uint64, bool) {
	state, ok := stateFromContext(ctx)
	if !ok {
		return 0, false
	}
	return state.actorID, true
}

func stateFromContext(ctx context.Context) (state, bool) {
	value, ok := ctx.Value(contextKey{}).(state)
	return value, ok
}

func DefaultLogWriter() LogWriter {
	return LogWriterFunc(func(ctx context.Context, db *oro.DB, entry Entry) error {
		if db == nil {
			return nil
		}
		payload, err := json.Marshal(entry.Values)
		if err != nil {
			return err
		}
		_, err = db.Use[Log]().SkipHooks().SkipEvents().Create(ctx, &Log{
			ActorID:   entry.ActorID,
			ModelName: entry.ModelName,
			Table:     entry.Table,
			Operation: entry.Operation,
			Rows:      entry.Rows,
			Values:    oro.JSONRaw(payload),
		})
		return err
	})
}

func copyMap(values oro.Map) oro.Map {
	if len(values) == 0 {
		return nil
	}
	copied := make(oro.Map, len(values))
	for key, value := range values {
		copied[key] = value
	}
	return copied
}

func visibleValues(schema *oro.ModelSchema, values oro.Map) oro.Map {
	if len(values) == 0 {
		return nil
	}
	if schema == nil {
		return copyMap(values)
	}
	hiddenColumns := map[string]bool{}
	for _, field := range schema.Fields {
		if field.Hidden {
			hiddenColumns[field.Column] = true
			hiddenColumns[field.Name] = true
		}
	}
	if len(hiddenColumns) == 0 {
		return copyMap(values)
	}
	copied := oro.Map{}
	for key, value := range values {
		if hiddenColumns[key] {
			continue
		}
		copied[key] = value
	}
	return copied
}
