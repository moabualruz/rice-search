package postrank

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

func TestDeduplicate(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewDedupService(0.85, log)

	// Create test results with embeddings
	results := []ResultWithEmbedding{
		{ID: "1", Path: "test.go", Score: 0.9, Embedding: []float32{1.0, 0.0, 0.0}},
		{ID: "2", Path: "test.go", Score: 0.85, Embedding: []float32{0.99, 0.01, 0.0}}, // Very similar to #1
		{ID: "3", Path: "test.go", Score: 0.8, Embedding: []float32{0.0, 1.0, 0.0}},    // Different
		{ID: "4", Path: "test.go", Score: 0.75, Embedding: []float32{0.0, 0.99, 0.01}}, // Very similar to #3
	}

	ctx := context.Background()
	deduped, stats := svc.Deduplicate(ctx, results)

	// Should remove #2 and #4 (duplicates)
	if stats.OutputCount != 2 {
		t.Errorf("Expected 2 results, got %d", stats.OutputCount)
	}

	if stats.Removed != 2 {
		t.Errorf("Expected 2 removed, got %d", stats.Removed)
	}

	// Check that we kept the higher-scoring ones
	if deduped[0].ID != "1" {
		t.Errorf("Expected first result to be ID 1, got %s", deduped[0].ID)
	}
	if deduped[1].ID != "3" {
		t.Errorf("Expected second result to be ID 3, got %s", deduped[1].ID)
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
		delta    float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 1.0,
			delta:    0.001,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{0.0, 1.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1.0, 0.0, 0.0},
			b:        []float32{-1.0, 0.0, 0.0},
			expected: -1.0,
			delta:    0.001,
		},
		{
			name:     "similar vectors",
			a:        []float32{1.0, 0.1, 0.0},
			b:        []float32{0.9, 0.1, 0.0},
			expected: 0.99,
			delta:    0.01,
		},
		{
			name:     "different lengths",
			a:        []float32{1.0, 0.0},
			b:        []float32{1.0, 0.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
		{
			name:     "zero vectors",
			a:        []float32{0.0, 0.0, 0.0},
			b:        []float32{0.0, 0.0, 0.0},
			expected: 0.0,
			delta:    0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			if abs(result-tt.expected) > tt.delta {
				t.Errorf("Expected %f ± %f, got %f", tt.expected, tt.delta, result)
			}
		})
	}
}

func TestDeduplicate_EmptyInput(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewDedupService(0.85, log)

	ctx := context.Background()
	deduped, stats := svc.Deduplicate(ctx, nil)

	if len(deduped) != 0 {
		t.Errorf("Expected empty results, got %d", len(deduped))
	}

	if stats.InputCount != 0 || stats.OutputCount != 0 || stats.Removed != 0 {
		t.Errorf("Expected all stats to be 0, got: %+v", stats)
	}
}

func TestDeduplicate_NoSimilar(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewDedupService(0.85, log)

	// All results are very different
	results := []ResultWithEmbedding{
		{ID: "1", Score: 0.9, Embedding: []float32{1.0, 0.0, 0.0}},
		{ID: "2", Score: 0.8, Embedding: []float32{0.0, 1.0, 0.0}},
		{ID: "3", Score: 0.7, Embedding: []float32{0.0, 0.0, 1.0}},
	}

	ctx := context.Background()
	_, stats := svc.Deduplicate(ctx, results)

	// Should keep all results
	if stats.OutputCount != 3 {
		t.Errorf("Expected 3 results, got %d", stats.OutputCount)
	}

	if stats.Removed != 0 {
		t.Errorf("Expected 0 removed, got %d", stats.Removed)
	}
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
