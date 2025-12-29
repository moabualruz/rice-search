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

// ApplyToConfig applies admin settings overrides to the application config.
// This implements the precedence: Admin Settings > Config File > Env Vars > Defaults.
// Call this after loading config to apply any admin overrides.
func (s *Service) ApplyToConfig(cfg interface{}) error {
	s.mu.RLock()
	adminCfg := s.config
	s.mu.RUnlock()

	// Use type assertion to access the config struct
	// We use interface{} to avoid import cycles with config package
	switch c := cfg.(type) {
	case ConfigApplier:
		return c.ApplyRuntimeSettings(adminCfg)
	default:
		// For basic configs, use reflection or direct field mapping
		return s.applyBasicOverrides(cfg, adminCfg)
	}
}

// ConfigApplier is an interface for configs that can apply runtime settings.
type ConfigApplier interface {
	ApplyRuntimeSettings(RuntimeConfig) error
}

// applyBasicOverrides applies common settings via JSON marshal/unmarshal.
func (s *Service) applyBasicOverrides(cfg interface{}, adminCfg RuntimeConfig) error {
	// This is a no-op for now - specific implementations should use ConfigApplier
	// or the GetEffective* methods below
	return nil
}

// GetEffectiveHost returns the effective server host (admin override or default).
func (s *Service) GetEffectiveHost(defaultHost string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.ServerHost != "" {
		return s.config.ServerHost
	}
	return defaultHost
}

// GetEffectivePort returns the effective server port (admin override or default).
func (s *Service) GetEffectivePort(defaultPort int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.ServerPort > 0 {
		return s.config.ServerPort
	}
	return defaultPort
}

// GetEffectiveLogLevel returns the effective log level (admin override or default).
func (s *Service) GetEffectiveLogLevel(defaultLevel string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.LogLevel != "" {
		return s.config.LogLevel
	}
	return defaultLevel
}

// GetEffectiveQdrantURL returns the effective Qdrant URL (admin override or default).
func (s *Service) GetEffectiveQdrantURL(defaultURL string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.config.QdrantURL != "" {
		return s.config.QdrantURL
	}
	return defaultURL
}

// GetEffectiveSearchSettings returns effective search settings.
func (s *Service) GetEffectiveSearchSettings() (topK int, rerank, dedup, diversity bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.DefaultTopK,
		s.config.DefaultRerank,
		s.config.DefaultDedup,
		s.config.DefaultDiversity
}

// GetEffectiveMLSettings returns effective ML settings.
func (s *Service) GetEffectiveMLSettings() (embedGPU, rerankGPU, queryGPU, queryEnabled bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.EmbedGPU,
		s.config.RerankGPU,
		s.config.QueryGPU,
		s.config.QueryEnabled
}

// IsEmpty returns true if settings have not been configured yet.
func (s *Service) IsEmpty() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config.Version == 0
}

// ExportYAML exports settings as YAML bytes.
func (s *Service) ExportYAML(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return yaml.Marshal(s.config)
}

// ExportJSON exports settings as JSON bytes.
func (s *Service) ExportJSON(ctx context.Context) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return json.MarshalIndent(s.config, "", "  ")
}

// ImportYAML imports settings from YAML bytes.
func (s *Service) ImportYAML(ctx context.Context, data []byte, changedBy string) error {
	var cfg RuntimeConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid YAML: %w", err)
	}
	return s.Update(ctx, cfg, changedBy)
}

// ImportJSON imports settings from JSON bytes.
func (s *Service) ImportJSON(ctx context.Context, data []byte, changedBy string) error {
	var cfg RuntimeConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return s.Update(ctx, cfg, changedBy)
}

// ValidationError represents a settings validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult holds the result of settings validation.
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validate validates the runtime configuration and returns any errors.
func (cfg *RuntimeConfig) Validate() ValidationResult {
	result := ValidationResult{Valid: true}

	// Server settings
	if cfg.ServerPort < 1 || cfg.ServerPort > 65535 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "server_port",
			Message: "must be between 1 and 65535",
		})
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if cfg.LogLevel != "" && !validLogLevels[cfg.LogLevel] {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "log_level",
			Message: "must be one of: debug, info, warn, error",
		})
	}

	validLogFormats := map[string]bool{"text": true, "json": true}
	if cfg.LogFormat != "" && !validLogFormats[cfg.LogFormat] {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "log_format",
			Message: "must be one of: text, json",
		})
	}

	// ML settings
	if cfg.BatchSize < 1 || cfg.BatchSize > 256 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "batch_size",
			Message: "must be between 1 and 256",
		})
	}

	if cfg.MaxConcurrent < 1 || cfg.MaxConcurrent > 32 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "max_concurrent",
			Message: "must be between 1 and 32",
		})
	}

	// Qdrant settings
	if cfg.QdrantTimeout < 1000 || cfg.QdrantTimeout > 300000 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "qdrant_timeout",
			Message: "must be between 1000 and 300000 (1s to 5min)",
		})
	}

	// Search settings
	if cfg.DefaultTopK < 1 || cfg.DefaultTopK > 1000 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "default_top_k",
			Message: "must be between 1 and 1000",
		})
	}

	if cfg.DedupThreshold < 0 || cfg.DedupThreshold > 1 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "dedup_threshold",
			Message: "must be between 0 and 1",
		})
	}

	if cfg.DiversityLambda < 0 || cfg.DiversityLambda > 1 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "diversity_lambda",
			Message: "must be between 0 and 1",
		})
	}

	if cfg.RerankCandidates < 1 || cfg.RerankCandidates > 500 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "rerank_candidates",
			Message: "must be between 1 and 500",
		})
	}

	if cfg.MaxChunksPerFile < 1 || cfg.MaxChunksPerFile > 100 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "max_chunks_per_file",
			Message: "must be between 1 and 100",
		})
	}

	if cfg.SparseWeight < 0 || cfg.SparseWeight > 1 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "sparse_weight",
			Message: "must be between 0 and 1",
		})
	}

	if cfg.DenseWeight < 0 || cfg.DenseWeight > 1 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "dense_weight",
			Message: "must be between 0 and 1",
		})
	}

	// Index settings
	if cfg.ChunkSize < 64 || cfg.ChunkSize > 4096 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "chunk_size",
			Message: "must be between 64 and 4096",
		})
	}

	if cfg.ChunkOverlap < 0 || cfg.ChunkOverlap >= cfg.ChunkSize {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "chunk_overlap",
			Message: "must be >= 0 and less than chunk_size",
		})
	}

	if cfg.MaxFileSize < 1024 || cfg.MaxFileSize > 100*1024*1024 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "max_file_size",
			Message: "must be between 1KB and 100MB",
		})
	}

	// Connection settings
	if cfg.MaxInactiveHours < 1 || cfg.MaxInactiveHours > 8760 {
		result.Errors = append(result.Errors, ValidationError{
			Field:   "max_inactive_hours",
			Message: "must be between 1 and 8760 (1 year)",
		})
	}

	result.Valid = len(result.Errors) == 0
	return result
}

// ValidateAndUpdate validates and updates settings. Returns validation errors if any.
func (s *Service) ValidateAndUpdate(ctx context.Context, cfg RuntimeConfig, changedBy string) (ValidationResult, error) {
	result := cfg.Validate()
	if !result.Valid {
		return result, fmt.Errorf("validation failed with %d errors", len(result.Errors))
	}
	return result, s.Update(ctx, cfg, changedBy)
}
