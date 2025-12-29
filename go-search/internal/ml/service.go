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
	Healthy      bool            `json:"healthy"`
	ModelsLoaded map[string]bool `json:"models_loaded"`
	Device       string          `json:"device"`
	Error        string          `json:"error,omitempty"`
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
	}

	status.ModelsLoaded["embedder"] = s.embedder != nil
	status.ModelsLoaded["sparse"] = s.sparse != nil
	status.ModelsLoaded["reranker"] = s.reranker != nil

	// Check if any required model is missing
	if s.embedder == nil || s.sparse == nil {
		status.Healthy = false
		status.Error = "required models not loaded"
	}

	return status
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
