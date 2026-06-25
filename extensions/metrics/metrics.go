package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/duxweb/oro"
)

const extensionName = "metrics"

type Recorder interface {
	Record(ctx context.Context, sample Sample)
}

type RecorderFunc func(ctx context.Context, sample Sample)

func (fn RecorderFunc) Record(ctx context.Context, sample Sample) {
	fn(ctx, sample)
}

type Sample struct {
	Event     oro.EventName
	Operation string
	ModelName string
	Table     string
	Rows      int64
	Duration  time.Duration
	Err       error
}

type Config struct {
	Recorder Recorder
}

type Option interface {
	applyMetricsOption(*Config)
}

type optionFunc func(*Config)

func (fn optionFunc) applyMetricsOption(config *Config) {
	fn(config)
}

func WithRecorder(recorder Recorder) Option {
	return optionFunc(func(config *Config) {
		config.Recorder = recorder
	})
}

type extension struct {
	config Config
}

func Extension(options ...Option) oro.Extension {
	config := Config{}
	for _, option := range options {
		if option != nil {
			option.applyMetricsOption(&config)
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

func (extension extension) Events() map[oro.EventName]oro.EventHandler {
	return map[oro.EventName]oro.EventHandler{
		oro.AfterSQL:       extension.handle,
		oro.AfterCacheHit:  extension.handle,
		oro.AfterCacheMiss: extension.handle,
		oro.AfterCommit:    extension.handle,
		oro.AfterRollback:  extension.handle,
	}
}

func (extension extension) handle(ctx context.Context, event *oro.Event) error {
	if extension.config.Recorder == nil || event == nil {
		return nil
	}
	extension.config.Recorder.Record(ctx, Sample{
		Event:     event.Name,
		Operation: event.Operation,
		ModelName: event.ModelName,
		Table:     event.Table,
		Rows:      event.RowsAffected,
		Duration:  event.Duration,
		Err:       event.Err,
	})
	return nil
}

type MemoryRecorder struct {
	mu      sync.RWMutex
	samples []Sample
	counts  sync.Map
}

func NewMemoryRecorder() *MemoryRecorder {
	return &MemoryRecorder{}
}

func (recorder *MemoryRecorder) Record(ctx context.Context, sample Sample) {
	recorder.mu.Lock()
	recorder.samples = append(recorder.samples, sample)
	recorder.mu.Unlock()

	key := sample.Event
	if value, _ := recorder.counts.LoadOrStore(key, &atomic.Int64{}); value != nil {
		value.(*atomic.Int64).Add(1)
	}
}

func (recorder *MemoryRecorder) Samples() []Sample {
	recorder.mu.RLock()
	defer recorder.mu.RUnlock()
	return append([]Sample(nil), recorder.samples...)
}

func (recorder *MemoryRecorder) Count(event oro.EventName) int64 {
	value, ok := recorder.counts.Load(event)
	if !ok {
		return 0
	}
	return value.(*atomic.Int64).Load()
}
