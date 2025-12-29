// Package search provides the search service for Rice Search.
package search

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/qdrant"
)

// Service provides search capabilities.
type Service struct {
	ml     ml.Service
	qdrant *qdrant.Client
	log    *logger.Logger
	cfg    Config
}

// Config configures the search service.
type Config struct {
	// DefaultTopK is the default number of results to return.
	DefaultTopK int

	// PrefetchMultiplier controls how many candidates to fetch for reranking.
	// Final candidates = topK * PrefetchMultiplier
	PrefetchMultiplier int

	// EnableReranking enables neural reranking by default.
	EnableReranking bool

	// RerankTopK is the number of candidates to rerank.
	RerankTopK int

	// SparseWeight is the weight for sparse (BM25-like) results in fusion.
	SparseWeight float32

	// DenseWeight is the weight for dense (semantic) results in fusion.
	DenseWeight float32
}

// DefaultConfig returns sensible search defaults.
func DefaultConfig() Config {
	return Config{
		DefaultTopK:        20,
		PrefetchMultiplier: 3,
		EnableReranking:    true,
		RerankTopK:         50,
		SparseWeight:       0.5,
		DenseWeight:        0.5,
	}
}

// NewService creates a new search service.
func NewService(mlSvc ml.Service, qc *qdrant.Client, log *logger.Logger, cfg Config) *Service {
	if cfg.DefaultTopK == 0 {
		cfg = DefaultConfig()
	}
	return &Service{
		ml:     mlSvc,
		qdrant: qc,
		log:    log,
		cfg:    cfg,
	}
}

// Request represents a search request.
type Request struct {
	// Query is the search query text.
	Query string `json:"query"`

	// Store is the store to search in.
	Store string `json:"store"`

	// TopK is the number of results to return.
	TopK int `json:"top_k,omitempty"`

	// Filter constrains the search.
	Filter *Filter `json:"filter,omitempty"`

	// EnableReranking enables neural reranking.
	EnableReranking *bool `json:"enable_reranking,omitempty"`

	// RerankTopK is the number of candidates to rerank.
	RerankTopK int `json:"rerank_top_k,omitempty"`

	// IncludeContent includes full content in results.
	IncludeContent bool `json:"include_content,omitempty"`

	// SparseWeight overrides the sparse weight (0-1).
	SparseWeight *float32 `json:"sparse_weight,omitempty"`

	// DenseWeight overrides the dense weight (0-1).
	DenseWeight *float32 `json:"dense_weight,omitempty"`
}

// Filter defines search filters.
type Filter struct {
	// PathPrefix filters by path prefix.
	PathPrefix string `json:"path_prefix,omitempty"`

	// Languages filters by programming language.
	Languages []string `json:"languages,omitempty"`
}

// Result represents a single search result.
type Result struct {
	// ID is the chunk identifier.
	ID string `json:"id"`

	// Path is the file path.
	Path string `json:"path"`

	// Language is the programming language.
	Language string `json:"language"`

	// StartLine is the starting line number.
	StartLine int `json:"start_line"`

	// EndLine is the ending line number.
	EndLine int `json:"end_line"`

	// Content is the chunk content (if requested).
	Content string `json:"content,omitempty"`

	// Symbols are the extracted symbols.
	Symbols []string `json:"symbols,omitempty"`

	// Score is the relevance score.
	Score float32 `json:"score"`

	// RerankScore is the reranker score (if reranking was applied).
	RerankScore *float32 `json:"rerank_score,omitempty"`
}

// Response represents a search response.
type Response struct {
	// Query is the original query.
	Query string `json:"query"`

	// Store is the store that was searched.
	Store string `json:"store"`

	// Results are the search results.
	Results []Result `json:"results"`

	// Total is the total number of matches (before limit).
	Total int `json:"total"`

	// Metadata contains search metadata.
	Metadata SearchMetadata `json:"metadata"`
}

// SearchMetadata contains information about how the search was performed.
type SearchMetadata struct {
	// SearchTimeMs is the total search time in milliseconds.
	SearchTimeMs int64 `json:"search_time_ms"`

	// EmbedTimeMs is the embedding generation time.
	EmbedTimeMs int64 `json:"embed_time_ms"`

	// RetrievalTimeMs is the vector search time.
	RetrievalTimeMs int64 `json:"retrieval_time_ms"`

	// RerankTimeMs is the reranking time (if applied).
	RerankTimeMs int64 `json:"rerank_time_ms,omitempty"`

	// CandidatesReranked is the number of candidates that were reranked.
	CandidatesReranked int `json:"candidates_reranked,omitempty"`

	// RerankingApplied indicates if reranking was applied.
	RerankingApplied bool `json:"reranking_applied"`
}

// Search performs a hybrid search with optional reranking.
func (s *Service) Search(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()

	// Apply defaults
	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}

	enableReranking := s.cfg.EnableReranking
	if req.EnableReranking != nil {
		enableReranking = *req.EnableReranking
	}

	rerankTopK := s.cfg.RerankTopK
	if req.RerankTopK > 0 {
		rerankTopK = req.RerankTopK
	}

	// Validate
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.Store == "" {
		return nil, fmt.Errorf("store is required")
	}

	// Check store exists
	exists, err := s.qdrant.CollectionExists(ctx, req.Store)
	if err != nil {
		return nil, fmt.Errorf("failed to check store: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("store not found: %s", req.Store)
	}

	// Generate embeddings for query
	embedStart := time.Now()
	denseVectors, err := s.ml.Embed(ctx, []string{req.Query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate dense embedding: %w", err)
	}

	sparseVectors, err := s.ml.SparseEncode(ctx, []string{req.Query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate sparse embedding: %w", err)
	}
	embedTime := time.Since(embedStart)

	// Build search request
	prefetchLimit := uint64(topK * s.cfg.PrefetchMultiplier)
	if enableReranking && uint64(rerankTopK) > prefetchLimit {
		prefetchLimit = uint64(rerankTopK)
	}

	searchReq := qdrant.SearchRequest{
		DenseVector:   denseVectors[0],
		SparseIndices: sparseVectors[0].Indices,
		SparseValues:  sparseVectors[0].Values,
		Limit:         prefetchLimit,
		PrefetchLimit: prefetchLimit,
		WithPayload:   true,
	}

	// Apply filter
	if req.Filter != nil {
		searchReq.Filter = &qdrant.SearchFilter{
			PathPrefix: req.Filter.PathPrefix,
			Languages:  req.Filter.Languages,
		}
	}

	// Execute hybrid search
	retrievalStart := time.Now()
	qdrantResults, err := s.qdrant.HybridSearch(ctx, req.Store, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	retrievalTime := time.Since(retrievalStart)

	// Convert to results
	results := make([]Result, len(qdrantResults))
	for i, qr := range qdrantResults {
		results[i] = Result{
			ID:        qr.ID,
			Path:      qr.Payload.Path,
			Language:  qr.Payload.Language,
			StartLine: qr.Payload.StartLine,
			EndLine:   qr.Payload.EndLine,
			Symbols:   qr.Payload.Symbols,
			Score:     qr.Score,
		}
		if req.IncludeContent {
			results[i].Content = qr.Payload.Content
		}
	}

	metadata := SearchMetadata{
		EmbedTimeMs:     embedTime.Milliseconds(),
		RetrievalTimeMs: retrievalTime.Milliseconds(),
	}

	// Apply reranking if enabled and we have results
	if enableReranking && len(results) > 0 {
		rerankStart := time.Now()

		// Get contents for reranking
		documents := make([]string, len(qdrantResults))
		for i, qr := range qdrantResults {
			documents[i] = qr.Payload.Content
		}

		// Rerank
		ranked, err := s.ml.Rerank(ctx, req.Query, documents, topK)
		if err != nil {
			s.log.Warn("Reranking failed, using original order", "error", err)
		} else {
			// Reorder results based on rerank scores
			rerankedResults := make([]Result, len(ranked))
			for i, r := range ranked {
				rerankedResults[i] = results[r.Index]
				rerankedResults[i].RerankScore = &r.Score
			}
			results = rerankedResults

			metadata.RerankTimeMs = time.Since(rerankStart).Milliseconds()
			metadata.CandidatesReranked = len(documents)
			metadata.RerankingApplied = true
		}
	}

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	metadata.SearchTimeMs = time.Since(start).Milliseconds()

	return &Response{
		Query:    req.Query,
		Store:    req.Store,
		Results:  results,
		Total:    len(qdrantResults),
		Metadata: metadata,
	}, nil
}

// SearchDenseOnly performs a dense-only (semantic) search.
func (s *Service) SearchDenseOnly(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()

	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}

	if req.Query == "" || req.Store == "" {
		return nil, fmt.Errorf("query and store are required")
	}

	// Generate dense embedding
	embedStart := time.Now()
	denseVectors, err := s.ml.Embed(ctx, []string{req.Query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	embedTime := time.Since(embedStart)

	// Search
	searchReq := qdrant.SearchRequest{
		DenseVector: denseVectors[0],
		Limit:       uint64(topK),
		WithPayload: true,
	}

	if req.Filter != nil {
		searchReq.Filter = &qdrant.SearchFilter{
			PathPrefix: req.Filter.PathPrefix,
			Languages:  req.Filter.Languages,
		}
	}

	retrievalStart := time.Now()
	qdrantResults, err := s.qdrant.DenseSearch(ctx, req.Store, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	retrievalTime := time.Since(retrievalStart)

	results := make([]Result, len(qdrantResults))
	for i, qr := range qdrantResults {
		results[i] = Result{
			ID:        qr.ID,
			Path:      qr.Payload.Path,
			Language:  qr.Payload.Language,
			StartLine: qr.Payload.StartLine,
			EndLine:   qr.Payload.EndLine,
			Symbols:   qr.Payload.Symbols,
			Score:     qr.Score,
		}
		if req.IncludeContent {
			results[i].Content = qr.Payload.Content
		}
	}

	return &Response{
		Query:   req.Query,
		Store:   req.Store,
		Results: results,
		Total:   len(results),
		Metadata: SearchMetadata{
			SearchTimeMs:    time.Since(start).Milliseconds(),
			EmbedTimeMs:     embedTime.Milliseconds(),
			RetrievalTimeMs: retrievalTime.Milliseconds(),
		},
	}, nil
}

// SearchSparseOnly performs a sparse-only (lexical) search.
func (s *Service) SearchSparseOnly(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()

	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}

	if req.Query == "" || req.Store == "" {
		return nil, fmt.Errorf("query and store are required")
	}

	// Generate sparse embedding
	embedStart := time.Now()
	sparseVectors, err := s.ml.SparseEncode(ctx, []string{req.Query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate sparse embedding: %w", err)
	}
	embedTime := time.Since(embedStart)

	// Search
	searchReq := qdrant.SearchRequest{
		SparseIndices: sparseVectors[0].Indices,
		SparseValues:  sparseVectors[0].Values,
		Limit:         uint64(topK),
		WithPayload:   true,
	}

	if req.Filter != nil {
		searchReq.Filter = &qdrant.SearchFilter{
			PathPrefix: req.Filter.PathPrefix,
			Languages:  req.Filter.Languages,
		}
	}

	retrievalStart := time.Now()
	qdrantResults, err := s.qdrant.SparseSearch(ctx, req.Store, searchReq)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	retrievalTime := time.Since(retrievalStart)

	results := make([]Result, len(qdrantResults))
	for i, qr := range qdrantResults {
		results[i] = Result{
			ID:        qr.ID,
			Path:      qr.Payload.Path,
			Language:  qr.Payload.Language,
			StartLine: qr.Payload.StartLine,
			EndLine:   qr.Payload.EndLine,
			Symbols:   qr.Payload.Symbols,
			Score:     qr.Score,
		}
		if req.IncludeContent {
			results[i].Content = qr.Payload.Content
		}
	}

	return &Response{
		Query:   req.Query,
		Store:   req.Store,
		Results: results,
		Total:   len(results),
		Metadata: SearchMetadata{
			SearchTimeMs:    time.Since(start).Milliseconds(),
			EmbedTimeMs:     embedTime.Milliseconds(),
			RetrievalTimeMs: retrievalTime.Milliseconds(),
		},
	}, nil
}

// Similar finds similar chunks to a given chunk ID.
func (s *Service) Similar(ctx context.Context, store, chunkID string, topK int) ([]Result, error) {
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}

	// For similarity, we'd need to fetch the chunk's vector and search
	// This is a simplified implementation that returns an error for now
	// A full implementation would:
	// 1. Fetch the chunk by ID
	// 2. Use its vector for similarity search
	// 3. Exclude the original chunk from results

	return nil, fmt.Errorf("similar search not yet implemented")
}

// GroupByFile groups results by file path.
func GroupByFile(results []Result, maxPerFile int) []Result {
	if maxPerFile <= 0 {
		maxPerFile = 3
	}

	fileGroups := make(map[string][]Result)
	fileOrder := make([]string, 0)

	for _, r := range results {
		if _, exists := fileGroups[r.Path]; !exists {
			fileOrder = append(fileOrder, r.Path)
		}
		fileGroups[r.Path] = append(fileGroups[r.Path], r)
	}

	var grouped []Result
	for _, path := range fileOrder {
		chunks := fileGroups[path]
		// Sort by score within file
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].Score > chunks[j].Score
		})
		// Take top N per file
		if len(chunks) > maxPerFile {
			chunks = chunks[:maxPerFile]
		}
		grouped = append(grouped, chunks...)
	}

	return grouped
}
