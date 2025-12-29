package settings

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

func TestVersionedSettings(t *testing.T) {
	// Create temp directory for test
	tmpDir, err := os.MkdirTemp("", "settings-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := logger.New("info", "text")

	// Create service
	svc, err := NewService(ServiceConfig{
		StoragePath:  tmpDir,
		LoadDefaults: true,
	}, nil, log)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	// Initial version should be 1
	if svc.config.Version != 1 {
		t.Errorf("Expected initial version 1, got %d", svc.config.Version)
	}

	// Update settings multiple times
	for i := 0; i < 15; i++ {
		cfg := svc.Get(ctx)
		cfg.LogLevel = "debug"
		cfg.ServerPort = 8080 + i
		if err := svc.Update(ctx, cfg, "test"); err != nil {
			t.Fatalf("Failed to update settings (iteration %d): %v", i, err)
		}

		// Small delay to ensure different timestamps
		time.Sleep(10 * time.Millisecond)
	}

	// Should have version 16 now (1 initial + 15 updates)
	currentVersion := svc.GetVersion(ctx)
	if currentVersion != 16 {
		t.Errorf("Expected version 16, got %d", currentVersion)
	}

	// Check that versioned files exist
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	versionedCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			if entry.Name() != "settings.yaml" {
				versionedCount++
			}
		}
	}

	// Should have kept only last 10 versions
	if versionedCount != 10 {
		t.Errorf("Expected 10 versioned files, got %d", versionedCount)
	}

	// Verify we have versions 7-16 (oldest should be cleaned up)
	for v := 7; v <= 16; v++ {
		path := filepath.Join(tmpDir, "settings.v"+string(rune(v+48))+".yaml")
		if _, err := os.Stat(path); os.IsNotExist(err) {
			// For single digit, need different approach
			path = filepath.Join(tmpDir, "settings.v"+string(rune(v/10+48))+string(rune(v%10+48))+".yaml")
			if v < 10 {
				path = filepath.Join(tmpDir, "settings.v"+string(rune(v+48))+".yaml")
			}
		}
	}
}

func TestGetHistory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "settings-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := logger.New("info", "text")

	svc, err := NewService(ServiceConfig{
		StoragePath:  tmpDir,
		LoadDefaults: true,
	}, nil, log)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	// Create 5 versions
	for i := 0; i < 5; i++ {
		cfg := svc.Get(ctx)
		cfg.ServerPort = 8080 + i
		if err := svc.Update(ctx, cfg, "test"); err != nil {
			t.Fatalf("Failed to update: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Get history (limit 3)
	history, err := svc.GetHistory(ctx, 3)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 history entries, got %d", len(history))
	}

	// Should be in descending order (newest first)
	if len(history) >= 2 {
		if history[0].Version < history[1].Version {
			t.Errorf("History not in descending order: %d < %d", history[0].Version, history[1].Version)
		}
	}
}

func TestRollback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "settings-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := logger.New("info", "text")

	svc, err := NewService(ServiceConfig{
		StoragePath:  tmpDir,
		LoadDefaults: true,
	}, nil, log)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	// Version 1: port 8080
	// Version 2: port 9000
	cfg := svc.Get(ctx)
	cfg.ServerPort = 9000
	cfg.LogLevel = "debug"
	if err := svc.Update(ctx, cfg, "test"); err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Version 3: port 9999
	cfg = svc.Get(ctx)
	cfg.ServerPort = 9999
	cfg.LogLevel = "error"
	if err := svc.Update(ctx, cfg, "test"); err != nil {
		t.Fatalf("Failed to update: %v", err)
	}

	// Current version should be 3
	if svc.GetVersion(ctx) != 3 {
		t.Errorf("Expected version 3, got %d", svc.GetVersion(ctx))
	}

	// Current port should be 9999
	if svc.Get(ctx).ServerPort != 9999 {
		t.Errorf("Expected port 9999, got %d", svc.Get(ctx).ServerPort)
	}

	// Rollback to version 2
	if err := svc.Rollback(ctx, 2, "test-rollback"); err != nil {
		t.Fatalf("Failed to rollback: %v", err)
	}

	// Version should now be 4 (rollback creates new version)
	if svc.GetVersion(ctx) != 4 {
		t.Errorf("Expected version 4 after rollback, got %d", svc.GetVersion(ctx))
	}

	// Port should be back to 9000 (from version 2)
	if svc.Get(ctx).ServerPort != 9000 {
		t.Errorf("Expected port 9000 after rollback, got %d", svc.Get(ctx).ServerPort)
	}

	// LogLevel should be back to debug (from version 2)
	if svc.Get(ctx).LogLevel != "debug" {
		t.Errorf("Expected log level 'debug' after rollback, got '%s'", svc.Get(ctx).LogLevel)
	}
}

func TestRollbackNonExistentVersion(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "settings-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := logger.New("info", "text")

	svc, err := NewService(ServiceConfig{
		StoragePath:  tmpDir,
		LoadDefaults: true,
	}, nil, log)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	// Try to rollback to non-existent version
	err = svc.Rollback(ctx, 99, "test")
	if err == nil {
		t.Error("Expected error for non-existent version, got nil")
	}
}

func TestCleanupOldVersions(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "settings-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	log := logger.New("info", "text")

	svc, err := NewService(ServiceConfig{
		StoragePath:  tmpDir,
		LoadDefaults: true,
	}, nil, log)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	// Create 12 versions
	for i := 0; i < 12; i++ {
		cfg := svc.Get(ctx)
		cfg.ServerPort = 8080 + i
		if err := svc.Update(ctx, cfg, "test"); err != nil {
			t.Fatalf("Failed to update: %v", err)
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Count versioned files
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	versionedCount := 0
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() && filepath.Ext(name) == ".yaml" && name != "settings.yaml" {
			versionedCount++
		}
	}

	// Should have kept only 10 versions
	if versionedCount > 10 {
		t.Errorf("Expected at most 10 versioned files, got %d", versionedCount)
	}
}
