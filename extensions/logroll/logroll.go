package logroll

import (
	"time"

	"github.com/duxweb/oro"
)

const extensionName = "logroll"

type TimeModel struct {
	OccurredAt time.Time
}

func (TimeModel) OroEmbeddedFields() {}

func (TimeModel) DefineOroFields(s *oro.SchemaBuilder) {
	DefineTime(s)
}

func DefineTime(s *oro.SchemaBuilder, options ...Option) {
	config := resolveOptions(options)
	s.Field(config.TimeField).Column(oro.Snake(config.TimeField)).Timestamp().Index()
}

type Config struct {
	Policies   []policy
	TimeField  string
	BatchSize  int
	Every      int64
	Now        func() time.Time
	Scope      oro.Map
	Connection string
}

type Option interface {
	applyLogRollOption(*Config)
}

type optionFunc func(*Config)

func (fn optionFunc) applyLogRollOption(config *Config) {
	fn(config)
}

type policy interface {
	logRollPolicy()
}

func KeepLast(count int64) Option {
	return keepLastPolicy{count: count}
}

func KeepFor(duration time.Duration) Option {
	return keepForPolicy{duration: duration}
}

func TimeField(field string) Option {
	return optionFunc(func(config *Config) {
		config.TimeField = field
	})
}

func BatchSize(size int) Option {
	return optionFunc(func(config *Config) {
		config.BatchSize = size
	})
}

func Every(count int64) Option {
	return optionFunc(func(config *Config) {
		config.Every = count
	})
}

func Now(fn func() time.Time) Option {
	return optionFunc(func(config *Config) {
		config.Now = fn
	})
}

func Scope(values oro.Map) Option {
	return optionFunc(func(config *Config) {
		config.Scope = copyMap(values)
	})
}

func Connection(name string) Option {
	return optionFunc(func(config *Config) {
		config.Connection = name
	})
}

type extension struct{}

func Extension(options ...Option) oro.Extension {
	return extension{}
}

func (extension) Name() string {
	return extensionName
}

func (extension) Install(db *oro.DB) error {
	return nil
}

type keepLastPolicy struct {
	count int64
}

func (policy keepLastPolicy) logRollPolicy() {}

func (policy keepLastPolicy) applyLogRollOption(config *Config) {
	config.Policies = append(config.Policies, policy)
}

type keepForPolicy struct {
	duration time.Duration
}

func (policy keepForPolicy) logRollPolicy() {}

func (policy keepForPolicy) applyLogRollOption(config *Config) {
	config.Policies = append(config.Policies, policy)
}

func resolveOptions(items []Option) Config {
	config := Config{
		TimeField: "CreatedAt",
		BatchSize: 1000,
		Every:     1,
		Now:       time.Now,
	}
	for _, item := range items {
		if item != nil {
			item.applyLogRollOption(&config)
		}
	}
	if config.TimeField == "" {
		config.TimeField = "CreatedAt"
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 1000
	}
	if config.Every <= 0 {
		config.Every = 1
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return config
}

func resolveConfig(options []Option) Config {
	config := resolveOptions(options)
	return config
}

func copyMap(values oro.Map) oro.Map {
	if len(values) == 0 {
		return nil
	}
	cloned := make(oro.Map, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
