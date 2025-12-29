// Package settings provides a runtime settings storage service.
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// RuntimeConfig represents all configurable runtime settings.
// This matches the web.RuntimeConfig structure.
type RuntimeConfig struct {
	// Server settings
	ServerHost string `json:"server_host" yaml:"server_host"`
	ServerPort int    `json:"server_port" yaml:"server_port"`
	LogLevel   string `json:"log_level" yaml:"log_level"`
	LogFormat  string `json:"log_format" yaml:"log_format"`

	// ML Settings
	EmbedModel    string `json:"embed_model" yaml:"embed_model"`
	RerankModel   string `json:"rerank_model" yaml:"rerank_model"`
	QueryModel    string `json:"query_model" yaml:"query_model"`
	EmbedGPU      bool   `json:"embed_gpu" yaml:"embed_gpu"`
	RerankGPU     bool   `json:"rerank_gpu" yaml:"rerank_gpu"`
	QueryGPU      bool   `json:"query_gpu" yaml:"query_gpu"`
	QueryEnabled  bool   `json:"query_enabled" yaml:"query_enabled"`
	BatchSize     int    `json:"batch_size" yaml:"batch_size"`
	MaxConcurrent int    `json:"max_concurrent" yaml:"max_concurrent"`

	// Qdrant Settings
	QdrantURL        string `json:"qdrant_url" yaml:"qdrant_url"`
	QdrantCollection string `json:"qdrant_collection" yaml:"qdrant_collection"`
	QdrantTimeout    int    `json:"qdrant_timeout" yaml:"qdrant_timeout"`

	// Search Settings
	DefaultTopK      int     `json:"default_top_k" yaml:"default_top_k"`
	DefaultRerank    bool    `json:"default_rerank" yaml:"default_rerank"`
	DefaultDedup     bool    `json:"default_dedup" yaml:"default_dedup"`
	DefaultDiversity bool    `json:"default_diversity" yaml:"default_diversity"`
	DedupThreshold   float32 `json:"dedup_threshold" yaml:"dedup_threshold"`
	DiversityLambda  float32 `json:"diversity_lambda" yaml:"diversity_lambda"`
	RerankCandidates int     `json:"rerank_candidates" yaml:"rerank_candidates"`
	MaxChunksPerFile int     `json:"max_chunks_per_file" yaml:"max_chunks_per_file"`
	SparseWeight     float32 `json:"sparse_weight" yaml:"sparse_weight"`
	DenseWeight      float32 `json:"dense_weight" yaml:"dense_weight"`

	// Index Settings
	ChunkSize       int    `json:"chunk_size" yaml:"chunk_size"`
	ChunkOverlap    int    `json:"chunk_overlap" yaml:"chunk_overlap"`
	MaxFileSize     int64  `json:"max_file_size" yaml:"max_file_size"`
	ExcludePatterns string `json:"exclude_patterns" yaml:"exclude_patterns"`
	SupportedLangs  string `json:"supported_langs" yaml:"supported_langs"`

	// Connection Settings
	ConnectionEnabled bool `json:"connection_enabled" yaml:"connection_enabled"`
	MaxInactiveHours  int  `json:"max_inactive_hours" yaml:"max_inactive_hours"`

	// Metadata
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
	Version   int       `json:"version" yaml:"version"`
}

// DefaultConfig returns sensible defaults for all settings.
func DefaultConfig() RuntimeConfig {
	return RuntimeConfig{
		// Server
		ServerHost: "0.0.0.0",
		ServerPort: 8080,
		LogLevel:   "info",
		LogFormat:  "text",

		// ML - GPU enabled by default
		EmbedModel:    "jinaai/jina-embeddings-v2-base-code",
		RerankModel:   "jinaai/jina-reranker-v2-base-multilingual",
		QueryModel:    "Salesforce/codet5p-110m-embedding",
		EmbedGPU:      true,
		RerankGPU:     true,
		QueryGPU:      false, // Query understanding is CPU by default
		QueryEnabled:  true,
		BatchSize:     32,
		MaxConcurrent: 4,

		// Qdrant
		QdrantURL:        "http://localhost:6333",
		QdrantCollection: "rice_search",
		QdrantTimeout:    30000,

		// Search
		DefaultTopK:      20,
		DefaultRerank:    true,
		DefaultDedup:     true,
		DefaultDiversity: true,
		DedupThreshold:   0.85,
		DiversityLambda:  0.7,
		RerankCandidates: 50,
		MaxChunksPerFile: 5,
		SparseWeight:     0.5,
		DenseWeight:      0.5,

		// Index
		ChunkSize:       512,
		ChunkOverlap:    128,
		MaxFileSize:     10 * 1024 * 1024, // 10MB
		ExcludePatterns: "node_modules,vendor,.git",
		SupportedLangs:  "go,python,javascript,typescript,rust",

		// Connection
		ConnectionEnabled: true,
		MaxInactiveHours:  168, // 7 days

		// Metadata
		UpdatedAt: time.Now(),
		Version:   1,
	}
}

// SettingsChangedEvent is published when settings are updated.
type SettingsChangedEvent struct {
	OldConfig RuntimeConfig `json:"old_config"`
	NewConfig RuntimeConfig `json:"new_config"`
	ChangedBy string        `json:"changed_by"` // "admin", "api", etc.
}

// Service manages runtime settings with persistence and event publishing.
type Service struct {
	mu          sync.RWMutex
	config      RuntimeConfig
	storagePath string
	eventBus    bus.Bus
	log         *logger.Logger
}

// ServiceConfig configures the settings service.
type ServiceConfig struct {
	// StoragePath is the directory where settings are persisted.
	StoragePath string

	// LoadDefaults determines whether to populate with defaults if no file exists.
	LoadDefaults bool
}

// NewService creates a new settings service.
func NewService(cfg ServiceConfig, eventBus bus.Bus, log *logger.Logger) (*Service, error) {
	s := &Service{
		storagePath: cfg.StoragePath,
		eventBus:    eventBus,
		log:         log,
	}

	// Ensure storage directory exists
	if err := os.MkdirAll(cfg.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create settings directory: %w", err)
	}

	// Load existing settings or use defaults
	if err := s.load(); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load settings: %w", err)
		}
		// No file exists, use defaults
		if cfg.LoadDefaults {
			s.config = DefaultConfig()
			if err := s.save(); err != nil {
				s.log.Warn("Failed to save default settings", "error", err)
			}
		}
	}

	return s, nil
}

// Get returns the current runtime configuration.
func (s *Service) Get(ctx context.Context) RuntimeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// Update updates the runtime configuration and persists it.
func (s *Service) Update(ctx context.Context, cfg RuntimeConfig, changedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldConfig := s.config

	// Update metadata
	cfg.UpdatedAt = time.Now()
	cfg.Version = oldConfig.Version + 1

	// Persist
	if err := s.saveUnlocked(cfg); err != nil {
		return fmt.Errorf("failed to save settings: %w", err)
	}

	s.config = cfg

	// Publish event
	if s.eventBus != nil {
		event := bus.Event{
			Type:      "settings.changed",
			Source:    "settings",
			Timestamp: time.Now().UnixNano(),
			Payload: SettingsChangedEvent{
				OldConfig: oldConfig,
				NewConfig: cfg,
				ChangedBy: changedBy,
			},
		}
		if err := s.eventBus.Publish(ctx, "settings.changed", event); err != nil {
			s.log.Warn("Failed to publish settings changed event", "error", err)
		}
	}

	s.log.Info("Settings updated",
		"version", cfg.Version,
		"changed_by", changedBy,
	)

	return nil
}

// UpdateField updates a single field in the configuration.
func (s *Service) UpdateField(ctx context.Context, field string, value interface{}, changedBy string) error {
	s.mu.Lock()
	cfg := s.config
	s.mu.Unlock()

	// Use JSON marshaling/unmarshaling to update the field dynamically
	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	var cfgMap map[string]interface{}
	if err := json.Unmarshal(data, &cfgMap); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cfgMap[field] = value

	data, err = json.Marshal(cfgMap)
	if err != nil {
		return fmt.Errorf("failed to marshal updated config: %w", err)
	}

	var newCfg RuntimeConfig
	if err := json.Unmarshal(data, &newCfg); err != nil {
		return fmt.Errorf("failed to unmarshal updated config: %w", err)
	}

	return s.Update(ctx, newCfg, changedBy)
}

// Reset resets the configuration to defaults.
func (s *Service) Reset(ctx context.Context, changedBy string) error {
	return s.Update(ctx, DefaultConfig(), changedBy)
}

// settingsFile returns the path to the settings file.
func (s *Service) settingsFile() string {
	return filepath.Join(s.storagePath, "settings.yaml")
}

// load loads settings from disk.
func (s *Service) load() error {
	data, err := os.ReadFile(s.settingsFile())
	if err != nil {
		return err
	}

	var cfg RuntimeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse settings: %w", err)
	}

	s.config = cfg
	return nil
}

// save saves settings to disk.
func (s *Service) save() error {
	s.mu.RLock()
	cfg := s.config
	s.mu.RUnlock()
	return s.saveUnlocked(cfg)
}

// saveUnlocked saves settings without acquiring lock (caller must hold lock).
func (s *Service) saveUnlocked(cfg RuntimeConfig) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	return os.WriteFile(s.settingsFile(), data, 0644)
}

// GetVersion returns the current settings version.
func (s *Service) GetVersion(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Version
}

// GetUpdatedAt returns when settings were last updated.
func (s *Service) GetUpdatedAt(ctx context.Context) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.UpdatedAt
}
