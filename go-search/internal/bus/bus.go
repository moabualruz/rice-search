// Package bus provides event bus implementations for inter-service communication.
package bus

import (
	"context"
	"time"
)

// Handler is a function that handles events.
type Handler func(ctx context.Context, event Event) error

// Bus defines the interface for event bus implementations.
type Bus interface {
	// Publish publishes an event to a topic.
	Publish(ctx context.Context, topic string, event Event) error

	// Subscribe subscribes to events on a topic.
	Subscribe(ctx context.Context, topic string, handler Handler) error

	// Request sends a request and waits for a response.
	Request(ctx context.Context, topic string, req Event) (Event, error)

	// Close closes the bus and releases resources.
	Close() error
}

// Drainable is an optional interface for buses that support graceful draining.
// Buses implementing this interface can wait for in-flight handlers to complete.
type Drainable interface {
	// DrainTimeout waits for in-flight handlers to complete.
	// Returns true if all handlers completed, false if timeout was reached.
	DrainTimeout(timeout time.Duration) bool
}

// DrainBus attempts to drain a bus if it supports draining.
// Returns true if draining completed or bus doesn't support draining.
func DrainBus(b Bus, timeout time.Duration) bool {
	if drainable, ok := b.(Drainable); ok {
		return drainable.DrainTimeout(timeout)
	}
	return true // Bus doesn't support draining, consider it done
}

// Event represents a bus event.
type Event struct {
	// ID is the unique event identifier.
	ID string `json:"id"`

	// Type is the event type (e.g., "embed.request", "search.response").
	Type string `json:"type"`

	// Source is the service that generated the event.
	Source string `json:"source"`

	// Timestamp is when the event was created.
	Timestamp int64 `json:"timestamp"`

	// CorrelationID links related events (e.g., request/response).
	CorrelationID string `json:"correlation_id,omitempty"`

	// Payload contains the event data.
	Payload any `json:"payload"`
}

// Topics for different event types.
const (
	// ML service topics.
	TopicEmbedRequest   = "ml.embed.request"
	TopicEmbedResponse  = "ml.embed.response"
	TopicSparseRequest  = "ml.sparse.request"
	TopicSparseResponse = "ml.sparse.response"
	TopicRerankRequest  = "ml.rerank.request"
	TopicRerankResponse = "ml.rerank.response"

	// Search service topics.
	TopicSearchRequest  = "search.request"
	TopicSearchResponse = "search.response"

	// Index service topics.
	TopicIndexRequest  = "index.request"
	TopicIndexResponse = "index.response"
	TopicChunkCreated  = "index.chunk.created"

	// Store service topics.
	TopicStoreCreated = "store.created"
	TopicStoreDeleted = "store.deleted"

	// Settings service topics.
	TopicSettingsChanged = "settings.changed"

	// Model service topics.
	TopicModelDownloaded = "model.downloaded"
	TopicModelValidated  = "model.validated"

	// Connection service topics.
	TopicConnectionRegistered   = "connection.registered"
	TopicConnectionUnregistered = "connection.unregistered"
	TopicConnectionActivity     = "connection.activity"

	// Alert topics.
	TopicAlertTriggered = "alert.triggered"
)
