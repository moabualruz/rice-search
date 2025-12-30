package connection

import (
	"context"
	"fmt"
	"sync"

	"github.com/ricesearch/rice-search/internal/bus"
)

// Service provides connection management operations.
type Service struct {
	bus         bus.Bus
	storage     Storage
	connections map[string]*Connection
	mu          sync.RWMutex
}

// ServiceConfig holds configuration for the connection service.
type ServiceConfig struct {
	// StoragePath is the path to connection metadata files.
	StoragePath string
}

// NewService creates a new connection service.
func NewService(eventBus bus.Bus, cfg ServiceConfig) (*Service, error) {
	var storage Storage
	if cfg.StoragePath != "" {
		storage = NewFileStorage(cfg.StoragePath)
	} else {
		storage = NewMemoryStorage()
	}

	svc := &Service{
		bus:         eventBus,
		storage:     storage,
		connections: make(map[string]*Connection),
	}

	// Load existing connections from storage
	if err := svc.loadConnections(); err != nil {
		return nil, fmt.Errorf("failed to load connections: %w", err)
	}

	return svc, nil
}

// loadConnections loads all connections from storage.
func (s *Service) loadConnections() error {
	connections, err := s.storage.LoadAll()
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, conn := range connections {
		s.connections[conn.ID] = conn
	}

	return nil
}

// RegisterConnection registers a new connection or updates existing one.
func (s *Service) RegisterConnection(ctx context.Context, conn *Connection) error {
	if err := conn.Validate(); err != nil {
		return fmt.Errorf("invalid connection: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if connection already exists
	existing, exists := s.connections[conn.ID]
	isNew := !exists

	if exists {
		// Update existing connection
		existing.Name = conn.Name
		existing.PCInfo = conn.PCInfo
		existing.Touch(conn.LastIP)
		if !existing.IsActive && conn.IsActive {
			existing.IsActive = true // reactivate
		}
		conn = existing
	} else {
		// New connection
		s.connections[conn.ID] = conn
	}

	// Save to storage
	if err := s.storage.Save(conn); err != nil {
		// Rollback if new
		if isNew {
			delete(s.connections, conn.ID)
		}
		return fmt.Errorf("failed to save connection: %w", err)
	}

	// Publish event
	if s.bus != nil {
		event := bus.Event{
			Type:    TopicConnectionRegistered,
			Source:  "connection-service",
			Payload: conn,
		}
		_ = s.bus.Publish(ctx, TopicConnectionRegistered, event)
	}

	return nil
}

// GetConnection retrieves a connection by ID.
func (s *Service) GetConnection(ctx context.Context, id string) (*Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	conn, exists := s.connections[id]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", id)
	}

	// Return a copy to prevent external mutations
	connCopy := *conn
	return &connCopy, nil
}

// ListConnections returns connections matching the filter.
func (s *Service) ListConnections(ctx context.Context, filter ConnectionFilter) ([]*Connection, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	connections := make([]*Connection, 0, len(s.connections))
	for _, conn := range s.connections {
		// Apply filters
		if filter.ActiveOnly && !conn.IsActive {
			continue
		}
		// When filtering by store, only include connections with explicit access
		// (not those with access to all stores via nil list)
		if filter.Store != "" && !conn.HasExplicitStoreAccess(filter.Store) {
			continue
		}

		// Return copy
		connCopy := *conn
		connections = append(connections, &connCopy)
	}

	return connections, nil
}

// UpdateLastSeen updates the last seen timestamp and IP for a connection.
func (s *Service) UpdateLastSeen(ctx context.Context, id, ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[id]
	if !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	conn.Touch(ip)

	// Save to storage
	if err := s.storage.Save(conn); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	// Publish event
	if s.bus != nil {
		event := bus.Event{
			Type:   TopicConnectionSeen,
			Source: "connection-service",
			Payload: map[string]interface{}{
				"connection_id": id,
				"ip":            ip,
				"timestamp":     conn.LastSeenAt,
			},
		}
		_ = s.bus.Publish(ctx, TopicConnectionSeen, event)
	}

	return nil
}

// DeleteConnection deletes a connection.
func (s *Service) DeleteConnection(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[id]
	if !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	// Delete from storage
	if err := s.storage.Delete(id); err != nil {
		return fmt.Errorf("failed to delete connection from storage: %w", err)
	}

	delete(s.connections, id)

	// Publish event
	if s.bus != nil {
		event := bus.Event{
			Type:   TopicConnectionDeleted,
			Source: "connection-service",
			Payload: map[string]interface{}{
				"connection_id": id,
				"name":          conn.Name,
			},
		}
		_ = s.bus.Publish(ctx, TopicConnectionDeleted, event)
	}

	return nil
}

// IncrementStats atomically increments connection statistics.
func (s *Service) IncrementStats(ctx context.Context, id string, indexed, searches int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[id]
	if !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	conn.IndexedFiles += indexed
	conn.SearchCount += searches

	// Save to storage
	if err := s.storage.Save(conn); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	return nil
}

// SetActive enables or disables a connection.
func (s *Service) SetActive(ctx context.Context, id string, active bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[id]
	if !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	conn.IsActive = active

	// Save to storage
	if err := s.storage.Save(conn); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	return nil
}

// GrantStoreAccess grants a connection access to a store.
func (s *Service) GrantStoreAccess(ctx context.Context, id, store string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[id]
	if !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	conn.GrantStoreAccess(store)

	// Save to storage
	if err := s.storage.Save(conn); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	return nil
}

// RevokeStoreAccess revokes a connection's access to a store.
func (s *Service) RevokeStoreAccess(ctx context.Context, id, store string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[id]
	if !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	conn.RevokeStoreAccess(store)

	// Save to storage
	if err := s.storage.Save(conn); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	return nil
}

// ConnectionExists checks if a connection exists.
func (s *Service) ConnectionExists(ctx context.Context, id string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, exists := s.connections[id]
	return exists
}

// GetOrCreate gets a connection or creates it if it doesn't exist.
func (s *Service) GetOrCreate(ctx context.Context, name string, pcInfo PCInfo) (*Connection, error) {
	id := GenerateConnectionID(pcInfo)

	// Check if exists first
	s.mu.RLock()
	conn, exists := s.connections[id]
	s.mu.RUnlock()

	if exists {
		// Update last seen
		_ = s.UpdateLastSeen(ctx, id, "")
		return conn, nil
	}

	// Create new connection
	newConn := NewConnection(name, pcInfo)
	if err := s.RegisterConnection(ctx, newConn); err != nil {
		return nil, err
	}

	return newConn, nil
}

// =============================================================================
// Convenience Methods (for web handlers compatibility)
// =============================================================================

// ListAllConnections returns all connections without filtering.
// This is a convenience wrapper for ListConnections with an empty filter.
func (s *Service) ListAllConnections(ctx context.Context) ([]*Connection, error) {
	return s.ListConnections(ctx, ConnectionFilter{})
}

// GetConnectionsForStore returns all connections with explicit access to a store.
func (s *Service) GetConnectionsForStore(ctx context.Context, storeName string) ([]*Connection, error) {
	return s.ListConnections(ctx, ConnectionFilter{
		Store: storeName,
	})
}

// RenameConnection renames a connection.
func (s *Service) RenameConnection(ctx context.Context, id, newName string) error {
	if err := ValidateConnectionName(newName); err != nil {
		return fmt.Errorf("invalid connection name: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	conn, exists := s.connections[id]
	if !exists {
		return fmt.Errorf("connection %s not found", id)
	}

	oldName := conn.Name
	conn.Name = newName

	// Save to storage
	if err := s.storage.Save(conn); err != nil {
		// Rollback on error
		conn.Name = oldName
		return fmt.Errorf("failed to save connection: %w", err)
	}

	// Publish event
	if s.bus != nil {
		event := bus.Event{
			Type:   "connection.renamed",
			Source: "connection-service",
			Payload: map[string]interface{}{
				"connection_id": id,
				"old_name":      oldName,
				"new_name":      newName,
			},
		}
		_ = s.bus.Publish(ctx, "connection.renamed", event)
	}

	return nil
}

// Close cleans up connection service resources.
func (s *Service) Close() error {
	// Stop any background goroutines if any (currently none)
	// Flush any pending data (currently none)

	// Close storage backend if it implements io.Closer
	if closer, ok := s.storage.(interface{ Close() error }); ok {
		if err := closer.Close(); err != nil {
			return fmt.Errorf("failed to close storage: %w", err)
		}
	}

	return nil
}
