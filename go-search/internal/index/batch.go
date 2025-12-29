package index

import (
	"context"
	"sync"
)

// BatchConfig configures batch processing.
type BatchConfig struct {
	// Size is the maximum batch size.
	Size int

	// Workers is the number of parallel workers.
	Workers int
}

// DefaultBatchConfig returns sensible defaults.
func DefaultBatchConfig() BatchConfig {
	return BatchConfig{
		Size:    32,
		Workers: 4,
	}
}

// BatchProcessor processes items in batches with optional parallelism.
type BatchProcessor[T any, R any] struct {
	cfg     BatchConfig
	process func(ctx context.Context, batch []T) ([]R, error)
	mu      sync.Mutex
	results []R
	errors  []error
}

// NewBatchProcessor creates a new batch processor.
func NewBatchProcessor[T any, R any](cfg BatchConfig, process func(ctx context.Context, batch []T) ([]R, error)) *BatchProcessor[T, R] {
	if cfg.Size <= 0 {
		cfg = DefaultBatchConfig()
	}
	return &BatchProcessor[T, R]{
		cfg:     cfg,
		process: process,
	}
}

// Process processes all items and returns results.
func (p *BatchProcessor[T, R]) Process(ctx context.Context, items []T) ([]R, error) {
	if len(items) == 0 {
		return nil, nil
	}

	// Split into batches
	batches := splitIntoBatches(items, p.cfg.Size)

	// Process with workers
	if p.cfg.Workers <= 1 {
		return p.processSequential(ctx, batches)
	}

	return p.processParallel(ctx, batches)
}

func (p *BatchProcessor[T, R]) processSequential(ctx context.Context, batches [][]T) ([]R, error) {
	var results []R

	for _, batch := range batches {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		batchResults, err := p.process(ctx, batch)
		if err != nil {
			return results, err
		}
		results = append(results, batchResults...)
	}

	return results, nil
}

func (p *BatchProcessor[T, R]) processParallel(ctx context.Context, batches [][]T) ([]R, error) {
	results := make([][]R, len(batches))
	errors := make([]error, len(batches))

	// Worker pool
	sem := make(chan struct{}, p.cfg.Workers)
	var wg sync.WaitGroup

	for i, batch := range batches {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire

		go func(idx int, b []T) {
			defer wg.Done()
			defer func() { <-sem }() // Release

			r, err := p.process(ctx, b)
			results[idx] = r
			errors[idx] = err
		}(i, batch)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return nil, err
		}
	}

	// Flatten results
	var flat []R
	for _, r := range results {
		flat = append(flat, r...)
	}

	return flat, nil
}

// splitIntoBatches splits items into batches of the given size.
func splitIntoBatches[T any](items []T, size int) [][]T {
	if size <= 0 {
		size = 1
	}

	var batches [][]T
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	return batches
}

// ChunkBatch groups chunks for batch processing while respecting size limits.
type ChunkBatch struct {
	Chunks   []*Chunk
	Contents []string
	IDs      []string
}

// BatchChunks groups chunks into batches for efficient embedding.
func BatchChunks(chunks []*Chunk, batchSize int) []*ChunkBatch {
	if batchSize <= 0 {
		batchSize = 32
	}

	batches := splitIntoBatches(chunks, batchSize)
	result := make([]*ChunkBatch, len(batches))

	for i, batch := range batches {
		contents := make([]string, len(batch))
		ids := make([]string, len(batch))

		for j, chunk := range batch {
			contents[j] = chunk.Content
			ids[j] = chunk.ID
		}

		result[i] = &ChunkBatch{
			Chunks:   batch,
			Contents: contents,
			IDs:      ids,
		}
	}

	return result
}

// ProgressCallback is called with indexing progress updates.
type ProgressCallback func(Progress)

// Progress represents indexing progress.
type Progress struct {
	Stage       string  `json:"stage"`   // chunking, embedding, upserting
	Current     int     `json:"current"` // Current item
	Total       int     `json:"total"`   // Total items
	Percent     float64 `json:"percent"` // Completion percentage
	CurrentFile string  `json:"current_file,omitempty"`
	Message     string  `json:"message,omitempty"`
}

// ProgressTracker tracks progress across stages.
type ProgressTracker struct {
	callback ProgressCallback
	mu       sync.Mutex
}

// NewProgressTracker creates a new progress tracker.
func NewProgressTracker(callback ProgressCallback) *ProgressTracker {
	return &ProgressTracker{
		callback: callback,
	}
}

// Update updates progress.
func (t *ProgressTracker) Update(p Progress) {
	if t.callback == nil {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	if p.Total > 0 {
		p.Percent = float64(p.Current) / float64(p.Total) * 100
	}

	t.callback(p)
}

// ChunkStage reports chunking progress.
func (t *ProgressTracker) ChunkStage(current, total int, file string) {
	t.Update(Progress{
		Stage:       "chunking",
		Current:     current,
		Total:       total,
		CurrentFile: file,
		Message:     "Chunking documents",
	})
}

// EmbedStage reports embedding progress.
func (t *ProgressTracker) EmbedStage(current, total int) {
	t.Update(Progress{
		Stage:   "embedding",
		Current: current,
		Total:   total,
		Message: "Generating embeddings",
	})
}

// UpsertStage reports upserting progress.
func (t *ProgressTracker) UpsertStage(current, total int) {
	t.Update(Progress{
		Stage:   "upserting",
		Current: current,
		Total:   total,
		Message: "Storing in vector database",
	})
}

// Complete reports completion.
func (t *ProgressTracker) Complete(indexed, skipped, failed int) {
	t.Update(Progress{
		Stage:   "complete",
		Current: indexed + skipped + failed,
		Total:   indexed + skipped + failed,
		Percent: 100,
		Message: "Indexing complete",
	})
}
