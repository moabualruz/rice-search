// Package bus provides event bus implementations for inter-service communication.
package bus

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/errors"
)

// LoggedEvent represents an event that has been logged to disk.
type LoggedEvent struct {
	Event     Event     `json:"event"`
	Topic     string    `json:"topic"`
	Timestamp time.Time `json:"timestamp"`
}

// EventLogger logs events to disk for debugging and replay.
// Events are written as JSON lines (one JSON object per line).
type EventLogger struct {
	logPath string
	mu      sync.Mutex
	file    *os.File
	enabled bool
	encoder *json.Encoder
}

// EventLoggerConfig holds configuration for the event logger.
type EventLoggerConfig struct {
	LogPath string
	Enabled bool
}

// NewEventLogger creates a new event logger.
// If enabled is false, the logger will be created but will not write events.
func NewEventLogger(logPath string, enabled bool) (*EventLogger, error) {
	logger := &EventLogger{
		logPath: logPath,
		enabled: enabled,
	}

	if !enabled {
		return logger, nil
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(logPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open file in append mode (create if doesn't exist)
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	logger.file = file
	logger.encoder = json.NewEncoder(file)

	return logger, nil
}

// Log writes an event to the log file.
// If the logger is disabled, this is a no-op.
func (l *EventLogger) Log(topic string, event Event) error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return errors.New(errors.CodeInternal, "event logger not initialized")
	}

	loggedEvent := LoggedEvent{
		Event:     event,
		Topic:     topic,
		Timestamp: time.Now(),
	}

	if err := l.encoder.Encode(loggedEvent); err != nil {
		return fmt.Errorf("failed to encode event: %w", err)
	}

	// Flush to ensure it's written immediately (important for debugging)
	if err := l.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync log file: %w", err)
	}

	return nil
}

// GetEvents reads events from the log file.
// Returns events that occurred after the 'since' timestamp.
// If limit > 0, returns at most that many events.
// Events are returned in chronological order.
func (l *EventLogger) GetEvents(since time.Time, limit int) ([]LoggedEvent, error) {
	if !l.enabled {
		return nil, errors.New(errors.CodeUnavailable, "event logging is disabled")
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Open file for reading
	file, err := os.Open(l.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []LoggedEvent{}, nil
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	var events []LoggedEvent
	scanner := bufio.NewScanner(file)

	// Increase buffer size for potentially large events
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		var loggedEvent LoggedEvent
		if err := json.Unmarshal(scanner.Bytes(), &loggedEvent); err != nil {
			// Skip malformed lines
			continue
		}

		// Filter by timestamp
		if loggedEvent.Timestamp.After(since) {
			events = append(events, loggedEvent)

			// Check limit
			if limit > 0 && len(events) >= limit {
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan log file: %w", err)
	}

	return events, nil
}

// Replay reads events from the log file and publishes them to the bus.
// This is useful for debugging and testing.
// Only events that occurred after 'since' will be replayed.
func (l *EventLogger) Replay(ctx context.Context, bus Bus, since time.Time) error {
	if !l.enabled {
		return errors.New(errors.CodeUnavailable, "event logging is disabled")
	}

	events, err := l.GetEvents(since, 0) // Get all events
	if err != nil {
		return fmt.Errorf("failed to get events: %w", err)
	}

	// Replay events in order
	for _, loggedEvent := range events {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := bus.Publish(ctx, loggedEvent.Topic, loggedEvent.Event); err != nil {
				return fmt.Errorf("failed to replay event %s: %w", loggedEvent.Event.ID, err)
			}
		}
	}

	return nil
}

// Close closes the log file.
func (l *EventLogger) Close() error {
	if !l.enabled {
		return nil
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		if err := l.file.Close(); err != nil {
			return fmt.Errorf("failed to close log file: %w", err)
		}
		l.file = nil
		l.encoder = nil
	}

	return nil
}

// IsEnabled returns true if the logger is enabled.
func (l *EventLogger) IsEnabled() bool {
	return l.enabled
}
