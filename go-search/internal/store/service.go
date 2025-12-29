package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/ricesearch/rice-search/internal/qdrant"
)

// Service provides store management operations.
type Service struct {
	qdrant  *qdrant.Client
	storage Storage
	stores  map[string]*Store
	mu      sync.RWMutex
}

// ServiceConfig holds configuration for the store service.
type ServiceConfig struct {
	// StoragePath is the path to store metadata files.
	StoragePath string

	// EnsureDefault creates the default store if it doesn't exist.
	EnsureDefault bool
}

// NewService creates a new store service.
func NewService(qdrantClient *qdrant.Client, cfg ServiceConfig) (*Service, error) {
	var storage Storage
	if cfg.StoragePath != "" {
		storage = NewFileStorage(cfg.StoragePath)
	} else {
		storage = NewMemoryStorage()
	}

	svc := &Service{
		qdrant:  qdrantClient,
		storage: storage,
		stores:  make(map[string]*Store),
	}

	// Load existing stores from storage
	if err := svc.loadStores(); err != nil {
		return nil, fmt.Errorf("failed to load stores: %w", err)
	}

	// Ensure default store exists
	if cfg.EnsureDefault {
		if _, exists := svc.stores[DefaultStoreName]; !exists {
			if err := svc.CreateStore(context.Background(), NewDefaultStore()); err != nil {
				return nil, fmt.Errorf("failed to create default store: %w", err)
			}
		}
	}

	return svc, nil
}

// loadStores loads all stores from storage.
func (s *Service) loadStores() error {
	stores, err := s.storage.LoadAll()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, store := range stores {
		s.stores[store.Name] = store
	}

	return nil
}

// CreateStore creates a new store.
func (s *Service) CreateStore(ctx context.Context, store *Store) error {
	if err := store.Validate(); err != nil {
		return fmt.Errorf("invalid store: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if store already exists
	if _, exists := s.stores[store.Name]; exists {
		return fmt.Errorf("store %s already exists", store.Name)
	}

	// Create Qdrant collection
	if s.qdrant != nil {
		cfg := qdrant.DefaultCollectionConfig(store.Name)
		if err := s.qdrant.CreateCollection(ctx, cfg); err != nil {
			return fmt.Errorf("failed to create qdrant collection: %w", err)
		}
	}

	// Save to storage
	if err := s.storage.Save(store); err != nil {
		// Rollback: delete Qdrant collection
		if s.qdrant != nil {
			_ = s.qdrant.DeleteCollection(ctx, store.Name)
		}
		return fmt.Errorf("failed to save store: %w", err)
	}

	s.stores[store.Name] = store
	return nil
}

// GetStore retrieves a store by name.
func (s *Service) GetStore(ctx context.Context, name string) (*Store, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	store, exists := s.stores[name]
	if !exists {
		return nil, fmt.Errorf("store %s not found", name)
	}

	return store, nil
}

// ListStores returns all stores.
func (s *Service) ListStores(ctx context.Context) ([]*Store, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stores := make([]*Store, 0, len(s.stores))
	for _, store := range s.stores {
		stores = append(stores, store)
	}

	return stores, nil
}

// UpdateStore updates a store's configuration.
func (s *Service) UpdateStore(ctx context.Context, store *Store) error {
	if err := store.Validate(); err != nil {
		return fmt.Errorf("invalid store: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.stores[store.Name]
	if !exists {
		return fmt.Errorf("store %s not found", store.Name)
	}

	// Preserve creation time
	store.CreatedAt = existing.CreatedAt
	store.Touch()

	// Save to storage
	if err := s.storage.Save(store); err != nil {
		return fmt.Errorf("failed to save store: %w", err)
	}

	s.stores[store.Name] = store
	return nil
}

// DeleteStore deletes a store.
func (s *Service) DeleteStore(ctx context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, exists := s.stores[name]
	if !exists {
		return fmt.Errorf("store %s not found", name)
	}

	// Prevent deletion of default store
	if store.IsDefaultStore() {
		return fmt.Errorf("cannot delete the default store")
	}

	// Delete Qdrant collection
	if s.qdrant != nil {
		if err := s.qdrant.DeleteCollection(ctx, name); err != nil {
			return fmt.Errorf("failed to delete qdrant collection: %w", err)
		}
	}

	// Delete from storage
	if err := s.storage.Delete(name); err != nil {
		return fmt.Errorf("failed to delete store from storage: %w", err)
	}

	delete(s.stores, name)
	return nil
}

// GetStoreStats returns the current statistics for a store.
func (s *Service) GetStoreStats(ctx context.Context, name string) (*StoreStats, error) {
	s.mu.RLock()
	store, exists := s.stores[name]
	s.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("store %s not found", name)
	}

	// Get live stats from Qdrant if available
	if s.qdrant != nil {
		info, err := s.qdrant.GetCollectionInfo(ctx, name)
		if err == nil {
			// Update stats from Qdrant
			store.Stats.ChunkCount = int64(info.PointsCount)
		}
	}

	return &store.Stats, nil
}

// UpdateStoreStats updates the statistics for a store.
func (s *Service) UpdateStoreStats(ctx context.Context, name string, stats StoreStats) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	store, exists := s.stores[name]
	if !exists {
		return fmt.Errorf("store %s not found", name)
	}

	store.Stats = stats
	store.Touch()

	// Save to storage
	if err := s.storage.Save(store); err != nil {
		return fmt.Errorf("failed to save store: %w", err)
	}

	return nil
}

// StoreExists checks if a store exists.
func (s *Service) StoreExists(ctx context.Context, name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.stores[name]
	return exists
}

// EnsureStore creates a store if it doesn't exist.
func (s *Service) EnsureStore(ctx context.Context, name string) (*Store, error) {
	// Check if exists first
	s.mu.RLock()
	store, exists := s.stores[name]
	s.mu.RUnlock()

	if exists {
		return store, nil
	}

	// Create new store
	newStore := NewStore(name)
	if err := s.CreateStore(ctx, newStore); err != nil {
		return nil, err
	}

	return newStore, nil
}
