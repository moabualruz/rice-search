# Implementation Plan: IR Evaluation Framework

**Priority:** 🟡 P2 (Medium)  
**Effort:** Medium (1-2 days)  
**Dependencies:** None

---

## Overview

Add Information Retrieval (IR) evaluation metrics to measure search quality. Supports NDCG, Recall, MRR, Precision, and MAP.

## Goals

1. **Standard IR metrics** - NDCG@K, Recall@K, MRR, Precision@K, MAP
2. **Relevance judgments** - Load human-labeled relevance scores
3. **Evaluation API** - Evaluate single query or batch
4. **Comparison** - Compare metrics between configurations

## Metrics

| Metric | Description | Range |
|--------|-------------|-------|
| **NDCG@K** | Normalized Discounted Cumulative Gain | 0-1 (higher better) |
| **Recall@K** | Fraction of relevant docs in top K | 0-1 (higher better) |
| **MRR** | Mean Reciprocal Rank (first relevant) | 0-1 (higher better) |
| **Precision@K** | Fraction of top K that are relevant | 0-1 (higher better) |
| **MAP** | Mean Average Precision | 0-1 (higher better) |

## Package Structure

```
internal/
├── evaluation/
│   ├── metrics.go       # Metric calculations
│   ├── judgments.go     # Relevance judgment loading
│   ├── evaluator.go     # Evaluation orchestration
│   └── service.go       # API service
```

## Implementation

### Step 1: Define Types

**File:** `internal/evaluation/types.go`
```go
package evaluation

// RelevanceJudgment represents human-labeled relevance for a query-doc pair
type RelevanceJudgment struct {
    QueryID    string  `json:"query_id"`
    DocID      string  `json:"doc_id"`
    Relevance  int     `json:"relevance"` // 0=not relevant, 1=partially, 2=relevant, 3=highly
}

// EvaluationResult contains metrics for a single query
type EvaluationResult struct {
    QueryID     string             `json:"query_id"`
    Query       string             `json:"query"`
    NDCG        map[int]float64    `json:"ndcg"`        // NDCG@K for various K
    Recall      map[int]float64    `json:"recall"`      // Recall@K
    Precision   map[int]float64    `json:"precision"`   // Precision@K
    MRR         float64            `json:"mrr"`
    AP          float64            `json:"ap"`          // Average Precision
    ResultCount int                `json:"result_count"`
}

// EvaluationSummary aggregates metrics across multiple queries
type EvaluationSummary struct {
    QueryCount   int                `json:"query_count"`
    MeanNDCG     map[int]float64    `json:"mean_ndcg"`
    MeanRecall   map[int]float64    `json:"mean_recall"`
    MeanPrecision map[int]float64   `json:"mean_precision"`
    MeanMRR      float64            `json:"mean_mrr"`
    MAP          float64            `json:"map"`
}
```

### Step 2: Implement Metrics

**File:** `internal/evaluation/metrics.go`
```go
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
```

### Step 3: Implement Evaluator

**File:** `internal/evaluation/evaluator.go`
```go
package evaluation

import (
    "context"
    
    "github.com/ricesearch/go-search/internal/search"
)

type Evaluator struct {
    searchSvc *search.Service
    judgments map[string]map[string]int // queryID -> docID -> relevance
}

func NewEvaluator(searchSvc *search.Service) *Evaluator {
    return &Evaluator{
        searchSvc: searchSvc,
        judgments: make(map[string]map[string]int),
    }
}

// LoadJudgments loads relevance judgments from JSONL file
func (e *Evaluator) LoadJudgments(judgments []RelevanceJudgment) {
    for _, j := range judgments {
        if e.judgments[j.QueryID] == nil {
            e.judgments[j.QueryID] = make(map[string]int)
        }
        e.judgments[j.QueryID][j.DocID] = j.Relevance
    }
}

// EvaluateQuery evaluates a single query
func (e *Evaluator) EvaluateQuery(ctx context.Context, queryID, query, store string, ks []int) (*EvaluationResult, error) {
    // Execute search
    results, err := e.searchSvc.Search(ctx, search.Request{
        Store: store,
        Query: query,
        TopK:  100, // Get enough for evaluation
    })
    if err != nil {
        return nil, err
    }
    
    // Get relevances for results
    qJudgments := e.judgments[queryID]
    relevances := make([]int, len(results.Results))
    for i, r := range results.Results {
        if qJudgments != nil {
            relevances[i] = qJudgments[r.DocID]
        }
    }
    
    // Calculate metrics
    result := &EvaluationResult{
        QueryID:     queryID,
        Query:       query,
        NDCG:        make(map[int]float64),
        Recall:      make(map[int]float64),
        Precision:   make(map[int]float64),
        MRR:         MRR(relevances, 1),
        AP:          AveragePrecision(relevances, 1),
        ResultCount: len(results.Results),
    }
    
    for _, k := range ks {
        result.NDCG[k] = NDCG(relevances, k)
        result.Recall[k] = Recall(relevances, k, 1)
        result.Precision[k] = Precision(relevances, k, 1)
    }
    
    return result, nil
}

// Summarize aggregates results across queries
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
```

### Step 4: Add API Endpoints

**File:** `internal/search/handlers_evaluation.go`
```go
// POST /v1/evaluation/evaluate
// Body: {"queries": [{"id": "q1", "query": "auth handler"}], "store": "default", "ks": [5, 10, 20]}

// POST /v1/evaluation/judgments
// Body: [{"query_id": "q1", "doc_id": "d1", "relevance": 2}, ...]

// GET /v1/evaluation/summary?store=default
```

## Judgment File Format

```json
{"query_id": "q1", "doc_id": "src/auth/handler.go:10-50", "relevance": 3}
{"query_id": "q1", "doc_id": "src/auth/middleware.go:1-30", "relevance": 2}
{"query_id": "q2", "doc_id": "src/errors/handler.go:5-25", "relevance": 3}
```

Relevance levels:
- 0 = Not relevant
- 1 = Marginally relevant
- 2 = Relevant
- 3 = Highly relevant

## Success Metrics

- [ ] NDCG@K calculation correct
- [ ] Recall@K calculation correct
- [ ] MRR calculation correct
- [ ] MAP calculation correct
- [ ] Judgment loading works
- [ ] Summary aggregation works
- [ ] API endpoints functional

## References

- Old implementation: `api/src/observability/evaluation.service.ts`
- NDCG: https://en.wikipedia.org/wiki/Discounted_cumulative_gain
