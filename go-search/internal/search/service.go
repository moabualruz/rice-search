// Package search provides the search service for Rice Search.
package search

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
	"github.com/ricesearch/rice-search/internal/metrics"
	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/query"
	"github.com/ricesearch/rice-search/internal/search/fusion"
	"github.com/ricesearch/rice-search/internal/search/postrank"
)

// Service provides search capabilities.
type Service struct {
	ml          ml.Service
	qdrant      *qdrant.Client
	querySvc    *query.Service
	bus         bus.Bus
	log         *logger.Logger
	cfg         Config
	postrank    *postrank.Pipeline
	metrics     *metrics.Metrics
	mu          sync.RWMutex
	monitorSvc  MonitoringService
	monitorOnce sync.Once
}

// MonitoringService defines the interface for connection monitoring.
type MonitoringService interface {
	RecordSearch(connectionID string)
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

	// Post-ranking configuration
	EnableDedup      bool
	DedupThreshold   float32
	EnableDiversity  bool
	DiversityLambda  float32
	GroupByFile      bool
	MaxChunksPerFile int
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
		EnableDedup:        true,
		DedupThreshold:     0.85,
		EnableDiversity:    true,
		DiversityLambda:    0.7,
		GroupByFile:        false,
		MaxChunksPerFile:   3,
	}
}

// NewService creates a new search service.
// querySvc, eventBus, and metrics are optional - if nil, features are disabled.
func NewService(mlSvc ml.Service, qc *qdrant.Client, log *logger.Logger, cfg Config, querySvc *query.Service, eventBus bus.Bus, metrics *metrics.Metrics) *Service {
	if cfg.DefaultTopK == 0 {
		cfg = DefaultConfig()
	}

	// Create post-ranking pipeline
	postrankCfg := postrank.Config{
		EnableDedup:      cfg.EnableDedup,
		DedupThreshold:   cfg.DedupThreshold,
		EnableDiversity:  cfg.EnableDiversity,
		DiversityLambda:  cfg.DiversityLambda,
		GroupByFile:      cfg.GroupByFile,
		MaxChunksPerFile: cfg.MaxChunksPerFile,
	}
	postrankPipeline := postrank.NewPipeline(postrankCfg, log)

	return &Service{
		ml:       mlSvc,
		qdrant:   qc,
		querySvc: querySvc,
		bus:      eventBus,
		log:      log,
		cfg:      cfg,
		postrank: postrankPipeline,
		metrics:  metrics,
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

	// GroupByConnection groups results by connection_id.
	GroupByConnection bool `json:"group_by_connection,omitempty"`

	// MaxChunksPerConnection limits chunks per connection when grouping (default: 3).
	MaxChunksPerConnection int `json:"max_chunks_per_connection,omitempty"`
}

// Filter defines search filters.
type Filter struct {
	// PathPrefix filters by path prefix.
	PathPrefix string `json:"path_prefix,omitempty"`

	// Languages filters by programming language.
	Languages []string `json:"languages,omitempty"`

	// ConnectionID filters by connection.
	ConnectionID string `json:"connection_id,omitempty"`
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

	// Score is the relevance score (fused or single retriever).
	Score float32 `json:"score"`

	// RerankScore is the reranker score (if reranking was applied).
	RerankScore *float32 `json:"rerank_score,omitempty"`

	// ConnectionID is the connection that indexed this chunk.
	ConnectionID string `json:"connection_id,omitempty"`

	// SparseRank is the rank in sparse-only results (1-based, 0 if manual fusion not used).
	SparseRank int `json:"sparse_rank,omitempty"`

	// DenseRank is the rank in dense-only results (1-based, 0 if manual fusion not used).
	DenseRank int `json:"dense_rank,omitempty"`

	// SparseScore is the original sparse score (0 if manual fusion not used).
	SparseScore float32 `json:"sparse_score,omitempty"`

	// DenseScore is the original dense score (0 if manual fusion not used).
	DenseScore float32 `json:"dense_score,omitempty"`

	// FusedScore is the combined RRF score (only when manual fusion is used).
	FusedScore float32 `json:"fused_score,omitempty"`
}

// ConnectionGroup represents results grouped by connection.
type ConnectionGroup struct {
	// ConnectionID is the connection identifier.
	ConnectionID string `json:"connection_id"`

	// ConnectionName is the human-readable connection name (if available).
	ConnectionName string `json:"connection_name,omitempty"`

	// ResultCount is the total number of results from this connection.
	ResultCount int `json:"result_count"`

	// TopResults are the top-scoring results from this connection.
	TopResults []Result `json:"top_results"`
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

	// ParsedQuery contains query understanding results (if available).
	ParsedQuery *query.ParsedQuery `json:"parsed_query,omitempty"`

	// ConnectionGroups contains results grouped by connection (if requested).
	ConnectionGroups []ConnectionGroup `json:"connection_groups,omitempty"`
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

	// Get config snapshot for this request
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	// Apply defaults
	topK := req.TopK
	if topK <= 0 {
		topK = cfg.DefaultTopK
	}

	enableReranking := cfg.EnableReranking
	if req.EnableReranking != nil {
		enableReranking = *req.EnableReranking
	}

	rerankTopK := cfg.RerankTopK
	if req.RerankTopK > 0 {
		rerankTopK = req.RerankTopK
	}

	// Get fusion weights (from request or config)
	sparseWeight := cfg.SparseWeight
	if req.SparseWeight != nil {
		sparseWeight = *req.SparseWeight
	}
	denseWeight := cfg.DenseWeight
	if req.DenseWeight != nil {
		denseWeight = *req.DenseWeight
	}

	// Validate
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if req.Store == "" {
		return nil, fmt.Errorf("store is required")
	}

	// Parse query for understanding (Option B/C with fallback)
	var parsedQuery *query.ParsedQuery
	if s.querySvc != nil {
		parsed, err := s.querySvc.Parse(ctx, req.Query)
		if err != nil {
			s.log.Debug("Query understanding failed, continuing with raw query", "error", err)
		} else {
			parsedQuery = parsed
			s.log.Debug("Query understood",
				"intent", parsed.ActionIntent,
				"target", parsed.TargetType,
				"keywords", parsed.Keywords,
				"confidence", parsed.Confidence,
				"used_model", parsed.UsedModel,
			)
		}
	}

	// Check store exists
	exists, err := s.qdrant.CollectionExists(ctx, req.Store)
	if err != nil {
		return nil, fmt.Errorf("failed to check store: %w", err)
	}
	if !exists {
		return nil, fmt.Errorf("store not found: %s", req.Store)
	}

	// Generate embeddings for query via event bus
	embedStart := time.Now()
	denseVectors, err := s.embedViaEventBus(ctx, []string{req.Query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate dense embedding: %w", err)
	}

	sparseVectors, err := s.sparseEncodeViaEventBus(ctx, []string{req.Query})
	if err != nil {
		return nil, fmt.Errorf("failed to generate sparse embedding: %w", err)
	}
	embedTime := time.Since(embedStart)

	// Build search request
	prefetchLimit := uint64(topK * cfg.PrefetchMultiplier)
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
			PathPrefix:   req.Filter.PathPrefix,
			Languages:    req.Filter.Languages,
			ConnectionID: req.Filter.ConnectionID,
		}
	}

	// Decide whether to use Qdrant's native RRF or manual fusion
	fusionCfg := fusion.RRFConfig{
		K:            fusion.DefaultK,
		SparseWeight: float32(sparseWeight),
		DenseWeight:  float32(denseWeight),
	}

	var results []Result
	var retrievalTime time.Duration
	var sparseTime, denseTime, fusionTime time.Duration

	if fusionCfg.IsBalanced() {
		// Use Qdrant's native RRF (faster, equal weights)
		retrievalStart := time.Now()
		qdrantResults, err := s.qdrant.HybridSearch(ctx, req.Store, searchReq)
		if err != nil {
			return nil, fmt.Errorf("search failed: %w", err)
		}
		retrievalTime = time.Since(retrievalStart)

		// For native RRF, we can't separate sparse/dense/fusion times
		// Record retrieval time as fusion since it combines both
		fusionTime = retrievalTime
		if s.metrics != nil {
			s.metrics.RecordSearchStage(req.Store, "fusion", fusionTime.Milliseconds())
		}

		// Convert to results (no rank/score breakdown)
		results = make([]Result, len(qdrantResults))
		for i, qr := range qdrantResults {
			results[i] = Result{
				ID:           qr.ID,
				Path:         qr.Payload.Path,
				Language:     qr.Payload.Language,
				StartLine:    qr.Payload.StartLine,
				EndLine:      qr.Payload.EndLine,
				Symbols:      qr.Payload.Symbols,
				Score:        qr.Score,
				ConnectionID: qr.Payload.ConnectionID,
			}
			if req.IncludeContent {
				results[i].Content = qr.Payload.Content
			}
		}
		s.log.Debug("Used Qdrant native RRF fusion",
			"sparse_weight", sparseWeight,
			"dense_weight", denseWeight,
		)
	} else {
		// Use manual RRF fusion with custom weights

		// Execute sparse and dense searches separately
		sparseStart := time.Now()
		sparseResults, err := s.qdrant.SparseSearch(ctx, req.Store, searchReq)
		if err != nil {
			return nil, fmt.Errorf("sparse search failed: %w", err)
		}
		sparseTime = time.Since(sparseStart)
		if s.metrics != nil {
			s.metrics.RecordSearchStage(req.Store, "sparse", sparseTime.Milliseconds())
		}

		denseStart := time.Now()
		denseResults, err := s.qdrant.DenseSearch(ctx, req.Store, searchReq)
		if err != nil {
			return nil, fmt.Errorf("dense search failed: %w", err)
		}
		denseTime = time.Since(denseStart)
		if s.metrics != nil {
			s.metrics.RecordSearchStage(req.Store, "dense", denseTime.Milliseconds())
		}

		retrievalTime = sparseTime + denseTime

		// Fuse results with custom weights
		fusionStart := time.Now()
		fusedResults := fusion.Fuse(sparseResults, denseResults, fusionCfg)
		fusionTime = time.Since(fusionStart)
		if s.metrics != nil {
			s.metrics.RecordSearchStage(req.Store, "fusion", fusionTime.Milliseconds())
		}

		// Convert to Result type with rank/score breakdown
		results = make([]Result, len(fusedResults))
		for i, fr := range fusedResults {
			results[i] = Result{
				ID:           fr.Result.ID,
				Path:         fr.Result.Payload.Path,
				Language:     fr.Result.Payload.Language,
				StartLine:    fr.Result.Payload.StartLine,
				EndLine:      fr.Result.Payload.EndLine,
				Symbols:      fr.Result.Payload.Symbols,
				Score:        fr.FusedScore, // Use fused score as primary score
				ConnectionID: fr.Result.Payload.ConnectionID,
				SparseRank:   fr.SparseRank,
				DenseRank:    fr.DenseRank,
				SparseScore:  fr.SparseScore,
				DenseScore:   fr.DenseScore,
				FusedScore:   fr.FusedScore,
			}
			if req.IncludeContent {
				results[i].Content = fr.Result.Payload.Content
			}
		}
		s.log.Debug("Used manual RRF fusion",
			"sparse_weight", sparseWeight,
			"dense_weight", denseWeight,
			"sparse_results", len(sparseResults),
			"dense_results", len(denseResults),
			"fused_results", len(fusedResults),
		)
	}

	metadata := SearchMetadata{
		EmbedTimeMs:     embedTime.Milliseconds(),
		RetrievalTimeMs: retrievalTime.Milliseconds(),
	}

	// Apply reranking if enabled and we have results
	if enableReranking && len(results) > 0 {
		rerankStart := time.Now()

		// Get contents for reranking
		documents := make([]string, len(results))
		for i := range results {
			documents[i] = results[i].Content
		}

		// Rerank via event bus
		ranked, err := s.rerankViaEventBus(ctx, req.Query, documents, topK)
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

			rerankTime := time.Since(rerankStart)
			metadata.RerankTimeMs = rerankTime.Milliseconds()
			metadata.CandidatesReranked = len(documents)
			metadata.RerankingApplied = true

			// Record reranking stage metric
			if s.metrics != nil {
				s.metrics.RecordSearchStage(req.Store, "rerank", rerankTime.Milliseconds())
			}
		}
	}

	// Apply connection grouping if requested
	var connectionGroups []ConnectionGroup
	if req.GroupByConnection {
		maxPerConnection := req.MaxChunksPerConnection
		if maxPerConnection <= 0 {
			maxPerConnection = 3
		}
		connectionGroups = groupByConnection(results, maxPerConnection, s)
	}

	// Total is the count before topK limiting (but after reranking filtering)
	totalBeforeLimit := len(results)

	// Limit to topK
	if len(results) > topK {
		results = results[:topK]
	}

	metadata.SearchTimeMs = time.Since(start).Milliseconds()

	resp := &Response{
		Query:            req.Query,
		Store:            req.Store,
		Results:          results,
		Total:            totalBeforeLimit,
		Metadata:         metadata,
		ParsedQuery:      parsedQuery,
		ConnectionGroups: connectionGroups,
	}

	// Record search activity for connection monitoring
	s.recordSearchActivity(req.Filter)

	// Publish search response event for metrics
	s.publishSearchEvent(ctx, resp, nil)

	return resp, nil
}

// publishSearchEvent publishes a search response event to the event bus.
func (s *Service) publishSearchEvent(ctx context.Context, resp *Response, err error) {
	if s.bus == nil {
		return
	}

	payload := map[string]interface{}{
		"query":        resp.Query,
		"store":        resp.Store,
		"result_count": len(resp.Results),
		"total":        resp.Total,
		"latency_ms":   resp.Metadata.SearchTimeMs,
		"embed_ms":     resp.Metadata.EmbedTimeMs,
		"retrieval_ms": resp.Metadata.RetrievalTimeMs,
		"rerank_ms":    resp.Metadata.RerankTimeMs,
		"reranking":    resp.Metadata.RerankingApplied,
	}
	if err != nil {
		payload["error"] = err.Error()
	}

	event := bus.Event{
		Type:    bus.TopicSearchResponse,
		Source:  "search",
		Payload: payload,
	}
	if pubErr := s.bus.Publish(ctx, bus.TopicSearchResponse, event); pubErr != nil {
		s.log.Debug("Failed to publish search event", "error", pubErr)
	}
}

// SearchDenseOnly performs a dense-only (semantic) search.
func (s *Service) SearchDenseOnly(ctx context.Context, req Request) (*Response, error) {
	start := time.Now()

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	topK := req.TopK
	if topK <= 0 {
		topK = cfg.DefaultTopK
	}

	if req.Query == "" || req.Store == "" {
		return nil, fmt.Errorf("query and store are required")
	}

	// Generate dense embedding via event bus
	embedStart := time.Now()
	denseVectors, err := s.embedViaEventBus(ctx, []string{req.Query})
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
			PathPrefix:   req.Filter.PathPrefix,
			Languages:    req.Filter.Languages,
			ConnectionID: req.Filter.ConnectionID,
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
			ID:           qr.ID,
			Path:         qr.Payload.Path,
			Language:     qr.Payload.Language,
			StartLine:    qr.Payload.StartLine,
			EndLine:      qr.Payload.EndLine,
			Symbols:      qr.Payload.Symbols,
			Score:        qr.Score,
			ConnectionID: qr.Payload.ConnectionID,
		}
		if req.IncludeContent {
			results[i].Content = qr.Payload.Content
		}
	}

	// Apply connection grouping if requested
	var connectionGroups []ConnectionGroup
	if req.GroupByConnection {
		maxPerConnection := req.MaxChunksPerConnection
		if maxPerConnection <= 0 {
			maxPerConnection = 3
		}
		connectionGroups = groupByConnection(results, maxPerConnection, s)
	}

	// Record search activity for connection monitoring
	s.recordSearchActivity(req.Filter)

	return &Response{
		Query:            req.Query,
		Store:            req.Store,
		Results:          results,
		Total:            len(results),
		ConnectionGroups: connectionGroups,
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

	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	topK := req.TopK
	if topK <= 0 {
		topK = cfg.DefaultTopK
	}

	if req.Query == "" || req.Store == "" {
		return nil, fmt.Errorf("query and store are required")
	}

	// Generate sparse embedding via event bus
	embedStart := time.Now()
	sparseVectors, err := s.sparseEncodeViaEventBus(ctx, []string{req.Query})
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
			PathPrefix:   req.Filter.PathPrefix,
			Languages:    req.Filter.Languages,
			ConnectionID: req.Filter.ConnectionID,
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
			ID:           qr.ID,
			Path:         qr.Payload.Path,
			Language:     qr.Payload.Language,
			StartLine:    qr.Payload.StartLine,
			EndLine:      qr.Payload.EndLine,
			Symbols:      qr.Payload.Symbols,
			Score:        qr.Score,
			ConnectionID: qr.Payload.ConnectionID,
		}
		if req.IncludeContent {
			results[i].Content = qr.Payload.Content
		}
	}

	// Apply connection grouping if requested
	var connectionGroups []ConnectionGroup
	if req.GroupByConnection {
		maxPerConnection := req.MaxChunksPerConnection
		if maxPerConnection <= 0 {
			maxPerConnection = 3
		}
		connectionGroups = groupByConnection(results, maxPerConnection, s)
	}

	// Record search activity for connection monitoring
	s.recordSearchActivity(req.Filter)

	return &Response{
		Query:            req.Query,
		Store:            req.Store,
		Results:          results,
		Total:            len(results),
		ConnectionGroups: connectionGroups,
		Metadata: SearchMetadata{
			SearchTimeMs:    time.Since(start).Milliseconds(),
			EmbedTimeMs:     embedTime.Milliseconds(),
			RetrievalTimeMs: retrievalTime.Milliseconds(),
		},
	}, nil
}

// Similar finds similar chunks to a given chunk ID.
func (s *Service) Similar(ctx context.Context, store, chunkID string, topK int) ([]Result, error) {
	s.mu.RLock()
	cfg := s.cfg
	s.mu.RUnlock()

	if topK <= 0 {
		topK = cfg.DefaultTopK
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

// groupByConnection groups results by connection_id and creates connection summaries.
func groupByConnection(results []Result, maxPerConnection int, svc *Service) []ConnectionGroup {
	if maxPerConnection <= 0 {
		maxPerConnection = 3
	}

	// Group results by connection_id
	connGroups := make(map[string][]Result)
	connOrder := make([]string, 0)

	for _, r := range results {
		connID := r.ConnectionID
		if connID == "" {
			connID = "unknown"
		}
		if _, exists := connGroups[connID]; !exists {
			connOrder = append(connOrder, connID)
		}
		connGroups[connID] = append(connGroups[connID], r)
	}

	// Build connection groups
	groups := make([]ConnectionGroup, 0, len(connGroups))
	for _, connID := range connOrder {
		chunks := connGroups[connID]

		// Sort by score within connection
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].Score > chunks[j].Score
		})

		// Take top N per connection
		topResults := chunks
		if len(topResults) > maxPerConnection {
			topResults = chunks[:maxPerConnection]
		}

		// Try to get connection name (optional - requires connection service)
		// For now, leave empty - can be enriched by API layer if needed
		connectionName := ""

		groups = append(groups, ConnectionGroup{
			ConnectionID:   connID,
			ConnectionName: connectionName,
			ResultCount:    len(chunks),
			TopResults:     topResults,
		})
	}

	// Sort groups by total result count (descending)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].ResultCount > groups[j].ResultCount
	})

	return groups
}

// UpdateConfig updates the search configuration at runtime.
// This is called when settings are changed via the admin UI.
func (s *Service) UpdateConfig(cfg Config) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cfg = cfg
	s.log.Info("Search config updated",
		"default_top_k", cfg.DefaultTopK,
		"enable_reranking", cfg.EnableReranking,
		"rerank_top_k", cfg.RerankTopK,
		"sparse_weight", cfg.SparseWeight,
		"dense_weight", cfg.DenseWeight,
	)
}

// GetConfig returns the current search configuration.
func (s *Service) GetConfig() Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// QueryService returns the query understanding service.
// Returns nil if query understanding is not configured.
func (s *Service) QueryService() *query.Service {
	return s.querySvc
}

// SetMonitoringService sets the monitoring service for search tracking.
// This is called during server initialization after both services are created.
func (s *Service) SetMonitoringService(monSvc MonitoringService) {
	s.monitorOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.monitorSvc = monSvc
		if monSvc != nil {
			s.log.Info("Monitoring service attached to search service")
		}
	})
}

// recordSearchActivity records search activity for connection monitoring.
func (s *Service) recordSearchActivity(filter *Filter) {
	s.mu.RLock()
	monSvc := s.monitorSvc
	s.mu.RUnlock()

	if monSvc == nil {
		return
	}

	// Extract connection ID from filter
	connectionID := ""
	if filter != nil {
		connectionID = filter.ConnectionID
	}

	if connectionID != "" {
		monSvc.RecordSearch(connectionID)
	}
}

// embedViaEventBus generates embeddings using the event bus.
// Falls back to direct ML service call if event bus is unavailable.
func (s *Service) embedViaEventBus(ctx context.Context, texts []string) ([][]float32, error) {
	// If no event bus, fall back to direct call
	if s.bus == nil {
		return s.ml.Embed(ctx, texts)
	}

	// Create request event
	correlationID := fmt.Sprintf("embed-%d", time.Now().UnixNano())
	req := bus.Event{
		ID:            correlationID,
		Type:          bus.TopicEmbedRequest,
		Source:        "search",
		Timestamp:     time.Now().UnixNano(),
		CorrelationID: correlationID,
		Payload: map[string]interface{}{
			"texts": texts,
		},
	}

	// Send request and wait for response
	resp, err := s.bus.Request(ctx, bus.TopicEmbedRequest, req)
	if err != nil {
		s.log.Debug("Event bus embed request failed, falling back to direct call", "error", err)
		return s.ml.Embed(ctx, texts)
	}

	// Parse response
	embeddings, err := parseEmbedResponse(resp)
	if err != nil {
		s.log.Debug("Failed to parse embed response, falling back to direct call", "error", err)
		return s.ml.Embed(ctx, texts)
	}

	return embeddings, nil
}

// sparseEncodeViaEventBus generates sparse vectors using the event bus.
// Falls back to direct ML service call if event bus is unavailable.
func (s *Service) sparseEncodeViaEventBus(ctx context.Context, texts []string) ([]ml.SparseVector, error) {
	// If no event bus, fall back to direct call
	if s.bus == nil {
		return s.ml.SparseEncode(ctx, texts)
	}

	// Create request event
	correlationID := fmt.Sprintf("sparse-%d", time.Now().UnixNano())
	req := bus.Event{
		ID:            correlationID,
		Type:          bus.TopicSparseRequest,
		Source:        "search",
		Timestamp:     time.Now().UnixNano(),
		CorrelationID: correlationID,
		Payload: map[string]interface{}{
			"texts": texts,
		},
	}

	// Send request and wait for response
	resp, err := s.bus.Request(ctx, bus.TopicSparseRequest, req)
	if err != nil {
		s.log.Debug("Event bus sparse request failed, falling back to direct call", "error", err)
		return s.ml.SparseEncode(ctx, texts)
	}

	// Parse response
	vectors, err := parseSparseResponse(resp)
	if err != nil {
		s.log.Debug("Failed to parse sparse response, falling back to direct call", "error", err)
		return s.ml.SparseEncode(ctx, texts)
	}

	return vectors, nil
}

// rerankViaEventBus reranks documents using the event bus.
// Falls back to direct ML service call if event bus is unavailable.
func (s *Service) rerankViaEventBus(ctx context.Context, query string, documents []string, topK int) ([]ml.RankedResult, error) {
	// If no event bus, fall back to direct call
	if s.bus == nil {
		return s.ml.Rerank(ctx, query, documents, topK)
	}

	// Create request event
	correlationID := fmt.Sprintf("rerank-%d", time.Now().UnixNano())
	req := bus.Event{
		ID:            correlationID,
		Type:          bus.TopicRerankRequest,
		Source:        "search",
		Timestamp:     time.Now().UnixNano(),
		CorrelationID: correlationID,
		Payload: map[string]interface{}{
			"query":     query,
			"documents": documents,
			"top_k":     topK,
		},
	}

	// Send request and wait for response
	resp, err := s.bus.Request(ctx, bus.TopicRerankRequest, req)
	if err != nil {
		s.log.Debug("Event bus rerank request failed, falling back to direct call", "error", err)
		return s.ml.Rerank(ctx, query, documents, topK)
	}

	// Parse response
	results, err := parseRerankResponse(resp)
	if err != nil {
		s.log.Debug("Failed to parse rerank response, falling back to direct call", "error", err)
		return s.ml.Rerank(ctx, query, documents, topK)
	}

	return results, nil
}

// parseEmbedResponse extracts embeddings from an event bus response.
func parseEmbedResponse(event bus.Event) ([][]float32, error) {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid embed response payload type")
	}

	// Check for error
	if errStr, ok := payload["error"].(string); ok && errStr != "" {
		return nil, fmt.Errorf("embed error: %s", errStr)
	}

	// Extract embeddings
	embeddingsRaw, ok := payload["embeddings"]
	if !ok {
		return nil, fmt.Errorf("missing embeddings in response")
	}

	// Convert to [][]float32
	return convertToFloat32Slice2D(embeddingsRaw)
}

// parseSparseResponse extracts sparse vectors from an event bus response.
func parseSparseResponse(event bus.Event) ([]ml.SparseVector, error) {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid sparse response payload type")
	}

	// Check for error
	if errStr, ok := payload["error"].(string); ok && errStr != "" {
		return nil, fmt.Errorf("sparse error: %s", errStr)
	}

	// Extract vectors
	vectorsRaw, ok := payload["vectors"]
	if !ok {
		return nil, fmt.Errorf("missing vectors in response")
	}

	// Convert to []ml.SparseVector
	return convertToSparseVectors(vectorsRaw)
}

// parseRerankResponse extracts ranked results from an event bus response.
func parseRerankResponse(event bus.Event) ([]ml.RankedResult, error) {
	payload, ok := event.Payload.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid rerank response payload type")
	}

	// Check for error
	if errStr, ok := payload["error"].(string); ok && errStr != "" {
		return nil, fmt.Errorf("rerank error: %s", errStr)
	}

	// Extract results
	resultsRaw, ok := payload["results"]
	if !ok {
		return nil, fmt.Errorf("missing results in response")
	}

	// Convert to []ml.RankedResult
	return convertToRankedResults(resultsRaw)
}

// convertToFloat32Slice2D converts interface{} to [][]float32.
func convertToFloat32Slice2D(v interface{}) ([][]float32, error) {
	switch arr := v.(type) {
	case [][]float32:
		return arr, nil
	case []interface{}:
		result := make([][]float32, len(arr))
		for i, row := range arr {
			switch rowArr := row.(type) {
			case []float32:
				result[i] = rowArr
			case []interface{}:
				result[i] = make([]float32, len(rowArr))
				for j, val := range rowArr {
					switch num := val.(type) {
					case float64:
						result[i][j] = float32(num)
					case float32:
						result[i][j] = num
					default:
						return nil, fmt.Errorf("invalid embedding value type at [%d][%d]: %T", i, j, val)
					}
				}
			default:
				return nil, fmt.Errorf("invalid embedding row type at [%d]: %T", i, row)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("invalid embeddings type: %T", v)
	}
}

// convertToSparseVectors converts interface{} to []ml.SparseVector.
func convertToSparseVectors(v interface{}) ([]ml.SparseVector, error) {
	arr, ok := v.([]interface{})
	if !ok {
		// Try direct type
		if vectors, ok := v.([]ml.SparseVector); ok {
			return vectors, nil
		}
		return nil, fmt.Errorf("invalid sparse vectors type: %T", v)
	}

	result := make([]ml.SparseVector, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid sparse vector item type at [%d]: %T", i, item)
		}

		// Extract indices
		indicesRaw, ok := m["indices"]
		if ok {
			result[i].Indices = convertToUint32Slice(indicesRaw)
		}

		// Extract values
		valuesRaw, ok := m["values"]
		if ok {
			result[i].Values = convertToFloat32Slice(valuesRaw)
		}
	}

	return result, nil
}

// convertToRankedResults converts interface{} to []ml.RankedResult.
func convertToRankedResults(v interface{}) ([]ml.RankedResult, error) {
	arr, ok := v.([]interface{})
	if !ok {
		// Try direct type
		if results, ok := v.([]ml.RankedResult); ok {
			return results, nil
		}
		return nil, fmt.Errorf("invalid ranked results type: %T", v)
	}

	result := make([]ml.RankedResult, len(arr))
	for i, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid ranked result item type at [%d]: %T", i, item)
		}

		if idx, ok := m["index"]; ok {
			result[i].Index = int(toFloat64(idx))
		}
		if score, ok := m["score"]; ok {
			result[i].Score = float32(toFloat64(score))
		}
	}

	return result, nil
}

// convertToUint32Slice converts interface{} to []uint32.
func convertToUint32Slice(v interface{}) []uint32 {
	switch arr := v.(type) {
	case []uint32:
		return arr
	case []interface{}:
		result := make([]uint32, len(arr))
		for i, val := range arr {
			result[i] = uint32(toFloat64(val))
		}
		return result
	default:
		return nil
	}
}

// convertToFloat32Slice converts interface{} to []float32.
func convertToFloat32Slice(v interface{}) []float32 {
	switch arr := v.(type) {
	case []float32:
		return arr
	case []interface{}:
		result := make([]float32, len(arr))
		for i, val := range arr {
			result[i] = float32(toFloat64(val))
		}
		return result
	default:
		return nil
	}
}

// toFloat64 safely converts various number types to float64.
func toFloat64(v interface{}) float64 {
	switch num := v.(type) {
	case float64:
		return num
	case float32:
		return float64(num)
	case int:
		return float64(num)
	case int32:
		return float64(num)
	case int64:
		return float64(num)
	case uint32:
		return float64(num)
	case uint64:
		return float64(num)
	default:
		return 0
	}
}
