package evaluation

// RelevanceJudgment represents human-labeled relevance for a query-doc pair
type RelevanceJudgment struct {
	QueryID   string `json:"query_id"`
	DocID     string `json:"doc_id"`
	Relevance int    `json:"relevance"` // 0=not relevant, 1=partially, 2=relevant, 3=highly
}

// EvaluationResult contains metrics for a single query
type EvaluationResult struct {
	QueryID     string          `json:"query_id"`
	Query       string          `json:"query"`
	NDCG        map[int]float64 `json:"ndcg"`      // NDCG@K for various K
	Recall      map[int]float64 `json:"recall"`    // Recall@K
	Precision   map[int]float64 `json:"precision"` // Precision@K
	MRR         float64         `json:"mrr"`
	AP          float64         `json:"ap"` // Average Precision
	ResultCount int             `json:"result_count"`
}

// EvaluationSummary aggregates metrics across multiple queries
type EvaluationSummary struct {
	QueryCount    int             `json:"query_count"`
	MeanNDCG      map[int]float64 `json:"mean_ndcg"`
	MeanRecall    map[int]float64 `json:"mean_recall"`
	MeanPrecision map[int]float64 `json:"mean_precision"`
	MeanMRR       float64         `json:"mean_mrr"`
	MAP           float64         `json:"map"`
}
