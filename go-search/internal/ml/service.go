// Package ml provides machine learning inference services.
package ml

import (
	"context"
	"sync"

	"github.com/ricesearch/rice-search/internal/config"
	"github.com/ricesearch/rice-search/internal/onnx"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// Service provides ML inference capabilities.
type Service interface {
	// Embed generates dense embeddings for texts.
	Embed(ctx context.Context, texts []string) ([][]float32, error)

	// SparseEncode generates sparse vectors (SPLADE) for texts.
	SparseEncode(ctx context.Context, texts []string) ([]SparseVector, error)

	// Rerank reranks documents for a query.
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]RankedResult, error)

	// Health returns the service health status.
	Health() HealthStatus

	// ReloadModels reloads all models (for hot-reload after download).
	ReloadModels() error

	// ReloadModelsWithConfig reloads models with new configuration.
	// This enables changing model paths or GPU settings at runtime without restart.
	ReloadModelsWithConfig(cfg config.MLConfig) error

	// Close releases resources.
	Close() error
}

// SparseVector represents a sparse vector with indices and values.
type SparseVector struct {
	Indices []uint32
	Values  []float32
}

// RankedResult represents a reranked document.
type RankedResult struct {
	Index int
	Score float32
}

// HealthStatus represents service health.
type HealthStatus struct {
	Healthy        bool            `json:"healthy"`
	ModelsLoaded   map[string]bool `json:"models_loaded"`
	Device         string          `json:"device"`            // Requested device
	ActualDevice   string          `json:"actual_device"`     // Actual device being used
	DeviceFallback bool            `json:"device_fallback"`   // True if actual differs from requested
	RuntimeAvail   bool            `json:"runtime_available"` // True if ONNX Runtime is available
	Error          string          `json:"error,omitempty"`
}

// ServiceImpl implements the ML Service.
type ServiceImpl struct {
	mu       sync.RWMutex
	cfg      config.MLConfig
	log      *logger.Logger
	runtime  *onnx.Runtime
	embedder *Embedder
	sparse   *SparseEncoder
	reranker *Reranker
	cache    *EmbeddingCache
}

// NewService creates a new ML service.
func NewService(cfg config.MLConfig, log *logger.Logger) (*ServiceImpl, error) {
	// Create ONNX runtime
	device := onnx.DeviceCPU
	switch cfg.Device {
	case "cuda":
		device = onnx.DeviceCUDA
	case "tensorrt":
		device = onnx.DeviceTensorRT
	}

	runtimeCfg := onnx.DefaultRuntimeConfig()
	runtimeCfg.Device = device
	runtimeCfg.CUDADeviceID = cfg.CUDADevice

	runtime, err := onnx.NewRuntime(runtimeCfg)
	if err != nil {
		return nil, errors.Wrap(errors.CodeMLError, "failed to create ONNX runtime", err)
	}

	svc := &ServiceImpl{
		cfg:     cfg,
		log:     log,
		runtime: runtime,
		cache:   NewEmbeddingCache(10000), // Default cache size
	}

	return svc, nil
}

// LoadModels loads all required models.
func (s *ServiceImpl) LoadModels() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error

	// Load embedder
	embedder, err := NewEmbedder(s.runtime, s.cfg, s.log)
	if err != nil {
		s.log.Error("failed to load embedder", "error", err)
		lastErr = err
	} else {
		s.embedder = embedder
	}

	// Load sparse encoder
	sparse, err := NewSparseEncoder(s.runtime, s.cfg, s.log)
	if err != nil {
		s.log.Error("failed to load sparse encoder", "error", err)
		lastErr = err
	} else {
		s.sparse = sparse
	}

	// Load reranker
	reranker, err := NewReranker(s.runtime, s.cfg, s.log)
	if err != nil {
		s.log.Error("failed to load reranker", "error", err)
		lastErr = err
	} else {
		s.reranker = reranker
	}

	return lastErr
}

// Embed generates dense embeddings.
func (s *ServiceImpl) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.embedder == nil {
		return nil, errors.New(errors.CodeMLError, "embedder not loaded")
	}

	// Check cache first
	results := make([][]float32, len(texts))
	uncached := make([]int, 0)
	uncachedTexts := make([]string, 0)

	for i, text := range texts {
		if emb, ok := s.cache.Get(text); ok {
			results[i] = emb
		} else {
			uncached = append(uncached, i)
			uncachedTexts = append(uncachedTexts, text)
		}
	}

	// Embed uncached texts
	if len(uncachedTexts) > 0 {
		embeddings, err := s.embedder.Embed(ctx, uncachedTexts)
		if err != nil {
			return nil, err
		}

		// Store in cache and results
		for i, idx := range uncached {
			results[idx] = embeddings[i]
			s.cache.Set(uncachedTexts[i], embeddings[i])
		}
	}

	return results, nil
}

// SparseEncode generates sparse vectors.
func (s *ServiceImpl) SparseEncode(ctx context.Context, texts []string) ([]SparseVector, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.sparse == nil {
		return nil, errors.New(errors.CodeMLError, "sparse encoder not loaded")
	}

	return s.sparse.Encode(ctx, texts)
}

// Rerank reranks documents.
func (s *ServiceImpl) Rerank(ctx context.Context, query string, documents []string, topK int) ([]RankedResult, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.reranker == nil {
		return nil, errors.New(errors.CodeMLError, "reranker not loaded")
	}

	return s.reranker.Rerank(ctx, query, documents, topK)
}

// Health returns service health.
func (s *ServiceImpl) Health() HealthStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := HealthStatus{
		Healthy:      true,
		ModelsLoaded: make(map[string]bool),
		Device:       s.cfg.Device,
		RuntimeAvail: onnx.IsAvailable(),
	}

	// Get actual device info from runtime
	if s.runtime != nil {
		status.ActualDevice = string(s.runtime.ActualDevice())
		status.DeviceFallback = s.runtime.DeviceFallback()
	} else {
		status.ActualDevice = "none"
		status.DeviceFallback = true
	}

	status.ModelsLoaded["embedder"] = s.embedder != nil
	status.ModelsLoaded["sparse"] = s.sparse != nil
	status.ModelsLoaded["reranker"] = s.reranker != nil

	// Check if any required model is missing
	if s.embedder == nil || s.sparse == nil {
		status.Healthy = false
		status.Error = "required models not loaded"
	}

	// Add warning about device fallback
	if status.DeviceFallback && status.Error == "" {
		if s.cfg.Device == "cuda" || s.cfg.Device == "tensorrt" {
			status.Error = "GPU requested but not available; using fallback"
		} else if !status.RuntimeAvail {
			status.Error = "ONNX Runtime not available"
		}
	}

	return status
}

// ReloadModels unloads existing models and reloads them from disk.
// This enables hot-reload after model download without server restart.
func (s *ServiceImpl) ReloadModels() error {
	return s.ReloadModelsWithConfig(s.cfg)
}

// ReloadModelsWithConfig reloads models with new configuration.
// This enables changing model paths or GPU settings at runtime without restart.
func (s *ServiceImpl) ReloadModelsWithConfig(newCfg config.MLConfig) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.log.Info("Reloading ML models with new config",
		"old_embed_gpu", s.cfg.EmbedGPU,
		"new_embed_gpu", newCfg.EmbedGPU,
		"old_rerank_gpu", s.cfg.RerankGPU,
		"new_rerank_gpu", newCfg.RerankGPU,
		"old_embed_model", s.cfg.EmbedModel,
		"new_embed_model", newCfg.EmbedModel,
		"old_rerank_model", s.cfg.RerankModel,
		"new_rerank_model", newCfg.RerankModel,
	)

	// Update config
	s.cfg = newCfg

	// Close existing models
	if s.embedder != nil {
		if err := s.embedder.Close(); err != nil {
			s.log.Warn("Failed to close embedder", "error", err)
		}
		s.embedder = nil
	}

	if s.sparse != nil {
		if err := s.sparse.Close(); err != nil {
			s.log.Warn("Failed to close sparse encoder", "error", err)
		}
		s.sparse = nil
	}

	if s.reranker != nil {
		if err := s.reranker.Close(); err != nil {
			s.log.Warn("Failed to close reranker", "error", err)
		}
		s.reranker = nil
	}

	// Clear embedding cache (models changed, old embeddings may be incompatible)
	s.cache = NewEmbeddingCache(10000)

	// Reload models with new config
	var lastErr error

	embedder, err := NewEmbedder(s.runtime, s.cfg, s.log)
	if err != nil {
		s.log.Error("Failed to reload embedder", "error", err)
		lastErr = err
	} else {
		s.embedder = embedder
		s.log.Info("Reloaded embedder")
	}

	sparse, err := NewSparseEncoder(s.runtime, s.cfg, s.log)
	if err != nil {
		s.log.Error("Failed to reload sparse encoder", "error", err)
		lastErr = err
	} else {
		s.sparse = sparse
		s.log.Info("Reloaded sparse encoder")
	}

	reranker, err := NewReranker(s.runtime, s.cfg, s.log)
	if err != nil {
		s.log.Error("Failed to reload reranker", "error", err)
		lastErr = err
	} else {
		s.reranker = reranker
		s.log.Info("Reloaded reranker")
	}

	s.log.Info("ML model reload complete")
	return lastErr
}

// Close releases resources.
func (s *ServiceImpl) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var lastErr error

	if s.embedder != nil {
		if err := s.embedder.Close(); err != nil {
			lastErr = err
		}
	}

	if s.sparse != nil {
		if err := s.sparse.Close(); err != nil {
			lastErr = err
		}
	}

	if s.reranker != nil {
		if err := s.reranker.Close(); err != nil {
			lastErr = err
		}
	}

	if s.runtime != nil {
		if err := s.runtime.Close(); err != nil {
			lastErr = err
		}
	}

	return lastErr
}
