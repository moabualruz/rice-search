package index

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/errors"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/qdrant"
)

// PipelineConfig configures the indexing pipeline.
type PipelineConfig struct {
	// EmbedBatchSize is the batch size for embedding generation.
	EmbedBatchSize int

	// UpsertBatchSize is the batch size for Qdrant upserts.
	UpsertBatchSize int

	// Workers is the number of parallel workers.
	Workers int

	// SkipExisting skips files that haven't changed (by hash).
	SkipExisting bool
}

// DefaultPipelineConfig returns sensible defaults.
func DefaultPipelineConfig() PipelineConfig {
	return PipelineConfig{
		EmbedBatchSize:  32,
		UpsertBatchSize: 100,
		Workers:         4,
		SkipExisting:    true,
	}
}

// Pipeline orchestrates the full indexing flow:
// Documents → Chunks → Embeddings → Qdrant
type Pipeline struct {
	cfg     PipelineConfig
	ml      ml.Service
	qdrant  *qdrant.Client
	chunker *Chunker
	tracker *Tracker
	log     *logger.Logger
	mu      sync.RWMutex
}

// NewPipeline creates a new indexing pipeline.
func NewPipeline(cfg PipelineConfig, mlSvc ml.Service, qc *qdrant.Client, log *logger.Logger) *Pipeline {
	if cfg.EmbedBatchSize == 0 {
		cfg = DefaultPipelineConfig()
	}

	return &Pipeline{
		cfg:     cfg,
		ml:      mlSvc,
		qdrant:  qc,
		chunker: NewChunker(DefaultChunkerConfig()),
		tracker: NewTracker(),
		log:     log,
	}
}

// IndexRequest represents a request to index documents.
type IndexRequest struct {
	Store     string
	Documents []*Document
	Force     bool // Force re-index even if unchanged
}

// IndexResult represents the result of an indexing operation.
type IndexResult struct {
	Store        string        `json:"store"`
	Indexed      int           `json:"indexed"`
	Skipped      int           `json:"skipped"`
	Failed       int           `json:"failed"`
	ChunksTotal  int           `json:"chunks_total"`
	Duration     time.Duration `json:"duration"`
	Errors       []IndexError  `json:"errors,omitempty"`
	DocumentInfo []DocInfo     `json:"document_info,omitempty"`
}

// IndexError represents an error during indexing.
type IndexError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

// DocInfo contains info about an indexed document.
type DocInfo struct {
	Path       string `json:"path"`
	Hash       string `json:"hash"`
	ChunkCount int    `json:"chunk_count"`
	Status     string `json:"status"` // indexed, skipped, failed
}

// Index indexes documents into a store.
func (p *Pipeline) Index(ctx context.Context, req IndexRequest) (*IndexResult, error) {
	start := time.Now()

	result := &IndexResult{
		Store:        req.Store,
		DocumentInfo: make([]DocInfo, 0, len(req.Documents)),
	}

	if len(req.Documents) == 0 {
		return result, nil
	}

	// Validate store exists
	exists, err := p.qdrant.CollectionExists(ctx, req.Store)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "failed to check collection", err)
	}
	if !exists {
		return nil, errors.NotFoundError(fmt.Sprintf("store: %s", req.Store))
	}

	// Filter documents
	var toIndex []*Document
	for _, doc := range req.Documents {
		if err := ValidateDocument(doc); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, IndexError{
				Path:    doc.Path,
				Message: err.Error(),
			})
			result.DocumentInfo = append(result.DocumentInfo, DocInfo{
				Path:   doc.Path,
				Status: "failed",
			})
			continue
		}

		// Check if document changed
		if p.cfg.SkipExisting && !req.Force {
			if p.tracker.HasHash(req.Store, doc.Path, doc.Hash) {
				result.Skipped++
				result.DocumentInfo = append(result.DocumentInfo, DocInfo{
					Path:   doc.Path,
					Hash:   doc.Hash,
					Status: "skipped",
				})
				continue
			}
		}

		toIndex = append(toIndex, doc)
	}

	if len(toIndex) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Process documents in batches
	chunks, docInfos, errs := p.processDocuments(ctx, req.Store, toIndex)
	result.Errors = append(result.Errors, errs...)
	result.Failed += len(errs)
	result.DocumentInfo = append(result.DocumentInfo, docInfos...)

	if len(chunks) == 0 {
		result.Duration = time.Since(start)
		return result, nil
	}

	// Generate embeddings
	points, err := p.generateEmbeddings(ctx, chunks)
	if err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "embedding generation failed", err)
	}

	// Upsert to Qdrant
	if err := p.upsertPoints(ctx, req.Store, points); err != nil {
		return nil, errors.Wrap(errors.CodeInternal, "qdrant upsert failed", err)
	}

	// Update tracker
	for _, doc := range toIndex {
		if !containsError(result.Errors, doc.Path) {
			p.tracker.SetHash(req.Store, doc.Path, doc.Hash)
			result.Indexed++
		}
	}

	result.ChunksTotal = len(chunks)
	result.Duration = time.Since(start)

	p.log.Info("Indexing complete",
		"store", req.Store,
		"indexed", result.Indexed,
		"skipped", result.Skipped,
		"failed", result.Failed,
		"chunks", result.ChunksTotal,
		"duration", result.Duration,
	)

	return result, nil
}

// processDocuments converts documents to chunks.
func (p *Pipeline) processDocuments(ctx context.Context, store string, docs []*Document) ([]*Chunk, []DocInfo, []IndexError) {
	var allChunks []*Chunk
	var infos []DocInfo
	var errs []IndexError

	for _, doc := range docs {
		// Extract symbols for the document
		doc.Symbols = ExtractSymbols(doc.Content, doc.Language)

		// Chunk the document
		chunks := p.chunker.ChunkDocument(store, doc)

		if len(chunks) == 0 {
			errs = append(errs, IndexError{
				Path:    doc.Path,
				Message: "no chunks generated",
			})
			infos = append(infos, DocInfo{
				Path:   doc.Path,
				Status: "failed",
			})
			continue
		}

		allChunks = append(allChunks, chunks...)
		infos = append(infos, DocInfo{
			Path:       doc.Path,
			Hash:       doc.Hash,
			ChunkCount: len(chunks),
			Status:     "indexed",
		})
	}

	return allChunks, infos, errs
}

// generateEmbeddings generates dense and sparse embeddings for chunks.
func (p *Pipeline) generateEmbeddings(ctx context.Context, chunks []*Chunk) ([]qdrant.Point, error) {
	if len(chunks) == 0 {
		return nil, nil
	}

	// Extract content for embedding
	contents := make([]string, len(chunks))
	for i, chunk := range chunks {
		contents[i] = chunk.Content
	}

	// Generate dense embeddings
	denseEmbeddings, err := p.ml.Embed(ctx, contents)
	if err != nil {
		return nil, fmt.Errorf("dense embedding failed: %w", err)
	}

	// Generate sparse embeddings
	sparseVectors, err := p.ml.SparseEncode(ctx, contents)
	if err != nil {
		return nil, fmt.Errorf("sparse encoding failed: %w", err)
	}

	// Build points
	points := make([]qdrant.Point, len(chunks))
	for i, chunk := range chunks {
		points[i] = qdrant.Point{
			ID:          chunk.ID,
			DenseVector: denseEmbeddings[i],
			Payload: qdrant.PointPayload{
				Store:        chunk.Store,
				Path:         chunk.Path,
				Language:     chunk.Language,
				Content:      chunk.Content,
				Symbols:      chunk.Symbols,
				StartLine:    chunk.StartLine,
				EndLine:      chunk.EndLine,
				DocumentHash: chunk.DocumentID,
				ChunkHash:    chunk.Hash,
				IndexedAt:    chunk.IndexedAt,
			},
		}

		if i < len(sparseVectors) {
			points[i].SparseIndices = sparseVectors[i].Indices
			points[i].SparseValues = sparseVectors[i].Values
		}
	}

	return points, nil
}

// upsertPoints uploads points to Qdrant in batches.
func (p *Pipeline) upsertPoints(ctx context.Context, store string, points []qdrant.Point) error {
	return p.qdrant.UpsertPointsBatch(ctx, store, points, p.cfg.UpsertBatchSize)
}

// Delete removes documents from the index.
func (p *Pipeline) Delete(ctx context.Context, store string, paths []string) error {
	for _, path := range paths {
		if err := p.qdrant.DeletePoints(ctx, store, qdrant.DeleteFilter{
			Path: path,
		}); err != nil {
			return fmt.Errorf("failed to delete %s: %w", path, err)
		}

		p.tracker.RemovePath(store, path)
	}

	return nil
}

// DeleteByPrefix removes documents by path prefix.
func (p *Pipeline) DeleteByPrefix(ctx context.Context, store, prefix string) error {
	if err := p.qdrant.DeletePoints(ctx, store, qdrant.DeleteFilter{
		PathPrefix: prefix,
	}); err != nil {
		return fmt.Errorf("failed to delete by prefix %s: %w", prefix, err)
	}

	p.tracker.RemoveByPrefix(store, prefix)
	return nil
}

// Sync removes documents that no longer exist.
func (p *Pipeline) Sync(ctx context.Context, store string, currentPaths []string) (int, error) {
	// Get all indexed paths from tracker
	indexedPaths := p.tracker.GetPaths(store)

	// Build set of current paths
	currentSet := make(map[string]struct{}, len(currentPaths))
	for _, path := range currentPaths {
		currentSet[path] = struct{}{}
	}

	// Find paths to remove
	var toRemove []string
	for _, path := range indexedPaths {
		if _, exists := currentSet[path]; !exists {
			toRemove = append(toRemove, path)
		}
	}

	// Delete removed paths
	if len(toRemove) > 0 {
		if err := p.Delete(ctx, store, toRemove); err != nil {
			return 0, err
		}
	}

	return len(toRemove), nil
}

// Reindex clears and rebuilds the entire index.
func (p *Pipeline) Reindex(ctx context.Context, req IndexRequest) (*IndexResult, error) {
	// Clear existing index
	if err := p.qdrant.DeletePoints(ctx, req.Store, qdrant.DeleteFilter{}); err != nil {
		// Ignore "no criteria" error - collection might be empty
		p.log.Debug("Delete returned", "error", err)
	}

	// Clear tracker
	p.tracker.ClearStore(req.Store)

	// Force reindex all
	req.Force = true
	return p.Index(ctx, req)
}

// GetStats returns indexing statistics for a store.
func (p *Pipeline) GetStats(ctx context.Context, store string) (*IndexStats, error) {
	count, err := p.qdrant.CountPoints(ctx, store, nil)
	if err != nil {
		return nil, err
	}

	paths := p.tracker.GetPaths(store)

	return &IndexStats{
		Store:       store,
		TotalChunks: int(count),
		TotalFiles:  len(paths),
	}, nil
}

// IndexStats contains statistics about an index.
type IndexStats struct {
	Store       string `json:"store"`
	TotalChunks int    `json:"total_chunks"`
	TotalFiles  int    `json:"total_files"`
}

// containsError checks if an error exists for a path.
func containsError(errs []IndexError, path string) bool {
	for _, e := range errs {
		if e.Path == path {
			return true
		}
	}
	return false
}
