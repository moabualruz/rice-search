package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestValidateStoreName(t *testing.T) {
	tests := []struct {
		name  string
		valid bool
	}{
		{"default", true},
		{"myproject", true},
		{"my-project", true},
		{"project123", true},
		{"a", true},
		{"", false},           // Empty
		{"123project", false}, // Starts with number
		{"-project", false},   // Starts with hyphen
		{"My-Project", false}, // Uppercase
		{"my_project", false}, // Underscore
		{"my.project", false}, // Dot
		{"my project", false}, // Space
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStoreName(tt.name)
			if tt.valid && err != nil {
				t.Errorf("expected %q to be valid, got error: %v", tt.name, err)
			}
			if !tt.valid && err == nil {
				t.Errorf("expected %q to be invalid, got no error", tt.name)
			}
		})
	}
}

func TestNewStore(t *testing.T) {
	store := NewStore("test")

	if store.Name != "test" {
		t.Errorf("expected name 'test', got %s", store.Name)
	}

	if store.DisplayName != "test" {
		t.Errorf("expected display name 'test', got %s", store.DisplayName)
	}

	if store.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	if store.Config.ChunkSize != 512 {
		t.Errorf("expected chunk size 512, got %d", store.Config.ChunkSize)
	}
}

func TestNewDefaultStore(t *testing.T) {
	store := NewDefaultStore()

	if store.Name != DefaultStoreName {
		t.Errorf("expected name %s, got %s", DefaultStoreName, store.Name)
	}

	if !store.IsDefaultStore() {
		t.Error("expected IsDefaultStore to return true")
	}
}

func TestStoreValidate(t *testing.T) {
	// Valid store
	store := NewStore("test")
	if err := store.Validate(); err != nil {
		t.Errorf("expected valid store, got error: %v", err)
	}

	// Invalid chunk size
	store.Config.ChunkSize = 0
	if err := store.Validate(); err == nil {
		t.Error("expected error for zero chunk size")
	}
	store.Config.ChunkSize = 512

	// Negative overlap
	store.Config.ChunkOverlap = -1
	if err := store.Validate(); err == nil {
		t.Error("expected error for negative overlap")
	}
	store.Config.ChunkOverlap = 64

	// Overlap >= chunk size
	store.Config.ChunkOverlap = 512
	if err := store.Validate(); err == nil {
		t.Error("expected error for overlap >= chunk size")
	}
}

func TestStoreUpdateStats(t *testing.T) {
	store := NewStore("test")
	originalUpdatedAt := store.UpdatedAt

	time.Sleep(time.Millisecond)
	store.UpdateStats(100, 500, 1024000)

	if store.Stats.DocumentCount != 100 {
		t.Errorf("expected document count 100, got %d", store.Stats.DocumentCount)
	}

	if store.Stats.ChunkCount != 500 {
		t.Errorf("expected chunk count 500, got %d", store.Stats.ChunkCount)
	}

	if store.Stats.TotalSize != 1024000 {
		t.Errorf("expected total size 1024000, got %d", store.Stats.TotalSize)
	}

	if store.Stats.LastIndexed.IsZero() {
		t.Error("expected LastIndexed to be set")
	}

	if !store.UpdatedAt.After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated")
	}
}

func TestMemoryStorage(t *testing.T) {
	storage := NewMemoryStorage()
	ctx := context.Background()

	// Test save and load
	store := NewStore("test")
	if err := storage.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	loaded, err := storage.Load("test")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Name != store.Name {
		t.Errorf("expected name %s, got %s", store.Name, loaded.Name)
	}

	// Test exists
	if !storage.Exists("test") {
		t.Error("expected store to exist")
	}

	if storage.Exists("nonexistent") {
		t.Error("expected nonexistent store to not exist")
	}

	// Test load all
	store2 := NewStore("test2")
	if err := storage.Save(store2); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	all, err := storage.LoadAll()
	if err != nil {
		t.Fatalf("failed to load all: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("expected 2 stores, got %d", len(all))
	}

	// Test delete
	if err := storage.Delete("test"); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if storage.Exists("test") {
		t.Error("expected store to be deleted")
	}

	_ = ctx // Unused but shows it's available for context-based operations
}

func TestFileStorage(t *testing.T) {
	// Create temp directory
	tempDir, err := os.MkdirTemp("", "store_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	storage := NewFileStorage(tempDir)

	// Test save and load
	store := NewStore("test")
	store.Description = "Test store"
	if err := storage.Save(store); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	// Check file exists
	filePath := filepath.Join(tempDir, "test.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("expected store file to exist")
	}

	loaded, err := storage.Load("test")
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}

	if loaded.Name != store.Name {
		t.Errorf("expected name %s, got %s", store.Name, loaded.Name)
	}

	if loaded.Description != store.Description {
		t.Errorf("expected description %s, got %s", store.Description, loaded.Description)
	}

	// Test exists
	if !storage.Exists("test") {
		t.Error("expected store to exist")
	}

	// Test load nonexistent
	_, err = storage.Load("nonexistent")
	if err == nil {
		t.Error("expected error loading nonexistent store")
	}

	// Test load all
	store2 := NewStore("test2")
	if err := storage.Save(store2); err != nil {
		t.Fatalf("failed to save: %v", err)
	}

	all, err := storage.LoadAll()
	if err != nil {
		t.Fatalf("failed to load all: %v", err)
	}

	if len(all) != 2 {
		t.Errorf("expected 2 stores, got %d", len(all))
	}

	// Test delete
	if err := storage.Delete("test"); err != nil {
		t.Fatalf("failed to delete: %v", err)
	}

	if storage.Exists("test") {
		t.Error("expected store to be deleted")
	}
}

func TestServiceWithMemoryStorage(t *testing.T) {
	ctx := context.Background()

	svc, err := NewService(nil, ServiceConfig{
		EnsureDefault: true,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Check default store was created
	if !svc.StoreExists(ctx, DefaultStoreName) {
		t.Error("expected default store to exist")
	}

	// Test create store
	newStore := NewStore("myproject")
	if err := svc.CreateStore(ctx, newStore); err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// Test duplicate creation fails
	if err := svc.CreateStore(ctx, newStore); err == nil {
		t.Error("expected error creating duplicate store")
	}

	// Test get store
	retrieved, err := svc.GetStore(ctx, "myproject")
	if err != nil {
		t.Fatalf("failed to get store: %v", err)
	}

	if retrieved.Name != "myproject" {
		t.Errorf("expected name 'myproject', got %s", retrieved.Name)
	}

	// Test list stores
	stores, err := svc.ListStores(ctx)
	if err != nil {
		t.Fatalf("failed to list stores: %v", err)
	}

	if len(stores) != 2 {
		t.Errorf("expected 2 stores, got %d", len(stores))
	}

	// Test update store
	retrieved.Description = "Updated description"
	if err := svc.UpdateStore(ctx, retrieved); err != nil {
		t.Fatalf("failed to update store: %v", err)
	}

	updated, _ := svc.GetStore(ctx, "myproject")
	if updated.Description != "Updated description" {
		t.Error("expected description to be updated")
	}

	// Test delete store
	if err := svc.DeleteStore(ctx, "myproject"); err != nil {
		t.Fatalf("failed to delete store: %v", err)
	}

	if svc.StoreExists(ctx, "myproject") {
		t.Error("expected store to be deleted")
	}

	// Test cannot delete default store
	if err := svc.DeleteStore(ctx, DefaultStoreName); err == nil {
		t.Error("expected error deleting default store")
	}

	// Test ensure store
	ensured, err := svc.EnsureStore(ctx, "newstore")
	if err != nil {
		t.Fatalf("failed to ensure store: %v", err)
	}

	if ensured.Name != "newstore" {
		t.Errorf("expected name 'newstore', got %s", ensured.Name)
	}

	// Ensure again should return existing
	ensuredAgain, err := svc.EnsureStore(ctx, "newstore")
	if err != nil {
		t.Fatalf("failed to ensure store again: %v", err)
	}

	if ensuredAgain.Name != "newstore" {
		t.Error("expected same store to be returned")
	}
}

func TestServiceGetStoreStats(t *testing.T) {
	ctx := context.Background()

	svc, err := NewService(nil, ServiceConfig{
		EnsureDefault: true,
	})
	if err != nil {
		t.Fatalf("failed to create service: %v", err)
	}

	// Update stats
	stats := StoreStats{
		DocumentCount: 100,
		ChunkCount:    500,
		TotalSize:     1024000,
		LastIndexed:   time.Now(),
	}

	if err := svc.UpdateStoreStats(ctx, DefaultStoreName, stats); err != nil {
		t.Fatalf("failed to update stats: %v", err)
	}

	// Get stats
	retrieved, err := svc.GetStoreStats(ctx, DefaultStoreName)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	if retrieved.DocumentCount != 100 {
		t.Errorf("expected document count 100, got %d", retrieved.DocumentCount)
	}
}
