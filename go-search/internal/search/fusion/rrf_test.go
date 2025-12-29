package fusion

import (
	"testing"

	"github.com/ricesearch/rice-search/internal/qdrant"
)

func TestFuse_EqualWeights(t *testing.T) {
	sparse := []qdrant.SearchResult{
		{ID: "doc1", Score: 10.0},
		{ID: "doc2", Score: 8.0},
		{ID: "doc3", Score: 6.0},
	}

	dense := []qdrant.SearchResult{
		{ID: "doc2", Score: 0.95},
		{ID: "doc1", Score: 0.90},
		{ID: "doc4", Score: 0.85},
	}

	cfg := RRFConfig{
		K:            60,
		SparseWeight: 0.5,
		DenseWeight:  0.5,
	}

	results := Fuse(sparse, dense, cfg)

	// Verify we have 4 unique documents
	if len(results) != 4 {
		t.Errorf("expected 4 results, got %d", len(results))
	}

	// Calculate expected scores:
	// doc1: 0.5/(60+1) + 0.5/(60+2) = 0.5/61 + 0.5/62 = 0.00820 + 0.00806 = 0.01626
	// doc2: 0.5/(60+2) + 0.5/(60+1) = 0.5/62 + 0.5/61 = 0.00806 + 0.00820 = 0.01626
	// Both should be equal, so doc1 comes first (stable sort by original order)
	if results[0].Result.ID != "doc1" {
		t.Errorf("expected doc1 first, got %s", results[0].Result.ID)
	}

	// Verify ranks are set correctly
	for _, r := range results {
		switch r.Result.ID {
		case "doc1":
			if r.SparseRank != 1 || r.DenseRank != 2 {
				t.Errorf("doc1: expected sparse=1, dense=2, got sparse=%d, dense=%d",
					r.SparseRank, r.DenseRank)
			}
		case "doc2":
			if r.SparseRank != 2 || r.DenseRank != 1 {
				t.Errorf("doc2: expected sparse=2, dense=1, got sparse=%d, dense=%d",
					r.SparseRank, r.DenseRank)
			}
		case "doc3":
			if r.SparseRank != 3 || r.DenseRank != 0 {
				t.Errorf("doc3: expected sparse=3, dense=0, got sparse=%d, dense=%d",
					r.SparseRank, r.DenseRank)
			}
		case "doc4":
			if r.SparseRank != 0 || r.DenseRank != 3 {
				t.Errorf("doc4: expected sparse=0, dense=3, got sparse=%d, dense=%d",
					r.SparseRank, r.DenseRank)
			}
		}
	}
}

func TestFuse_SparseHeavy(t *testing.T) {
	sparse := []qdrant.SearchResult{
		{ID: "doc1", Score: 10.0},
		{ID: "doc2", Score: 8.0},
	}

	dense := []qdrant.SearchResult{
		{ID: "doc3", Score: 0.95},
		{ID: "doc1", Score: 0.90},
	}

	cfg := RRFConfig{
		K:            60,
		SparseWeight: 0.8,
		DenseWeight:  0.2,
	}

	results := Fuse(sparse, dense, cfg)

	// doc1 should rank highest with sparse-heavy weighting
	// doc1: 0.8/(60+1) + 0.2/(60+2) = 0.8/61 + 0.2/62 = 0.01311 + 0.00323 = 0.01634
	// doc3: 0.0 + 0.2/(60+1) = 0.2/61 = 0.00328
	// doc2: 0.8/(60+2) + 0.0 = 0.8/62 = 0.01290
	if results[0].Result.ID != "doc1" {
		t.Errorf("expected doc1 first with sparse-heavy weights, got %s", results[0].Result.ID)
	}
}

func TestFuse_DenseHeavy(t *testing.T) {
	sparse := []qdrant.SearchResult{
		{ID: "doc1", Score: 10.0},
		{ID: "doc2", Score: 8.0},
	}

	dense := []qdrant.SearchResult{
		{ID: "doc3", Score: 0.95},
		{ID: "doc1", Score: 0.90},
	}

	cfg := RRFConfig{
		K:            60,
		SparseWeight: 0.2,
		DenseWeight:  0.8,
	}

	results := Fuse(sparse, dense, cfg)

	// doc3 should rank highest with dense-heavy weighting
	// doc3: 0.0 + 0.8/(60+1) = 0.8/61 = 0.01311
	// doc1: 0.2/(60+1) + 0.8/(60+2) = 0.2/61 + 0.8/62 = 0.00328 + 0.01290 = 0.01618
	if results[0].Result.ID != "doc1" {
		t.Errorf("expected doc1 first (appears in both), got %s", results[0].Result.ID)
	}
	if results[1].Result.ID != "doc3" {
		t.Errorf("expected doc3 second, got %s", results[1].Result.ID)
	}
}

func TestFuse_EmptyResults(t *testing.T) {
	cfg := DefaultRRFConfig()

	// Empty sparse
	results := Fuse([]qdrant.SearchResult{}, []qdrant.SearchResult{
		{ID: "doc1", Score: 0.9},
	}, cfg)
	if len(results) != 1 {
		t.Errorf("expected 1 result with empty sparse, got %d", len(results))
	}

	// Empty dense
	results = Fuse([]qdrant.SearchResult{
		{ID: "doc1", Score: 10.0},
	}, []qdrant.SearchResult{}, cfg)
	if len(results) != 1 {
		t.Errorf("expected 1 result with empty dense, got %d", len(results))
	}

	// Both empty
	results = Fuse([]qdrant.SearchResult{}, []qdrant.SearchResult{}, cfg)
	if len(results) != 0 {
		t.Errorf("expected 0 results with both empty, got %d", len(results))
	}
}

func TestFuse_OnlyOneRetriever(t *testing.T) {
	cfg := DefaultRRFConfig()

	// Only sparse results
	sparse := []qdrant.SearchResult{
		{ID: "doc1", Score: 10.0},
		{ID: "doc2", Score: 8.0},
	}

	results := Fuse(sparse, []qdrant.SearchResult{}, cfg)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}

	// Verify ranks
	if results[0].SparseRank != 1 || results[0].DenseRank != 0 {
		t.Errorf("expected sparse=1, dense=0, got sparse=%d, dense=%d",
			results[0].SparseRank, results[0].DenseRank)
	}
}

func TestIsBalanced(t *testing.T) {
	tests := []struct {
		name     string
		cfg      RRFConfig
		expected bool
	}{
		{
			name:     "default equal weights",
			cfg:      RRFConfig{SparseWeight: 0.5, DenseWeight: 0.5},
			expected: true,
		},
		{
			name:     "slightly off equal",
			cfg:      RRFConfig{SparseWeight: 0.48, DenseWeight: 0.52},
			expected: true,
		},
		{
			name:     "sparse heavy",
			cfg:      RRFConfig{SparseWeight: 0.7, DenseWeight: 0.3},
			expected: false,
		},
		{
			name:     "dense heavy",
			cfg:      RRFConfig{SparseWeight: 0.3, DenseWeight: 0.7},
			expected: false,
		},
		{
			name:     "extreme sparse",
			cfg:      RRFConfig{SparseWeight: 1.0, DenseWeight: 0.0},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.cfg.IsBalanced()
			if got != tt.expected {
				t.Errorf("IsBalanced() = %v, want %v (sparse=%.2f, dense=%.2f)",
					got, tt.expected, tt.cfg.SparseWeight, tt.cfg.DenseWeight)
			}
		})
	}
}

func TestFuse_PreservesOriginalScores(t *testing.T) {
	sparse := []qdrant.SearchResult{
		{ID: "doc1", Score: 10.5},
	}

	dense := []qdrant.SearchResult{
		{ID: "doc1", Score: 0.95},
	}

	cfg := DefaultRRFConfig()
	results := Fuse(sparse, dense, cfg)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	if results[0].SparseScore != 10.5 {
		t.Errorf("expected sparse score 10.5, got %.2f", results[0].SparseScore)
	}

	if results[0].DenseScore != 0.95 {
		t.Errorf("expected dense score 0.95, got %.2f", results[0].DenseScore)
	}
}
