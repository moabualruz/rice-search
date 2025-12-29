// Package postrank provides post-ranking operations for search results.
package postrank

import (
	"context"
	"math"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// DedupService performs semantic deduplication on search results.
type DedupService struct {
	threshold float32
	log       *logger.Logger
}

// NewDedupService creates a new deduplication service.
func NewDedupService(threshold float32, log *logger.Logger) *DedupService {
	if threshold <= 0 || threshold > 1 {
		threshold = 0.85 // Default threshold
	}
	return &DedupService{
		threshold: threshold,
		log:       log,
	}
}

// DedupResult contains deduplication statistics.
type DedupResult struct {
	InputCount  int
	OutputCount int
	Removed     int
	LatencyMs   int64
}

// Deduplicate removes near-duplicate results based on embedding similarity.
// Returns filtered results and statistics.
func (s *DedupService) Deduplicate(ctx context.Context, results []ResultWithEmbedding) ([]ResultWithEmbedding, DedupResult) {
	start := time.Now()

	if len(results) == 0 {
		return results, DedupResult{
			InputCount:  0,
			OutputCount: 0,
			Removed:     0,
			LatencyMs:   0,
		}
	}

	// Track which results to keep
	keep := make([]bool, len(results))
	for i := range keep {
		keep[i] = true
	}

	// Compare each result with previous ones
	for i := 1; i < len(results); i++ {
		if !keep[i] {
			continue
		}

		// Check context cancellation periodically
		if i%100 == 0 {
			select {
			case <-ctx.Done():
				// Return what we have so far
				return s.filterResults(results, keep), DedupResult{
					InputCount:  len(results),
					OutputCount: countTrue(keep),
					Removed:     len(results) - countTrue(keep),
					LatencyMs:   time.Since(start).Milliseconds(),
				}
			default:
			}
		}

		// Compare with all previous kept results
		for j := 0; j < i; j++ {
			if !keep[j] {
				continue
			}

			// Calculate cosine similarity
			similarity := cosineSimilarity(results[i].Embedding, results[j].Embedding)

			// If too similar, mark current result for removal
			if similarity >= s.threshold {
				keep[i] = false
				s.log.Debug("Removing duplicate result",
					"index", i,
					"similar_to", j,
					"similarity", similarity,
					"threshold", s.threshold,
				)
				break
			}
		}
	}

	filtered := s.filterResults(results, keep)

	return filtered, DedupResult{
		InputCount:  len(results),
		OutputCount: len(filtered),
		Removed:     len(results) - len(filtered),
		LatencyMs:   time.Since(start).Milliseconds(),
	}
}

// filterResults creates a new slice with only the kept results.
func (s *DedupService) filterResults(results []ResultWithEmbedding, keep []bool) []ResultWithEmbedding {
	filtered := make([]ResultWithEmbedding, 0, countTrue(keep))
	for i, r := range results {
		if keep[i] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// cosineSimilarity calculates the cosine similarity between two vectors.
// Returns a value between 0 and 1, where 1 means identical direction.
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	if len(a) == 0 {
		return 0
	}

	var dotProduct float64
	var normA float64
	var normB float64

	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	// Handle zero vectors
	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

// countTrue counts the number of true values in a boolean slice.
func countTrue(slice []bool) int {
	count := 0
	for _, v := range slice {
		if v {
			count++
		}
	}
	return count
}
