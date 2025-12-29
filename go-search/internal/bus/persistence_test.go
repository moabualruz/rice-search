package bus

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEventLogger(t *testing.T) {
	// Create temp directory for test
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "events.log")

	t.Run("NewEventLogger_Enabled", func(t *testing.T) {
		logger, err := NewEventLogger(logPath, true)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		if !logger.IsEnabled() {
			t.Error("Expected logger to be enabled")
		}
	})

	t.Run("NewEventLogger_Disabled", func(t *testing.T) {
		logger, err := NewEventLogger(logPath, false)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		if logger.IsEnabled() {
			t.Error("Expected logger to be disabled")
		}
	})

	t.Run("Log_Enabled", func(t *testing.T) {
		logger, err := NewEventLogger(logPath, true)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		event := Event{
			ID:     "test-123",
			Type:   "test.event",
			Source: "test",
			Payload: map[string]string{
				"key": "value",
			},
		}

		if err := logger.Log("test.topic", event); err != nil {
			t.Fatalf("Log failed: %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(logPath); os.IsNotExist(err) {
			t.Fatal("Log file was not created")
		}
	})

	t.Run("Log_Disabled", func(t *testing.T) {
		logger, err := NewEventLogger(logPath, false)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		event := Event{
			ID:     "test-456",
			Type:   "test.event",
			Source: "test",
		}

		// Should not error, just no-op
		if err := logger.Log("test.topic", event); err != nil {
			t.Fatalf("Log failed: %v", err)
		}
	})

	t.Run("GetEvents", func(t *testing.T) {
		// Clean up any existing log file
		os.Remove(logPath)

		logger, err := NewEventLogger(logPath, true)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		// Log multiple events
		now := time.Now()
		for i := 0; i < 5; i++ {
			event := Event{
				ID:        "event-" + string(rune('1'+i)),
				Type:      "test.event",
				Source:    "test",
				Timestamp: now.Add(time.Duration(i) * time.Second).Unix(),
			}
			if err := logger.Log("test.topic", event); err != nil {
				t.Fatalf("Log failed: %v", err)
			}
		}

		// Get all events
		events, err := logger.GetEvents(now.Add(-1*time.Minute), 0)
		if err != nil {
			t.Fatalf("GetEvents failed: %v", err)
		}

		if len(events) != 5 {
			t.Errorf("Expected 5 events, got %d", len(events))
		}

		// Get events with limit
		events, err = logger.GetEvents(now.Add(-1*time.Minute), 3)
		if err != nil {
			t.Fatalf("GetEvents failed: %v", err)
		}

		if len(events) != 3 {
			t.Errorf("Expected 3 events (limit), got %d", len(events))
		}

		// Get events after a specific time (wait a bit to ensure distinct times)
		time.Sleep(100 * time.Millisecond)
		cutoff := time.Now().Add(-2 * time.Second)
		events, err = logger.GetEvents(cutoff, 0)
		if err != nil {
			t.Fatalf("GetEvents failed: %v", err)
		}

		// Should get all 5 events since they were logged within the past few seconds
		if len(events) < 3 {
			t.Errorf("Expected at least 3 events (since filter), got %d", len(events))
		}
	})

	t.Run("Replay", func(t *testing.T) {
		// Clean up any existing log file
		os.Remove(logPath)

		logger, err := NewEventLogger(logPath, true)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		// Log events
		now := time.Now()
		for i := 0; i < 3; i++ {
			event := Event{
				ID:        "replay-" + string(rune('1'+i)),
				Type:      "test.replay",
				Source:    "test",
				Timestamp: now.Add(time.Duration(i) * time.Second).Unix(),
			}
			if err := logger.Log("test.replay", event); err != nil {
				t.Fatalf("Log failed: %v", err)
			}
		}

		// Create a new bus for replay
		replayBus := NewMemoryBus()
		defer replayBus.Close()

		// Subscribe to events
		eventCount := 0
		ctx := context.Background()
		err = replayBus.Subscribe(ctx, "test.replay", func(ctx context.Context, event Event) error {
			eventCount++
			return nil
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}

		// Replay events
		if err := logger.Replay(ctx, replayBus, now.Add(-1*time.Minute)); err != nil {
			t.Fatalf("Replay failed: %v", err)
		}

		// Give handlers time to process
		time.Sleep(100 * time.Millisecond)

		if eventCount != 3 {
			t.Errorf("Expected 3 replayed events, got %d", eventCount)
		}
	})
}

func TestLoggedBus(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "logged_bus.log")

	t.Run("Publish_LogsEvent", func(t *testing.T) {
		innerBus := NewMemoryBus()
		defer innerBus.Close()

		logger, err := NewEventLogger(logPath, true)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		loggedBus := NewLoggedBus(innerBus, logger)
		defer loggedBus.Close()

		event := Event{
			ID:     "test-pub",
			Type:   "test.publish",
			Source: "test",
		}

		ctx := context.Background()
		if err := loggedBus.Publish(ctx, "test.topic", event); err != nil {
			t.Fatalf("Publish failed: %v", err)
		}

		// Verify event was logged
		events, err := logger.GetEvents(time.Now().Add(-1*time.Minute), 0)
		if err != nil {
			t.Fatalf("GetEvents failed: %v", err)
		}

		if len(events) != 1 {
			t.Errorf("Expected 1 logged event, got %d", len(events))
		}

		if events[0].Event.ID != "test-pub" {
			t.Errorf("Expected event ID 'test-pub', got '%s'", events[0].Event.ID)
		}
	})

	t.Run("Request_LogsRequestAndResponse", func(t *testing.T) {
		// Clean up
		os.Remove(logPath)

		innerBus := NewMemoryBus()
		defer innerBus.Close()

		logger, err := NewEventLogger(logPath, true)
		if err != nil {
			t.Fatalf("NewEventLogger failed: %v", err)
		}
		defer logger.Close()

		loggedBus := NewLoggedBus(innerBus, logger)
		defer loggedBus.Close()

		ctx := context.Background()

		// Subscribe to handle request
		err = loggedBus.Subscribe(ctx, "test.request", func(ctx context.Context, event Event) error {
			// Respond to the request
			resp := Event{
				ID:            "resp-123",
				Type:          "test.response",
				Source:        "test",
				CorrelationID: event.CorrelationID,
			}
			return innerBus.Respond(event.CorrelationID, resp)
		})
		if err != nil {
			t.Fatalf("Subscribe failed: %v", err)
		}

		req := Event{
			ID:            "req-123",
			Type:          "test.request",
			Source:        "test",
			CorrelationID: "corr-123",
		}

		_, err = loggedBus.Request(ctx, "test.request", req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}

		// Verify both request and response were logged
		time.Sleep(100 * time.Millisecond)
		events, err := logger.GetEvents(time.Now().Add(-1*time.Minute), 0)
		if err != nil {
			t.Fatalf("GetEvents failed: %v", err)
		}

		// Should have 2 events: request and response
		if len(events) < 2 {
			t.Errorf("Expected at least 2 logged events (request + response), got %d", len(events))
		}
	})
}
