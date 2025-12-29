package bus

import (
	"context"
	"time"
)

// MetricsRecorder is an interface for recording bus metrics.
// This avoids import cycles with the metrics package.
type MetricsRecorder interface {
	RecordBusPublish(topic string, latencyMs int64, err error)
}

// InstrumentedBus wraps a Bus implementation with metrics instrumentation.
type InstrumentedBus struct {
	inner   Bus
	metrics MetricsRecorder
}

// NewInstrumentedBus creates a new instrumented bus that records metrics.
func NewInstrumentedBus(inner Bus, metrics MetricsRecorder) *InstrumentedBus {
	return &InstrumentedBus{
		inner:   inner,
		metrics: metrics,
	}
}

// Publish publishes an event to a topic and records metrics.
func (b *InstrumentedBus) Publish(ctx context.Context, topic string, event Event) error {
	start := time.Now()
	err := b.inner.Publish(ctx, topic, event)
	latencyMs := time.Since(start).Milliseconds()

	if b.metrics != nil {
		b.metrics.RecordBusPublish(topic, latencyMs, err)
	}

	return err
}

// Subscribe subscribes to events on a topic.
func (b *InstrumentedBus) Subscribe(ctx context.Context, topic string, handler Handler) error {
	return b.inner.Subscribe(ctx, topic, handler)
}

// Request sends a request and waits for a response, recording metrics.
func (b *InstrumentedBus) Request(ctx context.Context, topic string, req Event) (Event, error) {
	start := time.Now()
	resp, err := b.inner.Request(ctx, topic, req)
	latencyMs := time.Since(start).Milliseconds()

	// Record as publish for the request
	if b.metrics != nil {
		b.metrics.RecordBusPublish(topic, latencyMs, err)
	}

	return resp, err
}

// Close closes the underlying bus.
func (b *InstrumentedBus) Close() error {
	return b.inner.Close()
}
