package bus

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// MemoryBus is an in-memory event bus using Go channels.
type MemoryBus struct {
	mu         sync.RWMutex
	handlers   map[string][]Handler
	pending    map[string]chan Event
	closed     bool
	timeout    time.Duration
	inflightWg sync.WaitGroup // Tracks in-flight handlers for graceful shutdown
}

// NewMemoryBus creates a new in-memory event bus.
func NewMemoryBus() *MemoryBus {
	return &MemoryBus{
		handlers: make(map[string][]Handler),
		pending:  make(map[string]chan Event),
		timeout:  30 * time.Second,
	}
}

// Publish publishes an event to all subscribers of a topic.
func (b *MemoryBus) Publish(ctx context.Context, topic string, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return errors.New(errors.CodeUnavailable, "bus is closed")
	}

	handlers, ok := b.handlers[topic]
	if !ok || len(handlers) == 0 {
		return nil // No subscribers, not an error
	}

	// Fan out to all handlers with in-flight tracking
	for _, handler := range handlers {
		b.inflightWg.Add(1)
		go func(h Handler) {
			defer b.inflightWg.Done()
			if err := h(ctx, event); err != nil {
				// Log error but don't fail the publish
				fmt.Printf("handler error for topic %s: %v\n", topic, err)
			}
		}(handler)
	}

	return nil
}

// Subscribe registers a handler for events on a topic.
func (b *MemoryBus) Subscribe(ctx context.Context, topic string, handler Handler) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return errors.New(errors.CodeUnavailable, "bus is closed")
	}

	b.handlers[topic] = append(b.handlers[topic], handler)
	return nil
}

// Request sends a request and waits for a response.
func (b *MemoryBus) Request(ctx context.Context, topic string, req Event) (Event, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return Event{}, errors.New(errors.CodeUnavailable, "bus is closed")
	}

	// Create a response channel for this correlation ID
	responseChan := make(chan Event, 1)
	b.pending[req.CorrelationID] = responseChan
	b.mu.Unlock()

	// Clean up when done
	defer func() {
		b.mu.Lock()
		delete(b.pending, req.CorrelationID)
		close(responseChan)
		b.mu.Unlock()
	}()

	// Publish the request
	if err := b.Publish(ctx, topic, req); err != nil {
		return Event{}, err
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return Event{}, errors.Wrap(errors.CodeTimeout, "request timeout", ctx.Err())
	case <-time.After(b.timeout):
		return Event{}, errors.New(errors.CodeTimeout, "request timeout")
	case resp := <-responseChan:
		return resp, nil
	}
}

// Respond sends a response for a pending request.
func (b *MemoryBus) Respond(correlationID string, event Event) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.closed {
		return errors.New(errors.CodeUnavailable, "bus is closed")
	}

	ch, ok := b.pending[correlationID]
	if !ok {
		return errors.New(errors.CodeNotFound, "no pending request for correlation ID")
	}

	select {
	case ch <- event:
		return nil
	default:
		return errors.New(errors.CodeInternal, "response channel full")
	}
}

// Close closes the bus, waiting for in-flight handlers to complete.
func (b *MemoryBus) Close() error {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()

	// Wait for in-flight handlers with timeout
	done := make(chan struct{})
	go func() {
		b.inflightWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All handlers completed
	case <-time.After(10 * time.Second):
		fmt.Println("bus: event drain timeout reached, some handlers may not have completed")
	}

	b.mu.Lock()
	b.handlers = nil
	b.pending = nil
	b.mu.Unlock()

	return nil
}

// DrainTimeout waits for in-flight handlers to complete with custom timeout.
func (b *MemoryBus) DrainTimeout(timeout time.Duration) bool {
	done := make(chan struct{})
	go func() {
		b.inflightWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

// InFlightCount returns an approximate count of in-flight handlers.
// Note: This is not exact due to race conditions, use for monitoring only.
func (b *MemoryBus) InFlightCount() int {
	// WaitGroup doesn't expose count directly, so we track it separately if needed
	// For now, return 0 as we don't have a separate counter
	return 0
}
