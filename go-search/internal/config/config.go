// Package config handles configuration loading and validation.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"gopkg.in/yaml.v3"
)

// Config holds all application configuration.
type Config struct {
	// Server configuration
	Host string `envconfig:"RICE_HOST" yaml:"host"`
	Port int    `envconfig:"RICE_PORT" yaml:"port"`

	// Feature flags
	EnableWeb bool `envconfig:"RICE_ENABLE_WEB" yaml:"enable_web"`
	EnableML  bool `envconfig:"RICE_ENABLE_ML" yaml:"enable_ml"`

	// Qdrant configuration
	Qdrant QdrantConfig `yaml:"qdrant"`

	// ML configuration
	ML MLConfig `yaml:"ml"`

	// Connection tracking configuration
	Connection ConnectionConfig `yaml:"connection"`

	// Model registry configuration
	Models ModelsConfig `yaml:"models"`

	// Cache configuration
	Cache CacheConfig `yaml:"cache"`

	// Bus configuration
	Bus BusConfig `yaml:"bus"`

	// Index configuration
	Index IndexConfig `yaml:"index"`

	// Search configuration
	Search SearchConfig `yaml:"search"`

	// Logging configuration
	Log LogConfig `yaml:"log"`

	// Security configuration
	Security SecurityConfig `yaml:"security"`

	// Observability configuration
	Observability ObservabilityConfig `yaml:"observability"`
}

// QdrantConfig holds Qdrant connection settings.
type QdrantConfig struct {
	URL              string `envconfig:"QDRANT_URL" yaml:"url"`
	APIKey           string `envconfig:"QDRANT_API_KEY" yaml:"api_key"`
	CollectionPrefix string `envconfig:"QDRANT_COLLECTION_PREFIX" yaml:"collection_prefix"`
}

// MLConfig holds ML inference settings.
type MLConfig struct {
	Device          string `envconfig:"RICE_ML_DEVICE" yaml:"device"`
	CUDADevice      int    `envconfig:"RICE_ML_CUDA_DEVICE" yaml:"cuda_device"`
	EmbedModel      string `envconfig:"RICE_EMBED_MODEL" yaml:"embed_model"`
	SparseModel     string `envconfig:"RICE_SPARSE_MODEL" yaml:"sparse_model"`
	RerankModel     string `envconfig:"RICE_RERANK_MODEL" yaml:"rerank_model"`
	QueryModel      string `envconfig:"RICE_QUERY_MODEL" yaml:"query_model"` // Query understanding model
	EmbedDim        int    `envconfig:"RICE_EMBED_DIM" yaml:"embed_dim"`
	EmbedBatchSize  int    `envconfig:"RICE_EMBED_BATCH_SIZE" yaml:"embed_batch_size"`
	SparseBatchSize int    `envconfig:"RICE_SPARSE_BATCH_SIZE" yaml:"sparse_batch_size"`
	RerankBatchSize int    `envconfig:"RICE_RERANK_BATCH_SIZE" yaml:"rerank_batch_size"`
	MaxSeqLength    int    `envconfig:"RICE_MAX_SEQ_LENGTH" yaml:"max_seq_length"`
	ModelsDir       string `envconfig:"RICE_MODELS_DIR" yaml:"models_dir"`
	ExternalURL     string `envconfig:"RICE_ML_URL" yaml:"external_url"` // For distributed mode

	// Per-model GPU settings
	EmbedGPU  bool `envconfig:"RICE_EMBED_GPU" yaml:"embed_gpu"`   // GPU for embeddings
	RerankGPU bool `envconfig:"RICE_RERANK_GPU" yaml:"rerank_gpu"` // GPU for reranking
	QueryGPU  bool `envconfig:"RICE_QUERY_GPU" yaml:"query_gpu"`   // GPU for query understanding

	// Query understanding fallback
	QueryModelEnabled bool `envconfig:"RICE_QUERY_MODEL_ENABLED" yaml:"query_model_enabled"` // If false, use keyword extraction
}

// ConnectionConfig holds connection/PC tracking settings.
type ConnectionConfig struct {
	Enabled     bool   `envconfig:"RICE_CONNECTIONS_ENABLED" yaml:"enabled"`
	StoragePath string `envconfig:"RICE_CONNECTIONS_PATH" yaml:"storage_path"`
	MaxInactive int    `envconfig:"RICE_CONNECTIONS_MAX_INACTIVE" yaml:"max_inactive"` // Days before marking inactive
}

// ModelsConfig holds model registry settings.
type ModelsConfig struct {
	RegistryPath string `envconfig:"RICE_MODELS_REGISTRY" yaml:"registry_path"`
	MappersPath  string `envconfig:"RICE_MODELS_MAPPERS" yaml:"mappers_path"`
	AutoDownload bool   `envconfig:"RICE_MODELS_AUTO_DOWNLOAD" yaml:"auto_download"`
}

// CacheConfig holds cache settings.
type CacheConfig struct {
	Type     string `envconfig:"RICE_CACHE_TYPE" yaml:"type"`
	Size     int    `envconfig:"RICE_CACHE_SIZE" yaml:"size"`
	TTL      int    `envconfig:"RICE_CACHE_TTL" yaml:"ttl"` // 0 = no expiry
	RedisURL string `envconfig:"RICE_REDIS_URL" yaml:"redis_url"`
}

// BusConfig holds event bus settings.
type BusConfig struct {
	Type         string `envconfig:"RICE_BUS_TYPE" yaml:"type"`
	KafkaBrokers string `envconfig:"RICE_KAFKA_BROKERS" yaml:"kafka_brokers"`
	NatsURL      string `envconfig:"RICE_NATS_URL" yaml:"nats_url"`
	RedisURL     string `envconfig:"RICE_REDIS_STREAM_URL" yaml:"redis_url"`
}

// IndexConfig holds indexing settings.
type IndexConfig struct {
	ChunkSize    int `envconfig:"RICE_CHUNK_SIZE" yaml:"chunk_size"`
	ChunkOverlap int `envconfig:"RICE_CHUNK_OVERLAP" yaml:"chunk_overlap"`
	Workers      int `envconfig:"RICE_INDEX_WORKERS" yaml:"workers"`
}

// SearchConfig holds search settings.
type SearchConfig struct {
	DefaultTopK         int     `envconfig:"RICE_DEFAULT_TOP_K" yaml:"default_top_k"`
	DefaultSparseWeight float64 `envconfig:"RICE_DEFAULT_SPARSE_WEIGHT" yaml:"default_sparse_weight"`
	DefaultDenseWeight  float64 `envconfig:"RICE_DEFAULT_DENSE_WEIGHT" yaml:"default_dense_weight"`
	EnableReranking     bool    `envconfig:"RICE_ENABLE_RERANKING" yaml:"enable_reranking"`
	RerankCandidates    int     `envconfig:"RICE_RERANK_CANDIDATES" yaml:"rerank_candidates"`
}

// LogConfig holds logging settings.
type LogConfig struct {
	Level  string `envconfig:"RICE_LOG_LEVEL" yaml:"level"`
	Format string `envconfig:"RICE_LOG_FORMAT" yaml:"format"`
	File   string `envconfig:"RICE_LOG_FILE" yaml:"file"`
}

// SecurityConfig holds security settings.
type SecurityConfig struct {
	APIKey      string `envconfig:"RICE_API_KEY" yaml:"api_key"`
	RateLimit   int    `envconfig:"RICE_RATE_LIMIT" yaml:"rate_limit"` // 0 = disabled
	CORSOrigins string `envconfig:"RICE_CORS_ORIGINS" yaml:"cors_origins"`
}

// ObservabilityConfig holds observability settings.
type ObservabilityConfig struct {
	MetricsEnabled  bool   `envconfig:"RICE_METRICS_ENABLED" yaml:"metrics_enabled"`
	MetricsPath     string `envconfig:"RICE_METRICS_PATH" yaml:"metrics_path"`
	TracingEnabled  bool   `envconfig:"RICE_TRACING_ENABLED" yaml:"tracing_enabled"`
	TracingEndpoint string `envconfig:"RICE_TRACING_ENDPOINT" yaml:"tracing_endpoint"`
}

// Load loads configuration from environment variables and optional config file.
func Load(configPath string) (*Config, error) {
	cfg := &Config{}

	// Set defaults first
	setDefaults(cfg)

	// Load from YAML file if provided (overrides defaults)
	if configPath != "" {
		if err := loadFromFile(cfg, configPath); err != nil {
			return nil, fmt.Errorf("loading config file: %w", err)
		}
	}

	// Override with environment variables (highest priority)
	if err := envconfig.Process("", cfg); err != nil {
		return nil, fmt.Errorf("processing env config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// LoadFromEnv loads configuration from environment variables only.
func LoadFromEnv() (*Config, error) {
	return Load("")
}

func loadFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, cfg)
}

func setDefaults(cfg *Config) {
	cfg.Host = "0.0.0.0"
	cfg.Port = 8080
	cfg.EnableWeb = true
	cfg.EnableML = true

	cfg.Qdrant = QdrantConfig{
		URL:              "http://localhost:6333",
		CollectionPrefix: "rice_",
	}

	cfg.ML = MLConfig{
		Device:            "cuda", // GPU by default
		EmbedModel:        "jinaai/jina-code-embeddings-1.5b",
		SparseModel:       "splade-v3",
		RerankModel:       "jinaai/jina-reranker-v2-base-multilingual",
		QueryModel:        "Salesforce/codet5p-220m",
		EmbedDim:          1536,
		EmbedBatchSize:    32,
		SparseBatchSize:   32,
		RerankBatchSize:   16,
		MaxSeqLength:      8192,
		ModelsDir:         "./models",
		EmbedGPU:          true,  // GPU enabled by default
		RerankGPU:         true,  // GPU enabled by default
		QueryGPU:          false, // Query model GPU off by default (use Option C)
		QueryModelEnabled: false, // Use keyword extraction by default
	}

	cfg.Connection = ConnectionConfig{
		Enabled:     true,
		StoragePath: "./data/connections",
		MaxInactive: 30, // 30 days
	}

	cfg.Models = ModelsConfig{
		RegistryPath: "./data/models/registry.yaml",
		MappersPath:  "./data/models/mappers",
		AutoDownload: false,
	}

	cfg.Cache = CacheConfig{
		Type:     "memory",
		Size:     10000,
		TTL:      0,
		RedisURL: "redis://localhost:6379",
	}

	cfg.Bus = BusConfig{
		Type: "memory",
	}

	cfg.Index = IndexConfig{
		ChunkSize:    512,
		ChunkOverlap: 64,
		Workers:      4,
	}

	cfg.Search = SearchConfig{
		DefaultTopK:         20,
		DefaultSparseWeight: 0.5,
		DefaultDenseWeight:  0.5,
		EnableReranking:     true,
		RerankCandidates:    30,
	}

	cfg.Log = LogConfig{
		Level:  "info",
		Format: "text",
	}

	cfg.Security = SecurityConfig{
		RateLimit:   0,
		CORSOrigins: "*",
	}

	cfg.Observability = ObservabilityConfig{
		MetricsEnabled: true,
		MetricsPath:    "/metrics",
		TracingEnabled: false,
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	var errs []string

	// Server validation
	if c.Port < 1 || c.Port > 65535 {
		errs = append(errs, "port must be between 1 and 65535")
	}

	// ML validation
	validDevices := map[string]bool{"cpu": true, "cuda": true, "tensorrt": true}
	if !validDevices[c.ML.Device] {
		errs = append(errs, fmt.Sprintf("invalid ML device: %s (must be cpu, cuda, or tensorrt)", c.ML.Device))
	}

	if c.ML.EmbedDim < 1 {
		errs = append(errs, "embed_dim must be positive")
	}

	if c.ML.EmbedBatchSize < 1 {
		errs = append(errs, "embed_batch_size must be positive")
	}

	// Cache validation
	validCacheTypes := map[string]bool{"memory": true, "redis": true}
	if !validCacheTypes[c.Cache.Type] {
		errs = append(errs, fmt.Sprintf("invalid cache type: %s (must be memory or redis)", c.Cache.Type))
	}

	// Bus validation
	validBusTypes := map[string]bool{"memory": true, "kafka": true, "nats": true, "redis": true}
	if !validBusTypes[c.Bus.Type] {
		errs = append(errs, fmt.Sprintf("invalid bus type: %s (must be memory, kafka, nats, or redis)", c.Bus.Type))
	}

	// Log validation
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Log.Level] {
		errs = append(errs, fmt.Sprintf("invalid log level: %s (must be debug, info, warn, or error)", c.Log.Level))
	}

	validFormats := map[string]bool{"text": true, "json": true}
	if !validFormats[c.Log.Format] {
		errs = append(errs, fmt.Sprintf("invalid log format: %s (must be text or json)", c.Log.Format))
	}

	// Index validation
	if c.Index.ChunkSize < 64 {
		errs = append(errs, "chunk_size must be at least 64")
	}

	if c.Index.ChunkOverlap >= c.Index.ChunkSize {
		errs = append(errs, "chunk_overlap must be less than chunk_size")
	}

	// Search validation
	if c.Search.DefaultTopK < 1 {
		errs = append(errs, "default_top_k must be positive")
	}

	if c.Search.DefaultSparseWeight < 0 || c.Search.DefaultSparseWeight > 1 {
		errs = append(errs, "default_sparse_weight must be between 0 and 1")
	}

	if c.Search.DefaultDenseWeight < 0 || c.Search.DefaultDenseWeight > 1 {
		errs = append(errs, "default_dense_weight must be between 0 and 1")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// Address returns the server address.
func (c *Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// IsDevelopment returns true if running in development mode.
func (c *Config) IsDevelopment() bool {
	return c.Log.Level == "debug"
}
