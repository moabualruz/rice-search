package search

import (
	"testing"

	"github.com/ricesearch/rice-search/internal/qdrant"
	"github.com/ricesearch/rice-search/internal/search/fusion"
)

// TestFusionWeightsAffectResults verifies that different fusion weights
// produce different result orderings.
func TestFusionWeightsAffectResults(t *testing.T) {
	// Simulate two retrievers with different preferences
	sparseResults := []qdrant.SearchResult{
		{ID: "doc1", Score: 100.0, Payload: qdrant.PointPayload{Content: "exact match keywords"}},
		{ID: "doc2", Score: 50.0, Payload: qdrant.PointPayload{Content: "partial match"}},
		{ID: "doc3", Score: 25.0, Payload: qdrant.PointPayload{Content: "weak match"}},
	}

	denseResults := []qdrant.SearchResult{
		{ID: "doc3", Score: 0.95, Payload: qdrant.PointPayload{Content: "semantically similar"}},
		{ID: "doc2", Score: 0.85, Payload: qdrant.PointPayload{Content: "somewhat similar"}},
		{ID: "doc1", Score: 0.75, Payload: qdrant.PointPayload{Content: "weakly similar"}},
	}

	// Test equal weights (doc1 and doc3 compete)
	equalCfg := fusion.RRFConfig{
		K:            60,
		SparseWeight: 0.5,
		DenseWeight:  0.5,
	}
	equalResults := fusion.Fuse(sparseResults, denseResults, equalCfg)

	// Test sparse-heavy weights (should favor doc1)
	sparseHeavyCfg := fusion.RRFConfig{
		K:            60,
		SparseWeight: 0.9,
		DenseWeight:  0.1,
	}
	sparseHeavyResults := fusion.Fuse(sparseResults, denseResults, sparseHeavyCfg)

	// Test dense-heavy weights (should favor doc3)
	denseHeavyCfg := fusion.RRFConfig{
		K:            60,
		SparseWeight: 0.1,
		DenseWeight:  0.9,
	}
	denseHeavyResults := fusion.Fuse(sparseResults, denseResults, denseHeavyCfg)

	// Verify equal weights produces balanced ranking
	t.Logf("Equal weights - Top 3: %s, %s, %s",
		equalResults[0].Result.ID,
		equalResults[1].Result.ID,
		equalResults[2].Result.ID)

	// Verify sparse-heavy prefers sparse winner
	if sparseHeavyResults[0].Result.ID != "doc1" {
		t.Errorf("Sparse-heavy weights: expected doc1 first, got %s", sparseHeavyResults[0].Result.ID)
	}
	t.Logf("Sparse-heavy (0.9/0.1) - Top result: %s (fused=%.5f, sparse_rank=%d, dense_rank=%d)",
		sparseHeavyResults[0].Result.ID,
		sparseHeavyResults[0].FusedScore,
		sparseHeavyResults[0].SparseRank,
		sparseHeavyResults[0].DenseRank)

	// Verify dense-heavy prefers dense winner
	if denseHeavyResults[0].Result.ID != "doc3" {
		t.Errorf("Dense-heavy weights: expected doc3 first, got %s", denseHeavyResults[0].Result.ID)
	}
	t.Logf("Dense-heavy (0.1/0.9) - Top result: %s (fused=%.5f, sparse_rank=%d, dense_rank=%d)",
		denseHeavyResults[0].Result.ID,
		denseHeavyResults[0].FusedScore,
		denseHeavyResults[0].SparseRank,
		denseHeavyResults[0].DenseRank)

	// Verify scores change with different weights
	doc1EqualScore := equalResults[0].FusedScore
	doc1SparseScore := sparseHeavyResults[0].FusedScore
	if doc1EqualScore == doc1SparseScore {
		t.Errorf("Expected different scores with different weights, got same score: %.5f", doc1EqualScore)
	}
}

// TestIsBalancedDetection verifies the IsBalanced() method correctly identifies
// when to use native RRF vs manual fusion.
func TestIsBalancedDetection(t *testing.T) {
	tests := []struct {
		name            string
		cfg             fusion.RRFConfig
		shouldUseManual bool
	}{
		{
			name:            "Default equal weights",
			cfg:             fusion.RRFConfig{SparseWeight: 0.5, DenseWeight: 0.5},
			shouldUseManual: false,
		},
		{
			name:            "Slightly unbalanced (within tolerance)",
			cfg:             fusion.RRFConfig{SparseWeight: 0.48, DenseWeight: 0.52},
			shouldUseManual: false,
		},
		{
			name:            "Sparse-heavy",
			cfg:             fusion.RRFConfig{SparseWeight: 0.7, DenseWeight: 0.3},
			shouldUseManual: true,
		},
		{
			name:            "Dense-heavy",
			cfg:             fusion.RRFConfig{SparseWeight: 0.3, DenseWeight: 0.7},
			shouldUseManual: true,
		},
		{
			name:            "Extreme sparse-only",
			cfg:             fusion.RRFConfig{SparseWeight: 1.0, DenseWeight: 0.0},
			shouldUseManual: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBalanced := tt.cfg.IsBalanced()
			shouldUseManual := !isBalanced

			if shouldUseManual != tt.shouldUseManual {
				t.Errorf("IsBalanced() = %v (manual=%v), want manual=%v for weights %.2f/%.2f",
					isBalanced, shouldUseManual, tt.shouldUseManual, tt.cfg.SparseWeight, tt.cfg.DenseWeight)
			}
		})
	}
}
