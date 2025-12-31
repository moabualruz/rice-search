package evaluation

import (
	"math"
	"sort"
)

// NDCG calculates Normalized Discounted Cumulative Gain at K
func NDCG(relevances []int, k int) float64 {
	if k > len(relevances) {
		k = len(relevances)
	}
	if k == 0 {
		return 0
	}

	// DCG
	dcg := float64(relevances[0])
	for i := 1; i < k; i++ {
		dcg += float64(relevances[i]) / math.Log2(float64(i+2))
	}

	// Ideal DCG (sorted by relevance)
	sorted := make([]int, len(relevances))
	copy(sorted, relevances)
	sort.Sort(sort.Reverse(sort.IntSlice(sorted)))

	idcg := float64(sorted[0])
	for i := 1; i < k; i++ {
		idcg += float64(sorted[i]) / math.Log2(float64(i+2))
	}

	if idcg == 0 {
		return 0
	}
	return dcg / idcg
}

// Recall calculates Recall at K
func Recall(relevances []int, k int, threshold int) float64 {
	if k > len(relevances) {
		k = len(relevances)
	}

	// Count total relevant
	totalRelevant := 0
	for _, r := range relevances {
		if r >= threshold {
			totalRelevant++
		}
	}

	if totalRelevant == 0 {
		return 0
	}

	// Count relevant in top K
	relevantInK := 0
	for i := 0; i < k; i++ {
		if relevances[i] >= threshold {
			relevantInK++
		}
	}

	return float64(relevantInK) / float64(totalRelevant)
}

// Precision calculates Precision at K
func Precision(relevances []int, k int, threshold int) float64 {
	if k > len(relevances) {
		k = len(relevances)
	}
	if k == 0 {
		return 0
	}

	relevant := 0
	for i := 0; i < k; i++ {
		if relevances[i] >= threshold {
			relevant++
		}
	}

	return float64(relevant) / float64(k)
}

// MRR calculates Mean Reciprocal Rank
func MRR(relevances []int, threshold int) float64 {
	for i, r := range relevances {
		if r >= threshold {
			return 1.0 / float64(i+1)
		}
	}
	return 0
}

// AveragePrecision calculates Average Precision
func AveragePrecision(relevances []int, threshold int) float64 {
	relevant := 0
	sumPrecision := 0.0

	for i, r := range relevances {
		if r >= threshold {
			relevant++
			sumPrecision += float64(relevant) / float64(i+1)
		}
	}

	if relevant == 0 {
		return 0
	}
	return sumPrecision / float64(relevant)
}
