package reranker

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/ricesearch/rice-search/internal/ml"
	"github.com/ricesearch/rice-search/internal/pkg/logger"
	"github.com/ricesearch/rice-search/internal/search"
)

// MultiPassReranker performs two-pass reranking with early exit optimization.
type MultiPassReranker struct {
	reranker        ml.Service
	pass1Candidates int     // Default: 30
	pass2Candidates int     // Default: 100
	pass1Timeout    int     // Default: 80ms
	pass2Timeout    int     // Default: 150ms
	earlyExitThresh float32 // Default: 0.85
	earlyExitGap    float32 // Default: 0.3
	log             *logger.Logger
}

// NewMultiPassReranker creates a new multi-pass reranker.
func NewMultiPassReranker(reranker ml.Service, log *logger.Logger) *MultiPassReranker {
	return &MultiPassReranker{
		reranker:        reranker,
		pass1Candidates: 30,
		pass2Candidates: 100,
		pass1Timeout:    80,
		pass2Timeout:    150,
		earlyExitThresh: 0.85,
		earlyExitGap:    0.3,
		log:             log,
	}
}

// Config holds configuration for multi-pass reranking.
type Config struct {
	Pass1Candidates int
	Pass2Candidates int
	Pass1Timeout    int
	Pass2Timeout    int
	EarlyExitThresh float32
	EarlyExitGap    float32
}

// SetConfig updates the reranker configuration.
func (r *MultiPassReranker) SetConfig(cfg Config) {
	if cfg.Pass1Candidates > 0 {
		r.pass1Candidates = cfg.Pass1Candidates
	}
	if cfg.Pass2Candidates > 0 {
		r.pass2Candidates = cfg.Pass2Candidates
	}
	if cfg.Pass1Timeout > 0 {
		r.pass1Timeout = cfg.Pass1Timeout
	}
	if cfg.Pass2Timeout > 0 {
		r.pass2Timeout = cfg.Pass2Timeout
	}
	if cfg.EarlyExitThresh > 0 {
		r.earlyExitThresh = cfg.EarlyExitThresh
	}
	if cfg.EarlyExitGap > 0 {
		r.earlyExitGap = cfg.EarlyExitGap
	}
}

// MultiPassResult contains reranked results and metadata.
type MultiPassResult struct {
	Results         []search.Result
	Pass1Applied    bool
	Pass1LatencyMs  int64
	Pass2Applied    bool
	Pass2LatencyMs  int64
	EarlyExit       bool
	EarlyExitReason string
}

// DistributionShape describes the score distribution pattern.
type DistributionShape string

const (
	ShapePeaked  DistributionShape = "peaked"  // One clear winner
	ShapeFlat    DistributionShape = "flat"    // All scores similar (uncertain)
	ShapeBimodal DistributionShape = "bimodal" // Mixed distribution
)

// EarlyExitSignals contains signals for early exit decision.
type EarlyExitSignals struct {
	ScoreGap           float32
	ScoreRatio         float32
	TopClusterSize     int
	DistributionShape  DistributionShape
	NormalizedVariance float32
}

// Rerank performs multi-pass reranking with early exit.
//
// Pass 1 (Gate): Fast rerank to filter candidates (100 → 30)
// Pass 2 (Precision): Deeper rerank for final ordering (30 → K) [conditional]
//
// Early exit occurs when:
// - Fewer than 2 results
// - Peaked distribution with high score ratio (> 1.5)
// - Large score gap between top and second (> 0.3)
func (r *MultiPassReranker) Rerank(ctx context.Context, query string, results []search.Result) (*MultiPassResult, error) {
	result := &MultiPassResult{
		Results: results,
	}

	if len(results) == 0 {
		return result, nil
	}

	// Pass 1: Fast rerank top candidates
	pass1Start := time.Now()
	pass1Input := min(len(results), r.pass1Candidates)

	r.log.Debug("Starting pass 1 reranking",
		"input_count", pass1Input,
		"timeout_ms", r.pass1Timeout)

	pass1Results, err := r.executePass(ctx, query, results[:pass1Input], r.pass1Candidates, r.pass1Timeout)
	pass1Latency := time.Since(pass1Start).Milliseconds()
	result.Pass1LatencyMs = pass1Latency

	if err != nil {
		r.log.Warn("Pass 1 reranking failed, using original order", "error", err)
		return result, nil
	}

	result.Pass1Applied = true
	result.Results = pass1Results

	r.log.Debug("Pass 1 complete",
		"output_count", len(pass1Results),
		"latency_ms", pass1Latency)

	// Check early exit conditions
	if r.shouldExitEarly(pass1Results, result) {
		r.log.Debug("Early exit triggered",
			"reason", result.EarlyExitReason,
			"total_latency_ms", pass1Latency)
		return result, nil
	}

	// Pass 2: Deep rerank if needed
	pass2Start := time.Now()
	pass2Input := min(len(results), r.pass2Candidates)

	r.log.Debug("Starting pass 2 reranking",
		"input_count", pass2Input,
		"timeout_ms", r.pass2Timeout)

	pass2Results, err := r.executePass(ctx, query, results[:pass2Input], r.pass2Candidates, r.pass2Timeout)
	pass2Latency := time.Since(pass2Start).Milliseconds()
	result.Pass2LatencyMs = pass2Latency

	if err != nil {
		r.log.Warn("Pass 2 reranking failed, using pass 1 results", "error", err)
		return result, nil
	}

	result.Pass2Applied = true
	result.Results = pass2Results

	r.log.Debug("Pass 2 complete",
		"output_count", len(pass2Results),
		"latency_ms", pass2Latency,
		"total_latency_ms", pass1Latency+pass2Latency)

	return result, nil
}

// executePass performs a single reranking pass with timeout.
func (r *MultiPassReranker) executePass(
	ctx context.Context,
	query string,
	results []search.Result,
	topK int,
	timeoutMs int,
) ([]search.Result, error) {
	if len(results) == 0 {
		return results, nil
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Extract content for reranking
	documents := make([]string, len(results))
	for i, r := range results {
		documents[i] = r.Content
	}

	// Call reranker
	ranked, err := r.reranker.Rerank(ctx, query, documents, topK)
	if err != nil {
		return nil, fmt.Errorf("reranking failed: %w", err)
	}

	// Build score map
	scoreMap := make(map[int]float32)
	for _, rr := range ranked {
		scoreMap[rr.Index] = rr.Score
	}

	// Apply rerank scores
	reranked := make([]search.Result, len(results))
	for i, r := range results {
		reranked[i] = r
		if score, ok := scoreMap[i]; ok {
			reranked[i].Score = score
		}
	}

	// Sort by new scores
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	// Return top K
	if len(reranked) > topK {
		reranked = reranked[:topK]
	}

	return reranked, nil
}

// shouldExitEarly determines if we should skip pass 2 based on score distribution.
func (r *MultiPassReranker) shouldExitEarly(results []search.Result, mpResult *MultiPassResult) bool {
	if len(results) < 2 {
		mpResult.EarlyExit = true
		mpResult.EarlyExitReason = "insufficient_results"
		return true
	}

	signals := r.analyzeDistribution(results)

	// Exit if distribution is peaked (one clear winner)
	if signals.DistributionShape == ShapePeaked && signals.ScoreRatio > r.earlyExitThresh {
		mpResult.EarlyExit = true
		mpResult.EarlyExitReason = "peaked_distribution"
		r.log.Debug("Early exit: peaked distribution",
			"score_ratio", signals.ScoreRatio,
			"threshold", r.earlyExitThresh)
		return true
	}

	// Exit if very high gap between top and second
	if signals.ScoreGap > r.earlyExitGap {
		mpResult.EarlyExit = true
		mpResult.EarlyExitReason = "high_score_gap"
		r.log.Debug("Early exit: high score gap",
			"gap", signals.ScoreGap,
			"threshold", r.earlyExitGap)
		return true
	}

	// Don't exit if results are flat (uncertainty, needs second pass)
	if signals.DistributionShape == ShapeFlat {
		r.log.Debug("No early exit: flat distribution (uncertainty)")
		return false
	}

	return false
}

// analyzeDistribution analyzes the score distribution to determine early exit signals.
func (r *MultiPassReranker) analyzeDistribution(results []search.Result) EarlyExitSignals {
	if len(results) == 0 {
		return EarlyExitSignals{
			DistributionShape: ShapeFlat,
		}
	}

	scores := make([]float32, len(results))
	for i, r := range results {
		scores[i] = r.Score
	}

	top := scores[0]
	second := float32(0)
	if len(scores) > 1 {
		second = scores[1]
	}

	// Count results within 10% of top score
	threshold := top * 0.9
	topClusterSize := 0
	for _, s := range scores {
		if s >= threshold {
			topClusterSize++
		}
	}

	// Calculate distribution statistics
	var sum float32
	for _, s := range scores {
		sum += s
	}
	mean := sum / float32(len(scores))

	var varianceSum float32
	for _, s := range scores {
		diff := s - mean
		varianceSum += diff * diff
	}
	variance := varianceSum / float32(len(scores))

	normalizedVariance := float32(0)
	if mean > 0 {
		normalizedVariance = variance / (mean * mean)
	}

	// Determine distribution shape
	var shape DistributionShape
	if topClusterSize == 1 && normalizedVariance > 0.1 {
		shape = ShapePeaked // One clear winner
	} else if normalizedVariance < 0.05 {
		shape = ShapeFlat // All scores similar (uncertain)
	} else {
		shape = ShapeBimodal // Mixed distribution
	}

	scoreRatio := float32(0)
	if second > 0 {
		scoreRatio = top / second
	} else {
		scoreRatio = 999.0 // Effectively infinite
	}

	return EarlyExitSignals{
		ScoreGap:           top - second,
		ScoreRatio:         scoreRatio,
		TopClusterSize:     topClusterSize,
		DistributionShape:  shape,
		NormalizedVariance: normalizedVariance,
	}
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
