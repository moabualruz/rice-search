package reranker

import (
	"context"

	"github.com/ricesearch/rice-search/internal/search"
)

// Adapter wraps MultiPassReranker to implement search.MultiPassReranker interface.
// This breaks the circular dependency between search and reranker packages.
type Adapter struct {
	reranker *MultiPassReranker
}

// NewAdapter creates a new adapter for the multi-pass reranker.
func NewAdapter(reranker *MultiPassReranker) *Adapter {
	return &Adapter{reranker: reranker}
}

// Rerank implements search.MultiPassReranker interface.
func (a *Adapter) Rerank(ctx context.Context, query string, results []search.Result) (*search.MultiPassRerankResult, error) {
	// Call the underlying multi-pass reranker
	mpResult, err := a.reranker.Rerank(ctx, query, results)
	if err != nil {
		return nil, err
	}

	// Convert to search package types
	return &search.MultiPassRerankResult{
		Results:         mpResult.Results,
		Pass1Applied:    mpResult.Pass1Applied,
		Pass1LatencyMs:  mpResult.Pass1LatencyMs,
		Pass2Applied:    mpResult.Pass2Applied,
		Pass2LatencyMs:  mpResult.Pass2LatencyMs,
		EarlyExit:       mpResult.EarlyExit,
		EarlyExitReason: mpResult.EarlyExitReason,
	}, nil
}
