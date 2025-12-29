# Configurable RRF Fusion Weights

## Overview

Implemented configurable sparse/dense weights for RRF (Reciprocal Rank Fusion) in go-search, allowing users to control the balance between BM25 keyword search and semantic vector search.

## Implementation Summary

### New Components

1. **`internal/search/fusion/rrf.go`**: Manual RRF fusion implementation with configurable weights
   - `RRFConfig`: Configuration for fusion (K constant, sparse/dense weights)
   - `ScoredResult`: Result with component scores and ranks
   - `Fuse()`: Combines sparse and dense results using weighted RRF
   - `IsBalanced()`: Detects when native Qdrant RRF can be used

2. **Enhanced `search.Result`**: Added fields for manual fusion metadata
   - `SparseRank`: Rank in sparse-only results
   - `DenseRank`: Rank in dense-only results  
   - `SparseScore`: Original sparse score
   - `DenseScore`: Original dense score
   - `FusedScore`: Combined RRF score

3. **Updated `search.Service.Search()`**: Intelligent fusion strategy
   - Uses Qdrant native RRF when weights are balanced (0.5/0.5 ±0.05)
   - Executes separate sparse/dense searches + manual fusion when weights differ
   - Respects request-level weight overrides

### Configuration

#### Environment Variables (already existed, now functional)
```bash
RICE_DEFAULT_SPARSE_WEIGHT=0.5  # 0.0-1.0, default 0.5
RICE_DEFAULT_DENSE_WEIGHT=0.5   # 0.0-1.0, default 0.5
```

#### API Request Override
```json
{
  "query": "search text",
  "sparse_weight": 0.7,  // Override global default
  "dense_weight": 0.3
}
```

### RRF Formula

```
score = (sparseWeight / (k + sparseRank)) + (denseWeight / (k + denseRank))
```

Where:
- `k = 60` (RRF smoothing constant)
- `sparseRank, denseRank`: 1-based ranks in respective result lists
- Weights sum to 1.0 for balanced fusion

### Fusion Strategy

| Weights | Strategy | Performance |
|---------|----------|-------------|
| 0.5/0.5 (±0.05) | Qdrant native RRF | ✅ Fast (single query) |
| Other | Manual RRF | Slower (2 queries + fusion) |

**Why two modes?**
- Qdrant's native RRF uses equal weights (0.5/0.5) - very fast
- Custom weights require separate queries - adds ~2x latency
- Automatic fallback ensures best performance for common case

### Response Format

#### Equal Weights (Qdrant Native)
```json
{
  "results": [
    {
      "id": "doc1",
      "score": 0.85,  // Qdrant RRF score
      // No sparse_rank, dense_rank, etc. (not available)
    }
  ]
}
```

#### Custom Weights (Manual Fusion)
```json
{
  "results": [
    {
      "id": "doc1",
      "score": 0.01634,     // = fused_score (primary)
      "sparse_rank": 1,
      "dense_rank": 3,
      "sparse_score": 10.5,
      "dense_score": 0.75,
      "fused_score": 0.01634  // Explicit RRF score
    }
  ]
}
```

## Testing

### Unit Tests (`internal/search/fusion/rrf_test.go`)
- Equal weights fusion
- Sparse-heavy weights (0.9/0.1)
- Dense-heavy weights (0.1/0.9)
- Empty results handling
- Single retriever scenarios
- `IsBalanced()` detection

### Integration Tests (`internal/search/fusion_integration_test.go`)
- Verifies different weights produce different rankings
- Tests balanced detection logic
- Validates score calculations

**Test Coverage**: 7 unit tests + 2 integration tests, all passing

## Examples

### Default Balanced Search
```bash
# Uses Qdrant native RRF (fast)
curl -X POST localhost:8080/v1/stores/default/search \
  -H "Content-Type: application/json" \
  -d '{"query": "authentication handler"}'
```

### Keyword-Heavy Search
```bash
# Uses manual RRF (slower, more control)
curl -X POST localhost:8080/v1/stores/default/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "exact_function_name",
    "sparse_weight": 0.9,
    "dense_weight": 0.1
  }'
```

### Semantic-Heavy Search
```bash
# Uses manual RRF
curl -X POST localhost:8080/v1/stores/default/search \
  -H "Content-Type: application/json" \
  -d '{
    "query": "how does authentication work",
    "sparse_weight": 0.2,
    "dense_weight": 0.8
  }'
```

## Performance Impact

| Scenario | Query Count | Relative Latency |
|----------|-------------|------------------|
| Equal weights (0.5/0.5) | 1 hybrid | 1.0x (baseline) |
| Custom weights | 2 separate | ~1.8-2.2x |

**Optimization**: Balanced weights automatically use fast path (no performance penalty)

## Migration Notes

- **Backward compatible**: Default behavior unchanged (0.5/0.5)
- **No breaking changes**: Existing API requests work identically
- **Gradual adoption**: Users can experiment with weights per-request

## Files Changed

```
go-search/
├── internal/search/
│   ├── fusion/
│   │   ├── rrf.go                      # NEW: Manual RRF fusion
│   │   └── rrf_test.go                 # NEW: Unit tests
│   ├── fusion_integration_test.go      # NEW: Integration tests
│   └── service.go                      # MODIFIED: Conditional fusion
└── docs/
    └── FUSION_WEIGHTS.md               # NEW: This file
```

## Future Improvements

- [ ] Metrics for fusion strategy usage (native vs manual)
- [ ] Auto-tuning weights based on query type (navigational vs exploratory)
- [ ] Per-store default weights
- [ ] CLI flag for `rice-search search --sparse-weight 0.7`

## References

- [RRF Paper](https://plg.uwaterloo.ca/~gvcormac/cormacksigir09-rrf.pdf)
- Qdrant Hybrid Search: https://qdrant.tech/documentation/concepts/hybrid-queries/
- go-search/docs/04-search.md (RRF implementation details)
