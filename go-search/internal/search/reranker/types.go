package reranker

// RerankingMetadata contains metadata about multi-pass reranking.
// This type can be imported into the search package.
type RerankingMetadata struct {
	Pass1Applied    bool   `json:"pass1_applied"`
	Pass1LatencyMs  int64  `json:"pass1_latency_ms"`
	Pass2Applied    bool   `json:"pass2_applied"`
	Pass2LatencyMs  int64  `json:"pass2_latency_ms"`
	EarlyExit       bool   `json:"early_exit"`
	EarlyExitReason string `json:"early_exit_reason,omitempty"`
}
