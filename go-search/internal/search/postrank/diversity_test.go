package postrank

import (
	"context"
	"testing"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

func TestApplyMMR(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewDiversityService(0.7, log)

	// Create results where some are similar
	results := []ResultWithEmbedding{
		{ID: "1", Score: 1.0, Embedding: []float32{1.0, 0.0, 0.0}},    // Most relevant
		{ID: "2", Score: 0.95, Embedding: []float32{0.99, 0.01, 0.0}}, // Similar to #1, high score
		{ID: "3", Score: 0.9, Embedding: []float32{0.0, 1.0, 0.0}},    // Different, high score
		{ID: "4", Score: 0.85, Embedding: []float32{0.0, 0.99, 0.01}}, // Similar to #3
	}

	ctx := context.Background()
	reordered, stats := svc.ApplyMMR(ctx, results, 3)

	// Should select 1 (most relevant), then 3 (diverse), not 2 (too similar to 1)
	if len(reordered) != 3 {
		t.Errorf("Expected 3 results, got %d", len(reordered))
	}

	if reordered[0].ID != "1" {
		t.Errorf("First result should be most relevant (1), got %s", reordered[0].ID)
	}

	// Second should NOT be #2 (too similar to #1)
	if reordered[1].ID == "2" {
		t.Errorf("Second result should not be #2 (too similar to #1)")
	}

	if !stats.Enabled {
		t.Error("Expected diversity to be enabled")
	}
}

func TestApplyMMR_EmptyInput(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewDiversityService(0.7, log)

	ctx := context.Background()
	reordered, stats := svc.ApplyMMR(ctx, nil, 10)

	if len(reordered) != 0 {
		t.Errorf("Expected empty results, got %d", len(reordered))
	}

	if stats.Enabled {
		t.Error("Expected diversity to be disabled for empty input")
	}
}

func TestApplyMMR_SingleResult(t *testing.T) {
	log := logger.New("error", "text")
	svc := NewDiversityService(0.7, log)

	results := []ResultWithEmbedding{
		{ID: "1", Score: 1.0, Embedding: []float32{1.0, 0.0, 0.0}},
	}

	ctx := context.Background()
	reordered, stats := svc.ApplyMMR(ctx, results, 10)

	if len(reordered) != 1 {
		t.Errorf("Expected 1 result, got %d", len(reordered))
	}

	if reordered[0].ID != "1" {
		t.Errorf("Expected result 1, got %s", reordered[0].ID)
	}

	if !stats.Enabled {
		t.Error("Expected diversity to be enabled")
	}
}

func TestApplyMMR_LambdaExtreme(t *testing.T) {
	log := logger.New("error", "text")

	results := []ResultWithEmbedding{
		{ID: "1", Score: 1.0, Embedding: []float32{1.0, 0.0, 0.0}},
		{ID: "2", Score: 0.9, Embedding: []float32{0.99, 0.01, 0.0}}, // Very similar to #1
		{ID: "3", Score: 0.5, Embedding: []float32{0.0, 1.0, 0.0}},   // Different but lower score
	}

	ctx := context.Background()

	// Lambda = 1.0 (max relevance) - should prefer #2 over #3
	svcRelevance := NewDiversityService(1.0, log)
	reorderedRelevance, _ := svcRelevance.ApplyMMR(ctx, results, 3)

	// Lambda = 0.0 (max diversity) - should prefer #3 over #2
	svcDiversity := NewDiversityService(0.0, log)
	reorderedDiversity, _ := svcDiversity.ApplyMMR(ctx, results, 3)

	// With lambda=1, second should be #2 (similar but high score)
	if reorderedRelevance[1].ID != "2" {
		t.Errorf("With lambda=1, second should be #2, got %s", reorderedRelevance[1].ID)
	}

	// With lambda=0, second should be #3 (diverse)
	if reorderedDiversity[1].ID != "3" {
		t.Errorf("With lambda=0, second should be #3, got %s", reorderedDiversity[1].ID)
	}
}

func TestEstimateDiversity(t *testing.T) {
	tests := []struct {
		name    string
		results []ResultWithEmbedding
		minDiv  float32
		maxDiv  float32
	}{
		{
			name: "identical results",
			results: []ResultWithEmbedding{
				{Embedding: []float32{1.0, 0.0, 0.0}},
				{Embedding: []float32{1.0, 0.0, 0.0}},
			},
			minDiv: 0.0,
			maxDiv: 0.1,
		},
		{
			name: "orthogonal results",
			results: []ResultWithEmbedding{
				{Embedding: []float32{1.0, 0.0, 0.0}},
				{Embedding: []float32{0.0, 1.0, 0.0}},
			},
			minDiv: 0.9,
			maxDiv: 1.0,
		},
		{
			name:    "single result",
			results: []ResultWithEmbedding{{Embedding: []float32{1.0, 0.0, 0.0}}},
			minDiv:  1.0,
			maxDiv:  1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diversity := EstimateDiversity(tt.results)
			if diversity < tt.minDiv || diversity > tt.maxDiv {
				t.Errorf("Expected diversity in range [%f, %f], got %f", tt.minDiv, tt.maxDiv, diversity)
			}
		})
	}
}
