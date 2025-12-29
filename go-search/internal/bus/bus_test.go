package bus

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryBus_PublishSubscribe(t *testing.T) {
	bus := NewMemoryBus()
	defer bus.Close()

	var received atomic.Int32
	var wg sync.WaitGroup

	// Subscribe to topic
	err := bus.Subscribe(context.Background(), "test.topic", func(ctx context.Context, event Event) error {
		received.Add(1)
		wg.Done()
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	// Publish events
	wg.Add(3)
	for i := 0; i < 3; i++ {
		err := bus.Publish(context.Background(), "test.topic", Event{
			ID:   "test-" + string(rune('0'+i)),
			Type: "test",
		})
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	// Wait for handlers
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for events")
	}

	if got := received.Load(); got != 3 {
		t.Errorf("Received %d events, want 3", got)
	}
}

func TestMemoryBus_MultipleSubscribers(t *testing.T) {
	bus := NewMemoryBus()
	defer bus.Close()

	var count1, count2 atomic.Int32
	var wg sync.WaitGroup

	// First subscriber
	bus.Subscribe(context.Background(), "test.topic", func(ctx context.Context, event Event) error {
		count1.Add(1)
		wg.Done()
		return nil
	})

	// Second subscriber
	bus.Subscribe(context.Background(), "test.topic", func(ctx context.Context, event Event) error {
		count2.Add(1)
		wg.Done()
		return nil
	})

	// Publish one event - both subscribers should receive
	wg.Add(2)
	bus.Publish(context.Background(), "test.topic", Event{ID: "test", Type: "test"})

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Timeout")
	}

	if count1.Load() != 1 || count2.Load() != 1 {
		t.Errorf("Expected both subscribers to receive 1 event, got %d and %d", count1.Load(), count2.Load())
	}
}

func TestMemoryBus_NoSubscribers(t *testing.T) {
	bus := NewMemoryBus()
	defer bus.Close()

	// Publishing to a topic with no subscribers should not error
	err := bus.Publish(context.Background(), "empty.topic", Event{ID: "test", Type: "test"})
	if err != nil {
		t.Errorf("Publish() to empty topic error = %v", err)
	}
}

func TestMemoryBus_Request(t *testing.T) {
	bus := NewMemoryBus()
	defer bus.Close()

	// Subscribe to request topic and respond
	bus.Subscribe(context.Background(), "req.topic", func(ctx context.Context, event Event) error {
		// Simulate processing and respond
		go func() {
			time.Sleep(10 * time.Millisecond)
			bus.Respond(event.CorrelationID, Event{
				ID:            "resp-1",
				Type:          "response",
				CorrelationID: event.CorrelationID,
				Payload:       "response data",
			})
		}()
		return nil
	})

	// Make request
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	resp, err := bus.Request(ctx, "req.topic", Event{
		ID:            "req-1",
		Type:          "request",
		CorrelationID: "corr-123",
	})

	if err != nil {
		t.Fatalf("Request() error = %v", err)
	}

	if resp.CorrelationID != "corr-123" {
		t.Errorf("Response CorrelationID = %s, want corr-123", resp.CorrelationID)
	}

	if resp.Payload != "response data" {
		t.Errorf("Response Payload = %v, want 'response data'", resp.Payload)
	}
}

func TestMemoryBus_RequestTimeout(t *testing.T) {
	bus := NewMemoryBus()
	bus.timeout = 50 * time.Millisecond
	defer bus.Close()

	// Subscribe but don't respond
	bus.Subscribe(context.Background(), "slow.topic", func(ctx context.Context, event Event) error {
		// Don't respond
		return nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := bus.Request(ctx, "slow.topic", Event{
		ID:            "req-1",
		CorrelationID: "corr-timeout",
	})

	if err == nil {
		t.Error("Request() should timeout, but got nil error")
	}
}

func TestMemoryBus_Close(t *testing.T) {
	bus := NewMemoryBus()

	// Close the bus
	if err := bus.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	// Operations should fail after close
	err := bus.Publish(context.Background(), "test", Event{})
	if err == nil {
		t.Error("Publish() after Close() should error")
	}

	err = bus.Subscribe(context.Background(), "test", func(ctx context.Context, event Event) error {
		return nil
	})
	if err == nil {
		t.Error("Subscribe() after Close() should error")
	}
}

func TestMemoryBus_Concurrent(t *testing.T) {
	bus := NewMemoryBus()
	defer bus.Close()

	var received atomic.Int32
	var wg sync.WaitGroup

	// Subscribe
	bus.Subscribe(context.Background(), "concurrent", func(ctx context.Context, event Event) error {
		received.Add(1)
		wg.Done()
		return nil
	})

	// Publish concurrently
	numPublishers := 10
	eventsPerPublisher := 100
	wg.Add(numPublishers * eventsPerPublisher)

	for p := 0; p < numPublishers; p++ {
		go func(publisher int) {
			for i := 0; i < eventsPerPublisher; i++ {
				bus.Publish(context.Background(), "concurrent", Event{
					ID:   "test",
					Type: "test",
				})
			}
		}(p)
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("Timeout: received %d events, expected %d", received.Load(), numPublishers*eventsPerPublisher)
	}

	expected := int32(numPublishers * eventsPerPublisher)
	if got := received.Load(); got != expected {
		t.Errorf("Received %d events, want %d", got, expected)
	}
}
