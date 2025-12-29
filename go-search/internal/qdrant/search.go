package qdrant

import (
	"context"
	"fmt"
	"time"

	"github.com/qdrant/go-client/qdrant"
)

// HybridSearch performs a hybrid search using both sparse and dense vectors with RRF fusion.
func (c *Client) HybridSearch(ctx context.Context, collection string, req SearchRequest) ([]SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	// Build prefetch queries for sparse and dense
	prefetch := make([]*qdrant.PrefetchQuery, 0, 2)

	prefetchLimit := req.PrefetchLimit
	if prefetchLimit == 0 {
		prefetchLimit = 100 // Default prefetch
	}

	// Sparse prefetch
	if len(req.SparseIndices) > 0 && len(req.SparseValues) > 0 {
		sparsePrefetch := &qdrant.PrefetchQuery{
			Query: qdrant.NewQuerySparse(req.SparseIndices, req.SparseValues),
			Using: qdrant.PtrOf("sparse"),
			Limit: qdrant.PtrOf(prefetchLimit),
		}
		if req.Filter != nil {
			sparsePrefetch.Filter = buildSearchFilter(req.Filter)
		}
		prefetch = append(prefetch, sparsePrefetch)
	}

	// Dense prefetch
	if len(req.DenseVector) > 0 {
		densePrefetch := &qdrant.PrefetchQuery{
			Query: qdrant.NewQueryDense(req.DenseVector),
			Using: qdrant.PtrOf("dense"),
			Limit: qdrant.PtrOf(prefetchLimit),
		}
		if req.Filter != nil {
			densePrefetch.Filter = buildSearchFilter(req.Filter)
		}
		prefetch = append(prefetch, densePrefetch)
	}

	if len(prefetch) == 0 {
		return nil, fmt.Errorf("at least one of sparse or dense vector must be provided")
	}

	limit := req.Limit
	if limit == 0 {
		limit = 20
	}

	// Build the query with RRF fusion
	queryPoints := &qdrant.QueryPoints{
		CollectionName: collectionName(collection),
		Prefetch:       prefetch,
		Query:          qdrant.NewQueryFusion(qdrant.Fusion_RRF),
		Limit:          qdrant.PtrOf(limit),
		WithPayload:    qdrant.NewWithPayload(req.WithPayload),
	}

	if req.ScoreThreshold != nil {
		queryPoints.ScoreThreshold = req.ScoreThreshold
	}

	results, err := c.client.Query(ctx, queryPoints)
	if err != nil {
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	return scoredPointsToResults(results)
}

// DenseSearch performs a dense-only vector search.
func (c *Client) DenseSearch(ctx context.Context, collection string, req SearchRequest) ([]SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	if len(req.DenseVector) == 0 {
		return nil, fmt.Errorf("dense vector is required")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	limit := req.Limit
	if limit == 0 {
		limit = 20
	}

	queryPoints := &qdrant.QueryPoints{
		CollectionName: collectionName(collection),
		Query:          qdrant.NewQueryDense(req.DenseVector),
		Using:          qdrant.PtrOf("dense"),
		Limit:          qdrant.PtrOf(limit),
		WithPayload:    qdrant.NewWithPayload(req.WithPayload),
	}

	if req.Filter != nil {
		queryPoints.Filter = buildSearchFilter(req.Filter)
	}

	if req.ScoreThreshold != nil {
		queryPoints.ScoreThreshold = req.ScoreThreshold
	}

	results, err := c.client.Query(ctx, queryPoints)
	if err != nil {
		return nil, fmt.Errorf("dense search failed: %w", err)
	}

	return scoredPointsToResults(results)
}

// SparseSearch performs a sparse-only vector search.
func (c *Client) SparseSearch(ctx context.Context, collection string, req SearchRequest) ([]SearchResult, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, fmt.Errorf("client is closed")
	}

	if len(req.SparseIndices) == 0 || len(req.SparseValues) == 0 {
		return nil, fmt.Errorf("sparse indices and values are required")
	}

	ctx, cancel := context.WithTimeout(ctx, c.config.Timeout)
	defer cancel()

	limit := req.Limit
	if limit == 0 {
		limit = 20
	}

	queryPoints := &qdrant.QueryPoints{
		CollectionName: collectionName(collection),
		Query:          qdrant.NewQuerySparse(req.SparseIndices, req.SparseValues),
		Using:          qdrant.PtrOf("sparse"),
		Limit:          qdrant.PtrOf(limit),
		WithPayload:    qdrant.NewWithPayload(req.WithPayload),
	}

	if req.Filter != nil {
		queryPoints.Filter = buildSearchFilter(req.Filter)
	}

	if req.ScoreThreshold != nil {
		queryPoints.ScoreThreshold = req.ScoreThreshold
	}

	results, err := c.client.Query(ctx, queryPoints)
	if err != nil {
		return nil, fmt.Errorf("sparse search failed: %w", err)
	}

	return scoredPointsToResults(results)
}

// buildSearchFilter builds a Qdrant filter from SearchFilter.
func buildSearchFilter(f *SearchFilter) *qdrant.Filter {
	if f == nil {
		return nil
	}

	var conditions []*qdrant.Condition

	if f.PathPrefix != "" {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "path",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Text{
							Text: f.PathPrefix,
						},
					},
				},
			},
		})
	}

	if len(f.Languages) > 0 {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "language",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keywords{
							Keywords: &qdrant.RepeatedStrings{
								Strings: f.Languages,
							},
						},
					},
				},
			},
		})
	}

	if f.DocumentHash != "" {
		conditions = append(conditions, &qdrant.Condition{
			ConditionOneOf: &qdrant.Condition_Field{
				Field: &qdrant.FieldCondition{
					Key: "document_hash",
					Match: &qdrant.Match{
						MatchValue: &qdrant.Match_Keyword{
							Keyword: f.DocumentHash,
						},
					},
				},
			},
		})
	}

	if len(conditions) == 0 {
		return nil
	}

	return &qdrant.Filter{
		Must: conditions,
	}
}

// scoredPointsToResults converts Qdrant scored points to SearchResults.
func scoredPointsToResults(points []*qdrant.ScoredPoint) ([]SearchResult, error) {
	results := make([]SearchResult, 0, len(points))

	for _, p := range points {
		result, err := scoredPointToResult(p)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	return results, nil
}

// scoredPointToResult converts a single scored point to SearchResult.
func scoredPointToResult(p *qdrant.ScoredPoint) (SearchResult, error) {
	var id string
	switch v := p.Id.PointIdOptions.(type) {
	case *qdrant.PointId_Uuid:
		id = v.Uuid
	case *qdrant.PointId_Num:
		id = fmt.Sprintf("%d", v.Num)
	}

	payload := extractPayload(p.Payload)

	return SearchResult{
		ID:      id,
		Score:   p.Score,
		Payload: payload,
	}, nil
}

// extractPayload extracts PointPayload from Qdrant payload map.
func extractPayload(payload map[string]*qdrant.Value) PointPayload {
	result := PointPayload{}

	if v := getStringValue(payload, "store"); v != "" {
		result.Store = v
	}
	if v := getStringValue(payload, "path"); v != "" {
		result.Path = v
	}
	if v := getStringValue(payload, "language"); v != "" {
		result.Language = v
	}
	if v := getStringValue(payload, "content"); v != "" {
		result.Content = v
	}
	if v := getStringSliceValue(payload, "symbols"); len(v) > 0 {
		result.Symbols = v
	}
	if v := getIntValue(payload, "start_line"); v != 0 {
		result.StartLine = v
	}
	if v := getIntValue(payload, "end_line"); v != 0 {
		result.EndLine = v
	}
	if v := getStringValue(payload, "document_hash"); v != "" {
		result.DocumentHash = v
	}
	if v := getStringValue(payload, "chunk_hash"); v != "" {
		result.ChunkHash = v
	}
	if v := getStringValue(payload, "indexed_at"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			result.IndexedAt = t
		}
	}

	return result
}

// Helper functions to extract values from Qdrant payload

func getStringValue(payload map[string]*qdrant.Value, key string) string {
	if v, ok := payload[key]; ok {
		if sv, ok := v.Kind.(*qdrant.Value_StringValue); ok {
			return sv.StringValue
		}
	}
	return ""
}

func getIntValue(payload map[string]*qdrant.Value, key string) int {
	if v, ok := payload[key]; ok {
		if iv, ok := v.Kind.(*qdrant.Value_IntegerValue); ok {
			return int(iv.IntegerValue)
		}
	}
	return 0
}

func getStringSliceValue(payload map[string]*qdrant.Value, key string) []string {
	if v, ok := payload[key]; ok {
		if lv, ok := v.Kind.(*qdrant.Value_ListValue); ok {
			result := make([]string, 0, len(lv.ListValue.Values))
			for _, item := range lv.ListValue.Values {
				if sv, ok := item.Kind.(*qdrant.Value_StringValue); ok {
					result = append(result, sv.StringValue)
				}
			}
			return result
		}
	}
	return nil
}
