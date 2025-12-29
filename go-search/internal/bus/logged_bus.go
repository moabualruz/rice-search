package bus

import (
	"context"
)

// LoggedBus wraps another Bus implementation and logs all events to disk.
// This is useful for debugging and replay scenarios.
type LoggedBus struct {
	inner  Bus
	logger *EventLogger
}

// NewLoggedBus creates a new logged bus that wraps an inner bus.
// Events are logged before being published to the inner bus.
func NewLoggedBus(inner Bus, logger *EventLogger) *LoggedBus {
	return &LoggedBus{
		inner:  inner,
		logger: logger,
	}
}

// Publish logs the event and then delegates to the inner bus.
func (b *LoggedBus) Publish(ctx context.Context, topic string, event Event) error {
	// Log the event (non-blocking, best-effort)
	if err := b.logger.Log(topic, event); err != nil {
		// Log error but don't fail the publish
		// TODO: Add structured logging here
	}

	// Delegate to inner bus
	return b.inner.Publish(ctx, topic, event)
}

// Subscribe delegates to the inner bus.
func (b *LoggedBus) Subscribe(ctx context.Context, topic string, handler Handler) error {
	return b.inner.Subscribe(ctx, topic, handler)
}

// Request logs the request event and delegates to the inner bus.
func (b *LoggedBus) Request(ctx context.Context, topic string, req Event) (Event, error) {
	// Log the request
	if err := b.logger.Log(topic, req); err != nil {
		// Log error but don't fail the request
	}

	// Delegate to inner bus
	resp, err := b.inner.Request(ctx, topic, req)

	// Log the response if successful
	if err == nil {
		if logErr := b.logger.Log(topic+".response", resp); logErr != nil {
			// Log error but don't fail the response
		}
	}

	return resp, err
}

// Close closes both the logger and the inner bus.
func (b *LoggedBus) Close() error {
	// Close logger first
	if err := b.logger.Close(); err != nil {
		// Continue to close inner bus even if logger close fails
	}

	// Close inner bus
	return b.inner.Close()
}
