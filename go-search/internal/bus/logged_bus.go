package bus

import (
	"context"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// LoggedBus wraps another Bus implementation and logs all events to disk.
// This is useful for debugging and replay scenarios.
type LoggedBus struct {
	inner       Bus
	eventLogger *EventLogger
	log         *logger.Logger
}

// NewLoggedBus creates a new logged bus that wraps an inner bus.
// Events are logged before being published to the inner bus.
func NewLoggedBus(inner Bus, eventLogger *EventLogger, log *logger.Logger) *LoggedBus {
	if log == nil {
		log = logger.Default()
	}
	return &LoggedBus{
		inner:       inner,
		eventLogger: eventLogger,
		log:         log,
	}
}

// Publish logs the event and then delegates to the inner bus.
func (b *LoggedBus) Publish(ctx context.Context, topic string, event Event) error {
	// Log the event (non-blocking, best-effort)
	if err := b.eventLogger.Log(topic, event); err != nil {
		b.log.Warn("Failed to log event to disk",
			"topic", topic,
			"error", err.Error(),
		)
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
	if err := b.eventLogger.Log(topic, req); err != nil {
		b.log.Warn("Failed to log request event to disk",
			"topic", topic,
			"error", err.Error(),
		)
	}

	// Delegate to inner bus
	resp, err := b.inner.Request(ctx, topic, req)

	// Log the response if successful
	if err == nil {
		if logErr := b.eventLogger.Log(topic+".response", resp); logErr != nil {
			b.log.Warn("Failed to log response event to disk",
				"topic", topic+".response",
				"error", logErr.Error(),
			)
		}
	}

	return resp, err
}

// Close closes both the event logger and the inner bus.
func (b *LoggedBus) Close() error {
	// Close event logger first
	if err := b.eventLogger.Close(); err != nil {
		b.log.Warn("Failed to close event logger",
			"error", err.Error(),
		)
	}

	// Close inner bus
	return b.inner.Close()
}
