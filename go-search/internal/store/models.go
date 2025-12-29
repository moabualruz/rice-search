// Package store provides store management for Rice Search.
// A store is an isolated search index (like a database).
package store

import (
	"fmt"
	"regexp"
	"time"
)

// Store represents an isolated search index.
type Store struct {
	Name        string      `json:"name" yaml:"name"`
	DisplayName string      `json:"display_name" yaml:"display_name"`
	Description string      `json:"description" yaml:"description"`
	Config      StoreConfig `json:"config" yaml:"config"`
	Stats       StoreStats  `json:"stats" yaml:"stats"`
	CreatedAt   time.Time   `json:"created_at" yaml:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at" yaml:"updated_at"`
}

// StoreConfig holds configuration for a store.
type StoreConfig struct {
	// EmbedModel is the dense embedding model name.
	EmbedModel string `json:"embed_model" yaml:"embed_model"`

	// SparseModel is the sparse encoding model name.
	SparseModel string `json:"sparse_model" yaml:"sparse_model"`

	// ChunkSize is the target chunk size in tokens.
	ChunkSize int `json:"chunk_size" yaml:"chunk_size"`

	// ChunkOverlap is the overlap between chunks in tokens.
	ChunkOverlap int `json:"chunk_overlap" yaml:"chunk_overlap"`
}

// StoreStats holds statistics for a store.
type StoreStats struct {
	// DocumentCount is the number of source files.
	DocumentCount int64 `json:"document_count" yaml:"document_count"`

	// ChunkCount is the number of indexed chunks.
	ChunkCount int64 `json:"chunk_count" yaml:"chunk_count"`

	// TotalSize is the total content size in bytes.
	TotalSize int64 `json:"total_size" yaml:"total_size"`

	// LastIndexed is when the store was last indexed.
	LastIndexed time.Time `json:"last_indexed" yaml:"last_indexed"`
}

// DefaultStoreName is the name of the default store.
const DefaultStoreName = "default"

// DefaultStoreConfig returns sensible defaults for a store configuration.
func DefaultStoreConfig() StoreConfig {
	return StoreConfig{
		EmbedModel:   "jina-embeddings-v3",
		SparseModel:  "splade-v3",
		ChunkSize:    512,
		ChunkOverlap: 64,
	}
}

// NewStore creates a new store with the given name and default configuration.
func NewStore(name string) *Store {
	now := time.Now()
	return &Store{
		Name:        name,
		DisplayName: name,
		Description: "",
		Config:      DefaultStoreConfig(),
		Stats:       StoreStats{},
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NewDefaultStore creates the default store.
func NewDefaultStore() *Store {
	store := NewStore(DefaultStoreName)
	store.DisplayName = "Default Store"
	store.Description = "The default search index"
	return store
}

// Store name validation rules
var (
	// storeNameRegex validates store names: lowercase alphanumeric + hyphens, starting with a letter
	storeNameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	// MaxStoreNameLength is the maximum length of a store name
	MaxStoreNameLength = 64
)

// ValidateStoreName validates a store name.
func ValidateStoreName(name string) error {
	if name == "" {
		return fmt.Errorf("store name cannot be empty")
	}

	if len(name) > MaxStoreNameLength {
		return fmt.Errorf("store name cannot exceed %d characters", MaxStoreNameLength)
	}

	if !storeNameRegex.MatchString(name) {
		return fmt.Errorf("store name must be lowercase alphanumeric with hyphens, starting with a letter")
	}

	return nil
}

// IsDefaultStore returns true if this is the default store.
func (s *Store) IsDefaultStore() bool {
	return s.Name == DefaultStoreName
}

// Validate validates the store configuration.
func (s *Store) Validate() error {
	if err := ValidateStoreName(s.Name); err != nil {
		return err
	}

	if s.Config.ChunkSize <= 0 {
		return fmt.Errorf("chunk_size must be positive")
	}

	if s.Config.ChunkOverlap < 0 {
		return fmt.Errorf("chunk_overlap cannot be negative")
	}

	if s.Config.ChunkOverlap >= s.Config.ChunkSize {
		return fmt.Errorf("chunk_overlap must be less than chunk_size")
	}

	return nil
}

// Touch updates the UpdatedAt timestamp.
func (s *Store) Touch() {
	s.UpdatedAt = time.Now()
}

// UpdateStats updates the store statistics.
func (s *Store) UpdateStats(documentCount, chunkCount, totalSize int64) {
	s.Stats.DocumentCount = documentCount
	s.Stats.ChunkCount = chunkCount
	s.Stats.TotalSize = totalSize
	s.Stats.LastIndexed = time.Now()
	s.Touch()
}
