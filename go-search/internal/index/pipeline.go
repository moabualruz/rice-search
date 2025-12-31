package index

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ricesearch/rice-search/internal/bus"
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
	bus     bus.Bus
	chunker *Chunker
	tracker *Tracker
	log     *logger.Logger
	mu      sync.RWMutex
}

// NewPipeline creates a new indexing pipeline.
// eventBus is optional - if nil, event publishing is disabled.
func NewPipeline(cfg PipelineConfig, mlSvc ml.Service, qc *qdrant.Client, log *logger.Logger, eventBus bus.Bus) *Pipeline {
	if cfg.EmbedBatchSize == 0 {
		cfg = DefaultPipelineConfig()
	}

	return &Pipeline{
		cfg:     cfg,
		ml:      mlSvc,
		qdrant:  qc,
		bus:     eventBus,
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
	p.publishIndexEvent(ctx, &IndexResult{ // Send initial "started" event or just rely on progress?
		// Actually let's use the new progress event
	})

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

	// Publish index response event for metrics
	p.publishIndexEvent(ctx, result)

	return result, nil
}

// publishIndexEvent publishes an index response event to the event bus.
func (p *Pipeline) publishIndexEvent(ctx context.Context, result *IndexResult) {
	if p.bus == nil {
		return
	}

	event := bus.Event{
		Type:   bus.TopicIndexResponse,
		Source: "index",
		Payload: map[string]interface{}{
			"store":        result.Store,
			"indexed":      result.Indexed,
			"skipped":      result.Skipped,
			"failed":       result.Failed,
			"chunks_total": result.ChunksTotal,
			"duration_ms":  result.Duration.Milliseconds(),
		},
	}
	if err := p.bus.Publish(ctx, bus.TopicIndexResponse, event); err != nil {
		p.log.Debug("Failed to publish index event", "error", err)
	}
}

// publishIndexProgress publishes an index progress event.
func (p *Pipeline) publishIndexProgress(ctx context.Context, store, stage string, current, total int) {
	if p.bus == nil {
		return
	}

	percent := 0
	if total > 0 {
		percent = int(float64(current) / float64(total) * 100)
	}

	event := bus.Event{
		Type:   bus.TopicIndexProgress,
		Source: "index",
		Payload: map[string]interface{}{
			"store":      store,
			"stage":      stage,
			"current":    current,
			"total":      total,
			"percentage": percent,
		},
	}
	// Use fire-and-forget to avoid blocking
	go func() {
		_ = p.bus.Publish(context.Background(), bus.TopicIndexProgress, event)
	}()
}

// publishChunkCreatedEvent publishes a chunk.created event to the event bus.
func (p *Pipeline) publishChunkCreatedEvent(ctx context.Context, chunk *Chunk) {
	if p.bus == nil {
		return
	}

	event := bus.Event{
		Type:   bus.TopicChunkCreated,
		Source: "index",
		Payload: map[string]interface{}{
			"store":         chunk.Store,
			"chunk_id":      chunk.ID,
			"document_id":   chunk.DocumentID,
			"path":          chunk.Path,
			"language":      chunk.Language,
			"start_line":    chunk.StartLine,
			"end_line":      chunk.EndLine,
			"token_count":   chunk.TokenCount,
			"connection_id": chunk.ConnectionID,
		},
	}

	if err := p.bus.Publish(ctx, bus.TopicChunkCreated, event); err != nil {
		p.log.Debug("Failed to publish chunk.created event", "error", err, "chunk_id", chunk.ID)
	}
}

// processDocuments converts documents to chunks.
func (p *Pipeline) processDocuments(ctx context.Context, store string, docs []*Document) ([]*Chunk, []DocInfo, []IndexError) {
	var allChunks []*Chunk
	var infos []DocInfo
	var errs []IndexError

	for i, doc := range docs {
		// Publish progress every 10 docs or if last
		if i%10 == 0 || i == len(docs)-1 {
			p.publishIndexProgress(ctx, store, "processing", i+1, len(docs))
		}

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

		// Publish chunk.created events for each chunk
		for _, chunk := range chunks {
			p.publishChunkCreatedEvent(ctx, chunk)
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

	// Generate dense embeddings via event bus
	denseEmbeddings, err := p.embedViaEventBus(ctx, contents)
	if err != nil {
		return nil, fmt.Errorf("dense embedding failed: %w", err)
	}

	// Generate sparse embeddings via event bus
	sparseVectors, err := p.sparseEncodeViaEventBus(ctx, contents)
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
				ConnectionID: chunk.ConnectionID,
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

// ListFiles returns paginated list of indexed files for a store.
func (p *Pipeline) ListFiles(store string, page, pageSize int) ([]FileInfo, int) {
	return p.tracker.ListFiles(store, page, pageSize)
}

// embedViaEventBus generates embeddings using the event bus.
// Falls back to direct ML service call if event bus is unavailable.
func (p *Pipeline) embedViaEventBus(ctx context.Context, texts []string) ([][]float32, error) {
	// If no event bus, fall back to direct call
	if p.bus == nil {
		return p.ml.Embed(ctx, texts)
	}

	// Create request event
	correlationID := fmt.Sprintf("index-embed-%d", time.Now().UnixNano())
	req := bus.Event{
		ID:            correlationID,
		Type:          bus.TopicEmbedRequest,
		Source:        "index",
		Timestamp:     time.Now().UnixNano(),
		CorrelationID: correlationID,
		Payload: map[string]interface{}{
			"texts": texts,
		},
	}

	// Send request and wait for response
	resp, err := p.bus.Request(ctx, bus.TopicEmbedRequest, req)
	if err != nil {
		p.log.Debug("Event bus embed request failed, falling back to direct call", "error", err)
		return p.ml.Embed(ctx, texts)
	}

	// Parse response
	embeddings, err := parseEmbedResponse(resp)
	if err != nil {
		p.log.Debug("Failed to parse embed response, falling back to direct call", "error", err)
		return p.ml.Embed(ctx, texts)
	}

	return embeddings, nil
}

// sparseEncodeViaEventBus generates sparse vectors using the event bus.
// Falls back to direct ML service call if event bus is unavailable.
func (p *Pipeline) sparseEncodeViaEventBus(ctx context.Context, texts []string) ([]ml.SparseVector, error) {
	// If no event bus, fall back to direct call
	if p.bus == nil {
		return p.ml.SparseEncode(ctx, texts)
	}

	// Create request event
	correlationID := fmt.Sprintf("index-sparse-%d", time.Now().UnixNano())
	req := bus.Event{
		ID:            correlationID,
		Type:          bus.TopicSparseRequest,
		Source:        "index",
		Timestamp:     time.Now().UnixNano(),
		CorrelationID: correlationID,
		Payload: map[string]interface{}{
			"texts": texts,
		},
	}

	// Send request and wait for response
	resp, err := p.bus.Request(ctx, bus.TopicSparseRequest, req)
	if err != nil {
		p.log.Debug("Event bus sparse request failed, falling back to direct call", "error", err)
		return p.ml.SparseEncode(ctx, texts)
	}

	// Parse response
	vectors, err := parseSparseResponse(resp)
	if err != nil {
		p.log.Debug("Failed to parse sparse response, falling back to direct call", "error", err)
		return p.ml.SparseEncode(ctx, texts)
	}

	return vectors, nil
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

		if indicesRaw, ok := m["indices"]; ok {
			result[i].Indices = convertToUint32Slice(indicesRaw)
		}
		if valuesRaw, ok := m["values"]; ok {
			result[i].Values = convertToFloat32Slice(valuesRaw)
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
