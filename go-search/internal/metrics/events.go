package metrics

import (
	"context"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
)

// EventSubscriber subscribes to event bus and updates metrics.
type EventSubscriber struct {
	metrics *Metrics
	bus     bus.Bus
}

// NewEventSubscriber creates a new event subscriber.
func NewEventSubscriber(metrics *Metrics, eventBus bus.Bus) *EventSubscriber {
	return &EventSubscriber{
		metrics: metrics,
		bus:     eventBus,
	}
}

// SubscribeToEvents subscribes to all relevant events and updates metrics.
func (es *EventSubscriber) SubscribeToEvents(ctx context.Context) error {
	// Subscribe to search events
	if err := es.bus.Subscribe(ctx, bus.TopicSearchRequest, es.handleSearchRequest); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicSearchResponse, es.handleSearchResponse); err != nil {
		return err
	}

	// Subscribe to index events
	if err := es.bus.Subscribe(ctx, bus.TopicIndexRequest, es.handleIndexRequest); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicIndexResponse, es.handleIndexResponse); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicChunkCreated, es.handleChunkCreated); err != nil {
		return err
	}

	// Subscribe to ML events
	if err := es.bus.Subscribe(ctx, bus.TopicEmbedRequest, es.handleEmbedRequest); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicEmbedResponse, es.handleEmbedResponse); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicRerankRequest, es.handleRerankRequest); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicRerankResponse, es.handleRerankResponse); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicSparseRequest, es.handleSparseRequest); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicSparseResponse, es.handleSparseResponse); err != nil {
		return err
	}

	// Subscribe to store events
	if err := es.bus.Subscribe(ctx, bus.TopicStoreCreated, es.handleStoreCreated); err != nil {
		return err
	}
	if err := es.bus.Subscribe(ctx, bus.TopicStoreDeleted, es.handleStoreDeleted); err != nil {
		return err
	}

	return nil
}

// Event handlers

func (es *EventSubscriber) handleSearchRequest(ctx context.Context, event bus.Event) error {
	es.metrics.SearchRequests.Inc()
	return nil
}

func (es *EventSubscriber) handleSearchResponse(ctx context.Context, event bus.Event) error {
	// Extract latency and result count from payload if available
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if latency, ok := payload["latency_ms"].(int64); ok {
			if resultCount, ok := payload["result_count"].(int); ok {
				es.metrics.SearchLatency.Observe(float64(latency))
				es.metrics.SearchResults.Observe(float64(resultCount))
			}
		}
		if err, ok := payload["error"]; ok && err != nil {
			es.metrics.SearchErrors.WithLabels("generic").Inc()
		}
	}
	return nil
}

func (es *EventSubscriber) handleIndexRequest(ctx context.Context, event bus.Event) error {
	// Index request received
	return nil
}

func (es *EventSubscriber) handleIndexResponse(ctx context.Context, event bus.Event) error {
	// Extract indexing stats from payload
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if docCount, ok := payload["doc_count"].(int); ok {
			es.metrics.IndexedDocuments.Add(int64(docCount))
		}
		if chunkCount, ok := payload["chunk_count"].(int); ok {
			es.metrics.IndexedChunks.Add(int64(chunkCount))
		}
		if latency, ok := payload["latency_ms"].(int64); ok {
			es.metrics.IndexLatency.Observe(float64(latency))
		}
		if err, ok := payload["error"]; ok && err != nil {
			es.metrics.IndexErrors.WithLabels("generic").Inc()
		}
	}
	return nil
}

func (es *EventSubscriber) handleChunkCreated(ctx context.Context, event bus.Event) error {
	es.metrics.IndexedChunks.Inc()
	return nil
}

func (es *EventSubscriber) handleEmbedRequest(ctx context.Context, event bus.Event) error {
	es.metrics.EmbedRequests.Inc()

	// Extract batch size if available
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if batchSize, ok := payload["batch_size"].(int); ok {
			es.metrics.EmbedBatchSize.Observe(float64(batchSize))
		}
	}
	return nil
}

func (es *EventSubscriber) handleEmbedResponse(ctx context.Context, event bus.Event) error {
	// Extract latency from payload
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if latency, ok := payload["latency_ms"].(int64); ok {
			es.metrics.EmbedLatency.Observe(float64(latency))
		}
	}
	return nil
}

func (es *EventSubscriber) handleRerankRequest(ctx context.Context, event bus.Event) error {
	es.metrics.RerankRequests.Inc()
	return nil
}

func (es *EventSubscriber) handleRerankResponse(ctx context.Context, event bus.Event) error {
	// Extract latency from payload
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if latency, ok := payload["latency_ms"].(int64); ok {
			es.metrics.RerankLatency.Observe(float64(latency))
		}
	}
	return nil
}

func (es *EventSubscriber) handleSparseRequest(ctx context.Context, event bus.Event) error {
	es.metrics.SparseEncodeRequests.Inc()
	return nil
}

func (es *EventSubscriber) handleSparseResponse(ctx context.Context, event bus.Event) error {
	// Extract latency from payload
	if payload, ok := event.Payload.(map[string]interface{}); ok {
		if latency, ok := payload["latency_ms"].(int64); ok {
			es.metrics.SparseEncodeLatency.Observe(float64(latency))
		}
	}
	return nil
}

func (es *EventSubscriber) handleStoreCreated(ctx context.Context, event bus.Event) error {
	// Store count will be updated by collector
	return nil
}

func (es *EventSubscriber) handleStoreDeleted(ctx context.Context, event bus.Event) error {
	// Store count will be updated by collector
	return nil
}

// RecordSearchMetrics records metrics for a search operation.
// This is a helper for when events aren't available.
func (es *EventSubscriber) RecordSearchMetrics(latencyMs int64, resultCount int, err error) {
	es.metrics.RecordSearch(latencyMs, resultCount, err)
}

// RecordIndexMetrics records metrics for an indexing operation.
// This is a helper for when events aren't available.
func (es *EventSubscriber) RecordIndexMetrics(docCount, chunkCount int, latencyMs int64, err error) {
	es.metrics.RecordIndex(docCount, chunkCount, latencyMs, err)
}

// TimedSearch wraps a search operation and records metrics.
func (es *EventSubscriber) TimedSearch(fn func() (int, error)) error {
	start := time.Now()
	resultCount, err := fn()
	latencyMs := time.Since(start).Milliseconds()
	es.RecordSearchMetrics(latencyMs, resultCount, err)
	return err
}

// TimedIndex wraps an indexing operation and records metrics.
func (es *EventSubscriber) TimedIndex(fn func() (int, int, error)) error {
	start := time.Now()
	docCount, chunkCount, err := fn()
	latencyMs := time.Since(start).Milliseconds()
	es.RecordIndexMetrics(docCount, chunkCount, latencyMs, err)
	return err
}
