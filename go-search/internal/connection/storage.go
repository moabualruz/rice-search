package connection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Storage is the interface for connection persistence.
type Storage interface {
	// Save saves a connection to persistent storage.
	Save(conn *Connection) error

	// Load loads a connection by ID.
	Load(id string) (*Connection, error)

	// LoadAll loads all connections.
	LoadAll() ([]*Connection, error)

	// Delete deletes a connection from storage.
	Delete(id string) error

	// Exists checks if a connection exists in storage.
	Exists(id string) bool
}

// MemoryStorage stores connections in memory (for testing).
type MemoryStorage struct {
	connections map[string]*Connection
	mu          sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		connections: make(map[string]*Connection),
	}
}

func (m *MemoryStorage) Save(conn *Connection) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Deep copy to avoid mutations
	connCopy := *conn
	m.connections[conn.ID] = &connCopy
	return nil
}

func (m *MemoryStorage) Load(id string) (*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	conn, exists := m.connections[id]
	if !exists {
		return nil, fmt.Errorf("connection %s not found", id)
	}

	// Return copy
	connCopy := *conn
	return &connCopy, nil
}

func (m *MemoryStorage) LoadAll() ([]*Connection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	connections := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		connCopy := *conn
		connections = append(connections, &connCopy)
	}
	return connections, nil
}

func (m *MemoryStorage) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.connections, id)
	return nil
}

func (m *MemoryStorage) Exists(id string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.connections[id]
	return exists
}

// FileStorage stores connections in JSON files.
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

func (f *FileStorage) connectionPath(id string) string {
	return filepath.Join(f.basePath, id+".json")
}

func (f *FileStorage) Save(conn *Connection) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Ensure directory exists
	if err := os.MkdirAll(f.basePath, 0755); err != nil {
		return fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(conn, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal connection: %w", err)
	}

	// Write to file
	path := f.connectionPath(conn.ID)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write connection file: %w", err)
	}

	return nil
}

func (f *FileStorage) Load(id string) (*Connection, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.connectionPath(id)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("connection %s not found", id)
		}
		return nil, fmt.Errorf("failed to read connection file: %w", err)
	}

	var conn Connection
	if err := json.Unmarshal(data, &conn); err != nil {
		return nil, fmt.Errorf("failed to unmarshal connection: %w", err)
	}

	return &conn, nil
}

func (f *FileStorage) LoadAll() ([]*Connection, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	// Ensure directory exists
	if _, err := os.Stat(f.basePath); os.IsNotExist(err) {
		return []*Connection{}, nil
	}

	// List all JSON files
	entries, err := os.ReadDir(f.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read storage directory: %w", err)
	}

	var connections []*Connection
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(f.basePath, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Skip files we can't read
		}

		var conn Connection
		if err := json.Unmarshal(data, &conn); err != nil {
			continue // Skip invalid files
		}

		connections = append(connections, &conn)
	}

	return connections, nil
}

func (f *FileStorage) Delete(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	path := f.connectionPath(id)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete connection file: %w", err)
	}

	return nil
}

func (f *FileStorage) Exists(id string) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()

	path := f.connectionPath(id)
	_, err := os.Stat(path)
	return err == nil
}
