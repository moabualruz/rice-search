// Package settings provides audit logging for runtime settings changes.
package settings

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

// AuditEntry represents a single settings change audit log entry.
type AuditEntry struct {
	Timestamp time.Time     `json:"timestamp"`
	Version   int           `json:"version"`
	ChangedBy string        `json:"changed_by"` // "admin", "api", "import", "rollback"
	Changes   []FieldChange `json:"changes"`
	IPAddress string        `json:"ip_address,omitempty"`
	UserAgent string        `json:"user_agent,omitempty"`
}

// FieldChange represents a single field modification.
type FieldChange struct {
	Field    string      `json:"field"`
	OldValue interface{} `json:"old_value"`
	NewValue interface{} `json:"new_value"`
}

// AuditLogger writes settings changes to a JSON lines log file.
type AuditLogger struct {
	logPath string
	mu      sync.Mutex
	file    *os.File
	enabled bool
}

// AuditLoggerConfig configures the audit logger.
type AuditLoggerConfig struct {
	LogPath string
	Enabled bool
}

// NewAuditLogger creates a new audit logger.
func NewAuditLogger(cfg AuditLoggerConfig) (*AuditLogger, error) {
	if !cfg.Enabled {
		return &AuditLogger{enabled: false}, nil
	}

	// Ensure directory exists
	dir := filepath.Dir(cfg.LogPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create audit log directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &AuditLogger{
		logPath: cfg.LogPath,
		file:    file,
		enabled: true,
	}, nil
}

// Log writes an audit entry to the log file.
func (a *AuditLogger) Log(entry AuditEntry) error {
	if !a.enabled {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Ensure timestamp is set
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now()
	}

	// Marshal to JSON
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal audit entry: %w", err)
	}

	// Write JSON line
	if _, err := a.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit entry: %w", err)
	}

	// Sync to disk
	if err := a.file.Sync(); err != nil {
		return fmt.Errorf("failed to sync audit log: %w", err)
	}

	return nil
}

// GetEntries returns the last N audit entries from the log file.
func (a *AuditLogger) GetEntries(limit int) ([]AuditEntry, error) {
	if !a.enabled {
		return []AuditEntry{}, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// Open file for reading
	file, err := os.Open(a.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []AuditEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open audit log: %w", err)
	}
	defer file.Close()

	// Read all entries
	var entries []AuditEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var entry AuditEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			// Skip malformed lines
			continue
		}
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to scan audit log: %w", err)
	}

	// Return last N entries
	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	// Reverse to get newest first
	for i := 0; i < len(entries)/2; i++ {
		j := len(entries) - i - 1
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}

// Close closes the audit log file.
func (a *AuditLogger) Close() error {
	if !a.enabled || a.file == nil {
		return nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	return a.file.Close()
}

// Diff compares two configurations and returns the list of changes.
func Diff(old, new RuntimeConfig) []FieldChange {
	var changes []FieldChange

	// Use reflection to compare all fields
	oldVal := reflect.ValueOf(old)
	newVal := reflect.ValueOf(new)
	typ := oldVal.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldName := field.Name

		// Skip metadata fields
		if fieldName == "UpdatedAt" || fieldName == "Version" {
			continue
		}

		oldFieldVal := oldVal.Field(i).Interface()
		newFieldVal := newVal.Field(i).Interface()

		// Handle nested FeatureFlags struct
		if fieldName == "Features" {
			featChanges := diffFeatures(oldFieldVal.(FeatureFlags), newFieldVal.(FeatureFlags))
			changes = append(changes, featChanges...)
			continue
		}

		// Compare values
		if !reflect.DeepEqual(oldFieldVal, newFieldVal) {
			changes = append(changes, FieldChange{
				Field:    fieldName,
				OldValue: oldFieldVal,
				NewValue: newFieldVal,
			})
		}
	}

	return changes
}

// diffFeatures compares two FeatureFlags structs.
func diffFeatures(old, new FeatureFlags) []FieldChange {
	var changes []FieldChange

	oldVal := reflect.ValueOf(old)
	newVal := reflect.ValueOf(new)
	typ := oldVal.Type()

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		fieldName := "Features." + field.Name

		oldFieldVal := oldVal.Field(i).Interface()
		newFieldVal := newVal.Field(i).Interface()

		if !reflect.DeepEqual(oldFieldVal, newFieldVal) {
			changes = append(changes, FieldChange{
				Field:    fieldName,
				OldValue: oldFieldVal,
				NewValue: newFieldVal,
			})
		}
	}

	return changes
}
