package postrank

import (
	"context"
	"time"

	"github.com/ricesearch/rice-search/internal/pkg/logger"
)

// DiversityService applies Maximal Marginal Relevance (MMR) for diversity.
type DiversityService struct {
	lambda float32 // 0 = max diversity, 1 = max relevance
	log    *logger.Logger
}

// NewDiversityService creates a new diversity service.
func NewDiversityService(lambda float32, log *logger.Logger) *DiversityService {
	if lambda < 0 || lambda > 1 {
		lambda = 0.7 // Default: slightly favor relevance
	}
	return &DiversityService{
		lambda: lambda,
		log:    log,
	}
}

// DiversityResult contains diversity statistics.
type DiversityResult struct {
	Enabled      bool
	AvgDiversity float32
	LatencyMs    int64
}

// ApplyMMR applies Maximal Marginal Relevance to reorder results for diversity.
// MMR(Di) = λ * Relevance(Di) - (1-λ) * max_j∈S Similarity(Di, Dj)
// where S is the set of already selected documents.
func (s *DiversityService) ApplyMMR(ctx context.Context, results []ResultWithEmbedding, k int) ([]ResultWithEmbedding, DiversityResult) {
	start := time.Now()

	if len(results) == 0 {
		return results, DiversityResult{
			Enabled:      false,
			AvgDiversity: 0,
			LatencyMs:    0,
		}
	}

	// If k is larger than results, use all results
	if k > len(results) {
		k = len(results)
	}

	// Normalize relevance scores to 0-1 range
	maxScore := results[0].Score
	minScore := results[len(results)-1].Score
	scoreRange := maxScore - minScore
	if scoreRange == 0 {
		scoreRange = 1 // Avoid division by zero
	}

	normalizedRelevance := make([]float32, len(results))
	for i := range results {
		normalizedRelevance[i] = (results[i].Score - minScore) / scoreRange
	}

	// Selected indices in MMR order
	selected := make([]int, 0, k)
	available := make(map[int]bool)
	for i := range results {
		available[i] = true
	}

	// First result is always the most relevant
	selected = append(selected, 0)
	delete(available, 0)

	var totalDiversity float32

	// Select remaining results using MMR
	for len(selected) < k && len(available) > 0 {
		// Check context cancellation
		select {
		case <-ctx.Done():
			// Return what we have so far
			return s.reorderResults(results, selected), DiversityResult{
				Enabled:      true,
				AvgDiversity: totalDiversity / float32(len(selected)-1),
				LatencyMs:    time.Since(start).Milliseconds(),
			}
		default:
		}

		var bestIdx int
		var bestScore float32 = -1

		// Find the result that maximizes MMR
		for idx := range available {
			relevance := normalizedRelevance[idx]

			// Find maximum similarity to already selected results
			var maxSimilarity float32
			for _, selIdx := range selected {
				sim := cosineSimilarity(results[idx].Embedding, results[selIdx].Embedding)
				if sim > maxSimilarity {
					maxSimilarity = sim
				}
			}

			// Calculate MMR score
			mmrScore := s.lambda*relevance - (1-s.lambda)*maxSimilarity

			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = idx
			}
		}

		// Add the best result
		selected = append(selected, bestIdx)
		delete(available, bestIdx)

		// Track diversity (1 - similarity to most similar selected item)
		var maxSimToSelected float32
		for i := 0; i < len(selected)-1; i++ {
			sim := cosineSimilarity(results[bestIdx].Embedding, results[selected[i]].Embedding)
			if sim > maxSimToSelected {
				maxSimToSelected = sim
			}
		}
		diversity := 1 - maxSimToSelected
		totalDiversity += diversity
	}

	avgDiversity := float32(0)
	if len(selected) > 1 {
		avgDiversity = totalDiversity / float32(len(selected)-1)
	}

	reordered := s.reorderResults(results, selected)

	return reordered, DiversityResult{
		Enabled:      true,
		AvgDiversity: avgDiversity,
		LatencyMs:    time.Since(start).Milliseconds(),
	}
}

// reorderResults creates a new slice with results in the selected order.
func (s *DiversityService) reorderResults(results []ResultWithEmbedding, selectedIndices []int) []ResultWithEmbedding {
	reordered := make([]ResultWithEmbedding, len(selectedIndices))
	for i, idx := range selectedIndices {
		reordered[i] = results[idx]
	}
	return reordered
}

// EstimateDiversity calculates the average diversity of a result set.
// Diversity is measured as 1 - average_pairwise_similarity.
func EstimateDiversity(results []ResultWithEmbedding) float32 {
	if len(results) < 2 {
		return 1.0
	}

	var totalSimilarity float64
	var pairs int

	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			sim := cosineSimilarity(results[i].Embedding, results[j].Embedding)
			totalSimilarity += float64(sim)
			pairs++
		}
	}

	avgSimilarity := float32(totalSimilarity / float64(pairs))
	return 1 - avgSimilarity
}
