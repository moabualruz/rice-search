package evaluation

import (
	"context"
	"fmt"

	"github.com/ricesearch/rice-search/internal/search"
)

// Evaluator orchestrates search evaluation.
type Evaluator struct {
	searchSvc *search.Service
	judgments map[string]map[string]int // queryID -> docID -> relevance
}

// NewEvaluator creates a new evaluator.
func NewEvaluator(searchSvc *search.Service) *Evaluator {
	return &Evaluator{
		searchSvc: searchSvc,
		judgments: make(map[string]map[string]int),
	}
}

// LoadJudgments loads relevance judgments.
func (e *Evaluator) LoadJudgments(judgments []RelevanceJudgment) {
	for _, j := range judgments {
		if e.judgments[j.QueryID] == nil {
			e.judgments[j.QueryID] = make(map[string]int)
		}
		e.judgments[j.QueryID][j.DocID] = j.Relevance
	}
}

// EvaluateQuery evaluates a single query.
func (e *Evaluator) EvaluateQuery(ctx context.Context, queryID, queryText, store string, ks []int) (*EvaluationResult, error) {
	// Execute search
	// We disable reranking and other fancy features to test baseline,
	// OR we allow the request to specify them. Use defaults for now?
	// Actually, usually we evaluate the Full Pipeline.
	// So we should use default config or allow config override.
	// For now, simple search.
	req := search.Request{
		Store: store,
		Query: queryText,
		TopK:  100, // Get enough for evaluation
	}

	resp, err := e.searchSvc.Search(ctx, req)
	if err != nil {
		return nil, err
	}

	// Get relevances for results
	qJudgments := e.judgments[queryID]
	relevances := make([]int, len(resp.Results))
	for i, r := range resp.Results {
		if qJudgments != nil {
			// Construct DocID to match judgment format
			// Format: path:start-end
			docID := fmt.Sprintf("%s:%d-%d", r.Path, r.StartLine, r.EndLine)
			relevances[i] = qJudgments[docID]
		}
	}

	// Calculate metrics
	result := &EvaluationResult{
		QueryID:     queryID,
		Query:       queryText,
		NDCG:        make(map[int]float64),
		Recall:      make(map[int]float64),
		Precision:   make(map[int]float64),
		MRR:         MRR(relevances, 1),
		AP:          AveragePrecision(relevances, 1),
		ResultCount: len(resp.Results),
	}

	for _, k := range ks {
		result.NDCG[k] = NDCG(relevances, k)
		result.Recall[k] = Recall(relevances, k, 1)
		result.Precision[k] = Precision(relevances, k, 1)
	}

	return result, nil
}

// Summarize aggregates results across queries.
func (e *Evaluator) Summarize(results []*EvaluationResult) *EvaluationSummary {
	if len(results) == 0 {
		return &EvaluationSummary{}
	}

	summary := &EvaluationSummary{
		QueryCount:    len(results),
		MeanNDCG:      make(map[int]float64),
		MeanRecall:    make(map[int]float64),
		MeanPrecision: make(map[int]float64),
	}

	// Aggregate
	for _, r := range results {
		summary.MeanMRR += r.MRR
		summary.MAP += r.AP

		for k, v := range r.NDCG {
			summary.MeanNDCG[k] += v
		}
		for k, v := range r.Recall {
			summary.MeanRecall[k] += v
		}
		for k, v := range r.Precision {
			summary.MeanPrecision[k] += v
		}
	}

	// Average
	n := float64(len(results))
	summary.MeanMRR /= n
	summary.MAP /= n

	for k := range summary.MeanNDCG {
		summary.MeanNDCG[k] /= n
	}
	for k := range summary.MeanRecall {
		summary.MeanRecall[k] /= n
	}
	for k := range summary.MeanPrecision {
		summary.MeanPrecision[k] /= n
	}

	return summary
}
