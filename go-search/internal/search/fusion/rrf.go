// Package fusion provides configurable result fusion algorithms.
package fusion

import (
	"sort"

	"github.com/ricesearch/rice-search/internal/qdrant"
)

const (
	// DefaultK is the RRF smoothing constant.
	// Higher values reduce the impact of rank position differences.
	DefaultK = 60
)

// RRFConfig configures Reciprocal Rank Fusion parameters.
type RRFConfig struct {
	// K is the smoothing constant (default: 60).
	// Higher values give more weight to lower-ranked results.
	K int

	// SparseWeight is the weight for sparse (BM25-like) results (0.0-1.0).
	// Default: 0.5 for equal weighting.
	SparseWeight float32

	// DenseWeight is the weight for dense (semantic) results (0.0-1.0).
	// Default: 0.5 for equal weighting.
	DenseWeight float32
}

// DefaultRRFConfig returns the default RRF configuration with equal weights.
func DefaultRRFConfig() RRFConfig {
	return RRFConfig{
		K:            DefaultK,
		SparseWeight: 0.5,
		DenseWeight:  0.5,
	}
}

// ScoredResult represents a result with combined RRF score and component scores.
type ScoredResult struct {
	// Result is the original Qdrant result.
	Result qdrant.SearchResult

	// SparseRank is the rank in sparse-only results (1-based, 0 if not present).
	SparseRank int

	// DenseRank is the rank in dense-only results (1-based, 0 if not present).
	DenseRank int

	// SparseScore is the original sparse score from Qdrant.
	SparseScore float32

	// DenseScore is the original dense score from Qdrant.
	DenseScore float32

	// FusedScore is the combined RRF score.
	FusedScore float32
}

// Fuse combines sparse and dense results using weighted RRF.
//
// Formula: score = sparseWeight/(k + sparseRank) + denseWeight/(k + denseRank)
//
// Results are sorted by FusedScore in descending order.
func Fuse(sparseResults, denseResults []qdrant.SearchResult, cfg RRFConfig) []ScoredResult {
	// Use defaults if not set
	if cfg.K == 0 {
		cfg.K = DefaultK
	}
	if cfg.SparseWeight == 0 && cfg.DenseWeight == 0 {
		cfg = DefaultRRFConfig()
	}

	// Build a map of document ID to scored result
	scores := make(map[string]*ScoredResult)

	// Process sparse results
	for rank, r := range sparseResults {
		id := r.ID
		if scores[id] == nil {
			scores[id] = &ScoredResult{
				Result: r,
			}
		}
		scores[id].SparseRank = rank + 1 // 1-based rank
		scores[id].SparseScore = r.Score
		// Add sparse contribution to fused score
		scores[id].FusedScore += cfg.SparseWeight / float32(cfg.K+rank+1)
	}

	// Process dense results
	for rank, r := range denseResults {
		id := r.ID
		if scores[id] == nil {
			scores[id] = &ScoredResult{
				Result: r,
			}
		}
		scores[id].DenseRank = rank + 1 // 1-based rank
		scores[id].DenseScore = r.Score
		// Add dense contribution to fused score
		scores[id].FusedScore += cfg.DenseWeight / float32(cfg.K+rank+1)
	}

	// Convert map to slice
	results := make([]ScoredResult, 0, len(scores))
	for _, sr := range scores {
		// If result only appeared in one retriever, copy its payload
		if sr.SparseRank > 0 && sr.DenseRank == 0 {
			sr.DenseScore = 0
		} else if sr.DenseRank > 0 && sr.SparseRank == 0 {
			sr.SparseScore = 0
		}
		results = append(results, *sr)
	}

	// Sort by fused score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].FusedScore > results[j].FusedScore
	})

	return results
}

// IsBalanced returns true if weights are approximately equal (both ~0.5).
func (cfg RRFConfig) IsBalanced() bool {
	const epsilon = 0.05
	diff := abs(cfg.SparseWeight - cfg.DenseWeight)
	return diff < epsilon && abs(cfg.SparseWeight-0.5) < epsilon
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
