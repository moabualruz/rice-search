// Package connection provides connection and PC tracking for Rice Search.
package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// AuditEntry represents an audit log entry.
type AuditEntry struct {
	Timestamp    time.Time              `json:"timestamp"`
	EventType    string                 `json:"event_type"`
	ConnectionID string                 `json:"connection_id,omitempty"`
	Name         string                 `json:"name,omitempty"`
	IP           string                 `json:"ip,omitempty"`
	Details      map[string]interface{} `json:"details,omitempty"`
}

// AuditLogger logs connection events for security auditing.
type AuditLogger struct {
	log     *logger.Logger
	logPath string
	file    *os.File
	mu      sync.Mutex
}

// AuditLoggerConfig configures the audit logger.
type AuditLoggerConfig struct {
	// LogPath is the path to the audit log file.
	// If empty, logs to the application logger only.
	LogPath string

	// Enabled controls whether audit logging is active.
	Enabled bool
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(cfg AuditLoggerConfig, log *logger.Logger) (*AuditLogger, error) {
	a := &AuditLogger{
		log:     log,
		logPath: cfg.LogPath,
	}

	// Open log file if path specified
	if cfg.LogPath != "" {
		// Ensure directory exists
		dir := filepath.Dir(cfg.LogPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create audit log directory: %w", err)
		}

		f, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open audit log file: %w", err)
		}
		a.file = f
	}

	return a, nil
}

// SubscribeToEvents subscribes to connection events on the event bus.
func (a *AuditLogger) SubscribeToEvents(ctx context.Context, eventBus bus.Bus) error {
	// Subscribe to connection registered events
	if err := eventBus.Subscribe(ctx, TopicConnectionRegistered, a.handleConnectionRegistered); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", TopicConnectionRegistered, err)
	}

	// Subscribe to connection seen events
	if err := eventBus.Subscribe(ctx, TopicConnectionSeen, a.handleConnectionSeen); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", TopicConnectionSeen, err)
	}

	// Subscribe to connection deleted events
	if err := eventBus.Subscribe(ctx, TopicConnectionDeleted, a.handleConnectionDeleted); err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", TopicConnectionDeleted, err)
	}

	// Subscribe to connection renamed events
	if err := eventBus.Subscribe(ctx, "connection.renamed", a.handleConnectionRenamed); err != nil {
		return fmt.Errorf("failed to subscribe to connection.renamed: %w", err)
	}

	a.log.Info("Audit logger subscribed to connection events")
	return nil
}

func (a *AuditLogger) handleConnectionRegistered(ctx context.Context, event bus.Event) error {
	conn, ok := event.Payload.(*Connection)
	if !ok {
		a.log.Warn("Invalid payload for connection.registered event")
		return nil
	}

	entry := AuditEntry{
		Timestamp:    time.Now(),
		EventType:    "connection.registered",
		ConnectionID: conn.ID,
		Name:         conn.Name,
		IP:           conn.LastIP,
		Details: map[string]interface{}{
			"hostname":   conn.PCInfo.Hostname,
			"os":         conn.PCInfo.OS,
			"arch":       conn.PCInfo.Arch,
			"username":   conn.PCInfo.Username,
			"is_active":  conn.IsActive,
			"created_at": conn.CreatedAt,
		},
	}

	return a.writeEntry(entry)
}

func (a *AuditLogger) handleConnectionSeen(ctx context.Context, event bus.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		a.log.Warn("Invalid payload for connection.seen event")
		return nil
	}

	connID, _ := payload["connection_id"].(string)
	ip, _ := payload["ip"].(string)

	entry := AuditEntry{
		Timestamp:    time.Now(),
		EventType:    "connection.seen",
		ConnectionID: connID,
		IP:           ip,
	}

	return a.writeEntry(entry)
}

func (a *AuditLogger) handleConnectionDeleted(ctx context.Context, event bus.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		a.log.Warn("Invalid payload for connection.deleted event")
		return nil
	}

	connID, _ := payload["connection_id"].(string)
	name, _ := payload["name"].(string)

	entry := AuditEntry{
		Timestamp:    time.Now(),
		EventType:    "connection.deleted",
		ConnectionID: connID,
		Name:         name,
	}

	return a.writeEntry(entry)
}

func (a *AuditLogger) handleConnectionRenamed(ctx context.Context, event bus.Event) error {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		a.log.Warn("Invalid payload for connection.renamed event")
		return nil
	}

	connID, _ := payload["connection_id"].(string)
	oldName, _ := payload["old_name"].(string)
	newName, _ := payload["new_name"].(string)

	entry := AuditEntry{
		Timestamp:    time.Now(),
		EventType:    "connection.renamed",
		ConnectionID: connID,
		Details: map[string]interface{}{
			"old_name": oldName,
			"new_name": newName,
		},
	}

	return a.writeEntry(entry)
}

// writeEntry writes an audit entry to the log.
func (a *AuditLogger) writeEntry(entry AuditEntry) error {
	// Log to application logger
	a.log.Info("Connection audit",
		"event", entry.EventType,
		"connection_id", entry.ConnectionID,
		"name", entry.Name,
		"ip", entry.IP,
	)

	// Write to audit log file if configured
	if a.file != nil {
		a.mu.Lock()
		defer a.mu.Unlock()

		data, err := json.Marshal(entry)
		if err != nil {
			a.log.Error("Failed to marshal audit entry", "error", err)
			return err
		}

		if _, err := a.file.Write(append(data, '\n')); err != nil {
			a.log.Error("Failed to write audit entry", "error", err)
			return err
		}
	}

	return nil
}

// Close closes the audit logger.
func (a *AuditLogger) Close() error {
	if a.file != nil {
		return a.file.Close()
	}
	return nil
}
