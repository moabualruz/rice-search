package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Storage is the interface for store persistence.
type Storage interface {
	// Save saves a store to persistent storage.
	Save(store *Store) error

	// Load loads a store by name.
	Load(name string) (*Store, error)

	// LoadAll loads all stores.
	LoadAll() ([]*Store, error)

	// Delete deletes a store from storage.
	Delete(name string) error

	// Exists checks if a store exists in storage.
	Exists(name string) bool
}

// MemoryStorage stores metadata in memory (for testing).
type MemoryStorage struct {
	stores map[string]*Store
	mu     sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		stores: make(map[string]*Store),
	}
}

func (m *MemoryStorage) Save(store *Store) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy to avoid mutations
	storeCopy := *store
	m.stores[store.Name] = &storeCopy
	return nil
}

func (m *MemoryStorage) Load(name string) (*Store, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	store, exists := m.stores[name]
	if !exists {
		return nil, fmt.Errorf("store %s not found", name)
	}

	// Return copy
	storeCopy := *store
	return &storeCopy, nil
}

func (m *MemoryStorage) LoadAll() ([]*Store, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stores := make([]*Store, 0, len(m.stores))
	for _, store := range m.stores {
		storeCopy := *store
		stores = append(stores, &storeCopy)
	}
	return stores, nil
}

func (m *MemoryStorage) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.stores, name)
	return nil
}

func (m *MemoryStorage) Exists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.stores[name]
	return exists
}

// FileStorage stores metadata in JSON files.
type FileStorage struct {
	basePath string
	mu       sync.RWMutex
}

// NewFileStorage creates a new file-based storage.
func NewFileStorage(basePath string) *FileStorage {
	return &FileStorage{
		basePath: basePath,
	}
}

func (f *FileStorage) storePath(name string) string {
	return filepath.Join(f.basePath, name+".json")
}

func (f *FileStorage) Save(store *Store) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(f.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal store: %w", err)
	}

	// Write to file
	path := f.storePath(store.Name)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write store file: %w", err)
	}

	return nil
}

func (f *FileStorage) Load(name string) (*Store, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.storePath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("store %s not found", name)
		}
		return nil, fmt.Errorf("failed to read store file: %w", err)
	}

	var store Store
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, fmt.Errorf("failed to unmarshal store: %w", err)
	}

	return &store, nil
}

func (f *FileStorage) LoadAll() ([]*Store, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Ensure directory exists
	if _, err := os.Stat(f.basePath); os.IsNotExist(err) {
		return []*Store{}, nil
	}

	// List all JSON files
	entries, err := os.ReadDir(f.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	var stores []*Store
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(f.basePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var store Store
		if err := json.Unmarshal(data, &store); err != nil {
			continue // Skip invalid files
		}

		stores = append(stores, &store)
	}

	return stores, nil
}

func (f *FileStorage) Delete(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.storePath(name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete store file: %w", err)
	}

	return nil
}

func (f *FileStorage) Exists(name string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.storePath(name)
	_, err := os.Stat(path)
	return err == nil
}
