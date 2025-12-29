package postrank

import (
	"context"
	"sort"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// AggregationService groups results by file path.
type AggregationService struct {
	maxChunksPerFile int
	log              *logger.Logger
}

// NewAggregationService creates a new aggregation service.
func NewAggregationService(maxChunksPerFile int, log *logger.Logger) *AggregationService {
	if maxChunksPerFile <= 0 {
		maxChunksPerFile = 3 // Default
	}
	return &AggregationService{
		maxChunksPerFile: maxChunksPerFile,
		log:              log,
	}
}

// AggregationResult contains aggregation statistics.
type AggregationResult struct {
	UniqueFiles   int
	ChunksDropped int
	LatencyMs     int64
}

// FileGroup represents results grouped by file.
type FileGroup struct {
	Path                     string
	TopChunks                []ResultWithEmbedding
	TotalChunks              int
	AverageScore             float32
	RepresentativeChunkIndex int // Index of the representative chunk in TopChunks
}

// GroupByFile groups results by file path and selects top N chunks per file.
func (s *AggregationService) GroupByFile(ctx context.Context, results []ResultWithEmbedding) ([]ResultWithEmbedding, AggregationResult) {
	start := time.Now()

	if len(results) == 0 {
		return results, AggregationResult{
			UniqueFiles:   0,
			ChunksDropped: 0,
			LatencyMs:     0,
		}
	}

	// Group by file path
	fileGroups := make(map[string][]ResultWithEmbedding)
	fileOrder := make([]string, 0) // Preserve first-seen order

	for _, result := range results {
		path := result.Path
		if _, exists := fileGroups[path]; !exists {
			fileOrder = append(fileOrder, path)
		}
		fileGroups[path] = append(fileGroups[path], result)
	}

	// Process each group
	var aggregated []ResultWithEmbedding
	totalDropped := 0

	for _, path := range fileOrder {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return aggregated, AggregationResult{
				UniqueFiles:   len(fileGroups),
				ChunksDropped: totalDropped,
				LatencyMs:     time.Since(start).Milliseconds(),
			}
		default:
		}

		chunks := fileGroups[path]

		// Sort chunks by score (descending)
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].Score > chunks[j].Score
		})

		// Take top N chunks per file
		selected := chunks
		if len(chunks) > s.maxChunksPerFile {
			selected = chunks[:s.maxChunksPerFile]
			totalDropped += len(chunks) - s.maxChunksPerFile
		}

		aggregated = append(aggregated, selected...)
	}

	return aggregated, AggregationResult{
		UniqueFiles:   len(fileGroups),
		ChunksDropped: totalDropped,
		LatencyMs:     time.Since(start).Milliseconds(),
	}
}

// GroupByFileDetailed groups results by file and returns detailed file groups.
// This provides more information for display purposes.
func (s *AggregationService) GroupByFileDetailed(ctx context.Context, results []ResultWithEmbedding) ([]FileGroup, AggregationResult) {
	start := time.Now()

	if len(results) == 0 {
		return nil, AggregationResult{
			UniqueFiles:   0,
			ChunksDropped: 0,
			LatencyMs:     0,
		}
	}

	// Group by file path
	fileGroups := make(map[string][]ResultWithEmbedding)
	fileOrder := make([]string, 0)

	for _, result := range results {
		path := result.Path
		if _, exists := fileGroups[path]; !exists {
			fileOrder = append(fileOrder, path)
		}
		fileGroups[path] = append(fileGroups[path], result)
	}

	// Build detailed groups
	groups := make([]FileGroup, 0, len(fileGroups))
	totalDropped := 0

	for _, path := range fileOrder {
		chunks := fileGroups[path]

		// Sort chunks by score (descending)
		sort.Slice(chunks, func(i, j int) bool {
			return chunks[i].Score > chunks[j].Score
		})

		// Calculate average score
		var totalScore float32
		for _, chunk := range chunks {
			totalScore += chunk.Score
		}
		avgScore := totalScore / float32(len(chunks))

		// Take top N chunks
		topChunks := chunks
		if len(chunks) > s.maxChunksPerFile {
			topChunks = chunks[:s.maxChunksPerFile]
			totalDropped += len(chunks) - s.maxChunksPerFile
		}

		// Representative chunk is the highest scoring one
		groups = append(groups, FileGroup{
			Path:                     path,
			TopChunks:                topChunks,
			TotalChunks:              len(chunks),
			AverageScore:             avgScore,
			RepresentativeChunkIndex: 0,
		})
	}

	// Sort groups by average score (descending)
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].AverageScore > groups[j].AverageScore
	})

	return groups, AggregationResult{
		UniqueFiles:   len(groups),
		ChunksDropped: totalDropped,
		LatencyMs:     time.Since(start).Milliseconds(),
	}
}

// MergeTopChunks extracts all top chunks from file groups into a flat list.
func MergeTopChunks(groups []FileGroup) []ResultWithEmbedding {
	var merged []ResultWithEmbedding
	for _, group := range groups {
		merged = append(merged, group.TopChunks...)
	}
	return merged
}
