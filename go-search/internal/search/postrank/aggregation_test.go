package postrank

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

func TestGroupByFile(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewAggregationService(2, log) // Max 2 chunks per file

	results := []ResultWithEmbedding{
		{ID: "1", Path: "a.go", Score: 1.0, Embedding: []float32{1.0, 0.0}},
		{ID: "2", Path: "a.go", Score: 0.9, Embedding: []float32{0.9, 0.1}},
		{ID: "3", Path: "a.go", Score: 0.8, Embedding: []float32{0.8, 0.2}}, // Should be dropped
		{ID: "4", Path: "b.go", Score: 0.7, Embedding: []float32{0.7, 0.3}},
		{ID: "5", Path: "b.go", Score: 0.6, Embedding: []float32{0.6, 0.4}},
	}

	ctx := context.Background()
	grouped, stats := svc.GroupByFile(ctx, results)

	// Should have 4 results (2 from a.go, 2 from b.go)
	if len(grouped) != 4 {
		t.Errorf("Expected 4 results, got %d", len(grouped))
	}

	// Should have dropped 1 chunk
	if stats.ChunksDropped != 1 {
		t.Errorf("Expected 1 chunk dropped, got %d", stats.ChunksDropped)
	}

	// Should have 2 unique files
	if stats.UniqueFiles != 2 {
		t.Errorf("Expected 2 unique files, got %d", stats.UniqueFiles)
	}

	// Check that highest scoring chunks are kept
	fileA := 0
	fileB := 0
	for _, r := range grouped {
		if r.Path == "a.go" {
			fileA++
			if r.ID == "3" {
				t.Error("Lowest scoring chunk from a.go should have been dropped")
			}
		} else if r.Path == "b.go" {
			fileB++
		}
	}

	if fileA != 2 || fileB != 2 {
		t.Errorf("Expected 2 chunks per file, got %d from a.go and %d from b.go", fileA, fileB)
	}
}

func TestGroupByFile_EmptyInput(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewAggregationService(3, log)

	ctx := context.Background()
	grouped, stats := svc.GroupByFile(ctx, nil)

	if len(grouped) != 0 {
		t.Errorf("Expected empty results, got %d", len(grouped))
	}

	if stats.UniqueFiles != 0 || stats.ChunksDropped != 0 {
		t.Errorf("Expected all stats to be 0, got: %+v", stats)
	}
}

func TestGroupByFile_NoDrops(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewAggregationService(5, log) // High limit

	results := []ResultWithEmbedding{
		{ID: "1", Path: "a.go", Score: 1.0, Embedding: []float32{1.0, 0.0}},
		{ID: "2", Path: "a.go", Score: 0.9, Embedding: []float32{0.9, 0.1}},
		{ID: "3", Path: "b.go", Score: 0.8, Embedding: []float32{0.8, 0.2}},
	}

	ctx := context.Background()
	grouped, stats := svc.GroupByFile(ctx, results)

	// Should keep all results
	if len(grouped) != 3 {
		t.Errorf("Expected 3 results, got %d", len(grouped))
	}

	if stats.ChunksDropped != 0 {
		t.Errorf("Expected 0 chunks dropped, got %d", stats.ChunksDropped)
	}
}

func TestGroupByFileDetailed(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewAggregationService(2, log)

	results := []ResultWithEmbedding{
		{ID: "1", Path: "a.go", Score: 1.0, Embedding: []float32{1.0, 0.0}},
		{ID: "2", Path: "a.go", Score: 0.9, Embedding: []float32{0.9, 0.1}},
		{ID: "3", Path: "a.go", Score: 0.8, Embedding: []float32{0.8, 0.2}},
		{ID: "4", Path: "b.go", Score: 0.7, Embedding: []float32{0.7, 0.3}},
	}

	ctx := context.Background()
	groups, stats := svc.GroupByFileDetailed(ctx, results)

	if len(groups) != 2 {
		t.Errorf("Expected 2 file groups, got %d", len(groups))
	}

	// First group should be a.go (higher average score)
	if groups[0].Path != "a.go" {
		t.Errorf("Expected first group to be a.go, got %s", groups[0].Path)
	}

	// a.go should have 3 total chunks, 2 top chunks
	if groups[0].TotalChunks != 3 {
		t.Errorf("Expected 3 total chunks for a.go, got %d", groups[0].TotalChunks)
	}

	if len(groups[0].TopChunks) != 2 {
		t.Errorf("Expected 2 top chunks for a.go, got %d", len(groups[0].TopChunks))
	}

	// Representative chunk should be index 0 (highest scoring)
	if groups[0].RepresentativeChunkIndex != 0 {
		t.Errorf("Expected representative chunk at index 0, got %d", groups[0].RepresentativeChunkIndex)
	}

	// Check stats
	if stats.ChunksDropped != 1 {
		t.Errorf("Expected 1 chunk dropped, got %d", stats.ChunksDropped)
	}
}

func TestMergeTopChunks(t *testing.T) {
	groups := []FileGroup{
		{
			Path: "a.go",
			TopChunks: []ResultWithEmbedding{
				{ID: "1", Score: 1.0},
				{ID: "2", Score: 0.9},
			},
		},
		{
			Path: "b.go",
			TopChunks: []ResultWithEmbedding{
				{ID: "3", Score: 0.8},
			},
		},
	}

	merged := MergeTopChunks(groups)

	if len(merged) != 3 {
		t.Errorf("Expected 3 merged results, got %d", len(merged))
	}

	// Check order is preserved
	if merged[0].ID != "1" || merged[1].ID != "2" || merged[2].ID != "3" {
		t.Error("Merge did not preserve order")
	}
}
