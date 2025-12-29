package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuditLogger(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create audit logger
	logger, err := NewAuditLogger(AuditLoggerConfig{
		LogPath: logPath,
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("Failed to create audit logger: %v", err)
	}
	defer logger.Close()

	// Log some entries
	entry1 := AuditEntry{
		Timestamp: time.Now(),
		Version:   2,
		ChangedBy: "admin",
		Changes: []FieldChange{
			{Field: "ServerPort", OldValue: 8080, NewValue: 9090},
			{Field: "LogLevel", OldValue: "info", NewValue: "debug"},
		},
		IPAddress: "127.0.0.1",
		UserAgent: "test-agent/1.0",
	}

	entry2 := AuditEntry{
		Timestamp: time.Now().Add(1 * time.Minute),
		Version:   3,
		ChangedBy: "api",
		Changes: []FieldChange{
			{Field: "DefaultTopK", OldValue: 20, NewValue: 50},
		},
	}

	if err := logger.Log(entry1); err != nil {
		t.Fatalf("Failed to log entry 1: %v", err)
	}

	if err := logger.Log(entry2); err != nil {
		t.Fatalf("Failed to log entry 2: %v", err)
	}

	// Read entries back
	entries, err := logger.GetEntries(10)
	if err != nil {
		t.Fatalf("Failed to get entries: %v", err)
	}

	// Should have 2 entries (newest first)
	if len(entries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(entries))
	}

	// Check newest entry first
	if entries[0].Version != 3 {
		t.Errorf("Expected newest entry first (version 3), got version %d", entries[0].Version)
	}

	if entries[1].Version != 2 {
		t.Errorf("Expected second entry to be version 2, got version %d", entries[1].Version)
	}

	// Check entry details
	if entries[1].ChangedBy != "admin" {
		t.Errorf("Expected ChangedBy 'admin', got '%s'", entries[1].ChangedBy)
	}

	if len(entries[1].Changes) != 2 {
		t.Errorf("Expected 2 changes, got %d", len(entries[1].Changes))
	}

	// Test limit
	limitedEntries, err := logger.GetEntries(1)
	if err != nil {
		t.Fatalf("Failed to get limited entries: %v", err)
	}

	if len(limitedEntries) != 1 {
		t.Errorf("Expected 1 entry with limit=1, got %d", len(limitedEntries))
	}

	// Verify log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Audit log file was not created")
	}
}

func TestAuditLoggerDisabled(t *testing.T) {
	logger, err := NewAuditLogger(AuditLoggerConfig{
		Enabled: false,
	})
	if err != nil {
		t.Fatalf("Failed to create disabled audit logger: %v", err)
	}

	entry := AuditEntry{
		Timestamp: time.Now(),
		Version:   1,
		ChangedBy: "test",
		Changes:   []FieldChange{{Field: "Test", OldValue: "a", NewValue: "b"}},
	}

	// Should not error when logging to disabled logger
	if err := logger.Log(entry); err != nil {
		t.Errorf("Logging to disabled logger should not error: %v", err)
	}

	// Should return empty entries
	entries, err := logger.GetEntries(10)
	if err != nil {
		t.Fatalf("GetEntries on disabled logger failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("Expected 0 entries from disabled logger, got %d", len(entries))
	}
}

func TestDiff(t *testing.T) {
	old := DefaultConfig()
	new := DefaultConfig()

	// No changes
	changes := Diff(old, new)
	if len(changes) != 0 {
		t.Errorf("Expected 0 changes for identical configs, got %d", len(changes))
	}

	// Modify some fields
	new.ServerPort = 9090
	new.LogLevel = "debug"
	new.DefaultTopK = 50
	new.Features.EnableReranking = false

	changes = Diff(old, new)

	// Should have 4 changes (3 top-level + 1 feature flag)
	if len(changes) != 4 {
		t.Fatalf("Expected 4 changes, got %d", len(changes))
	}

	// Check specific changes
	changeMap := make(map[string]FieldChange)
	for _, c := range changes {
		changeMap[c.Field] = c
	}

	if c, ok := changeMap["ServerPort"]; !ok {
		t.Error("Missing ServerPort change")
	} else if c.OldValue != 8080 || c.NewValue != 9090 {
		t.Errorf("ServerPort change incorrect: old=%v, new=%v", c.OldValue, c.NewValue)
	}

	if c, ok := changeMap["LogLevel"]; !ok {
		t.Error("Missing LogLevel change")
	} else if c.OldValue != "info" || c.NewValue != "debug" {
		t.Errorf("LogLevel change incorrect: old=%v, new=%v", c.OldValue, c.NewValue)
	}

	if _, ok := changeMap["DefaultTopK"]; !ok {
		t.Error("Missing DefaultTopK change")
	}

	if c, ok := changeMap["Features.EnableReranking"]; !ok {
		t.Error("Missing Features.EnableReranking change")
	} else if c.OldValue != true || c.NewValue != false {
		t.Errorf("EnableReranking change incorrect: old=%v, new=%v", c.OldValue, c.NewValue)
	}
}

func TestDiffFeatureFlags(t *testing.T) {
	old := FeatureFlags{
		EnableQueryUnderstanding: true,
		EnableDiversity:          true,
		EnableDedup:              true,
		EnableReranking:          true,
		EnableABTesting:          false,
		EnableExperimental:       false,
	}

	new := old
	new.EnableReranking = false
	new.EnableExperimental = true

	changes := diffFeatures(old, new)

	// Should have 2 changes
	if len(changes) != 2 {
		t.Fatalf("Expected 2 changes in feature flags, got %d", len(changes))
	}

	// Check field names have "Features." prefix
	for _, c := range changes {
		if c.Field != "Features.EnableReranking" && c.Field != "Features.EnableExperimental" {
			t.Errorf("Unexpected feature flag change: %s", c.Field)
		}
	}
}
