# Multi-Pass Reranker

Two-pass neural reranking with early exit optimization for go-search.

## Overview

The multi-pass reranker implements an intelligent two-stage reranking system that balances search quality with latency:

1. **Pass 1 (Gate)**: Fast rerank of top 30 candidates (default: 80ms timeout)
2. **Pass 2 (Precision)**: Deeper rerank of top 100 candidates if needed (default: 150ms timeout)
3. **Early Exit**: Skip pass 2 when high-confidence signal detected

## Architecture

```
Query + Results
    │
    ▼
┌───────────────────────────────────┐
│  Pass 1: Fast Gate (30 candidates)│
│  - Timeout: 80ms                  │
│  - Neural reranking (ONNX)        │
└──────────────┬────────────────────┘
               │
               ▼
      ┌────────────────┐
      │  Early Exit?   │
      │ - Peaked dist  │  NO
      │ - High gap     │ ────┐
      │ - Flat dist    │     │
      └────────┬───────┘     │
               │YES          │
               │             ▼
               │    ┌─────────────────────────────┐
               │    │ Pass 2: Precision Rerank    │
               │    │ (100 candidates)            │
               │    │  - Timeout: 150ms           │
               │    │  - Full neural rerank       │
               │    └──────────┬──────────────────┘
               │               │
               ▼               ▼
          Final Results
```

## Distribution Analysis

The reranker analyzes score distributions to make intelligent early exit decisions:

### Distribution Shapes

1. **Peaked** (early exit): One clear winner, high score ratio (> 1.5)
   - Example: [0.95, 0.50, 0.45, 0.40]
   - Signal: Strong confidence in top result

2. **Flat** (no early exit): Similar scores, uncertainty
   - Example: [0.70, 0.69, 0.68, 0.67]
   - Signal: Needs deeper analysis (pass 2)

3. **Bimodal**: Mixed distribution
   - Example: [0.90, 0.85, 0.50, 0.45]
   - Signal: Conditional exit based on gap

### Early Exit Conditions

- **Peaked Distribution**: `scoreRatio > 0.85` (default threshold)
- **High Score Gap**: `scoreGap > 0.3` (default threshold)
- **Insufficient Results**: `len(results) < 2`

## Configuration

Default values (can be customized):

```go
cfg := reranker.Config{
    Pass1Candidates: 30,      // Candidates for fast pass
    Pass2Candidates: 100,     // Candidates for deep pass
    Pass1Timeout:    80,      // Milliseconds
    Pass2Timeout:    150,     // Milliseconds
    EarlyExitThresh: 0.85,    // Score ratio threshold
    EarlyExitGap:    0.3,     // Score gap threshold
}
reranker.SetConfig(cfg)
```

Environment variables:

```bash
RICE_ENABLE_MULTI_PASS=true       # Enable multi-pass (default: true)
RICE_PASS1_CANDIDATES=30          # Pass 1 candidates
RICE_PASS2_CANDIDATES=100         # Pass 2 candidates
RICE_PASS1_TIMEOUT=80            # Pass 1 timeout (ms)
RICE_PASS2_TIMEOUT=150           # Pass 2 timeout (ms)
RICE_EARLY_EXIT_THRESHOLD=0.85   # Early exit threshold
RICE_EARLY_EXIT_GAP=0.3          # Early exit gap
```

## Usage

```go
import (
    "github.com/ricesearch/rice-search/internal/ml"
    "github.com/ricesearch/rice-search/internal/search"
    "github.com/ricesearch/rice-search/internal/search/reranker"
)

// Create reranker
log := logger.New("info", "text")
mlService := // ... initialize ML service
multiPass := reranker.NewMultiPassReranker(mlService, log)

// Optional: customize config
cfg := reranker.Config{
    Pass1Candidates: 50,
    Pass2Candidates: 150,
}
multiPass.SetConfig(cfg)

// Rerank results
result, err := multiPass.Rerank(ctx, query, searchResults)
if err != nil {
    // Handle error
}

// Check metadata
fmt.Printf("Pass 1: %v (% dms)\n", result.Pass1Applied, result.Pass1LatencyMs)
fmt.Printf("Pass 2: %v (%dms)\n", result.Pass2Applied, result.Pass2LatencyMs)
fmt.Printf("Early exit: %v (%s)\n", result.EarlyExit, result.EarlyExitReason)
```

## Response Metadata

```go
type MultiPassResult struct {
    Results         []search.Result
    Pass1Applied    bool
    Pass1LatencyMs  int64
    Pass2Applied    bool
    Pass2LatencyMs  int64
    EarlyExit       bool
    EarlyExitReason string  // "insufficient_results", "peaked_distribution", "high_score_gap"
}
```

## Performance Characteristics

| Scenario | Pass 1 | Pass 2 | Total Latency | Exit Reason |
|----------|--------|--------|---------------|-------------|
| Clear winner | ✓ | ✗ | ~80ms | peaked_distribution |
| High gap | ✓ | ✗ | ~80ms | high_score_gap |
| Ambiguous | ✓ | ✓ | ~230ms | none |
| Few results | ✓ | ✗ | ~80ms | insufficient_results |

## Testing

Run tests:

```bash
cd go-search
go test -v ./internal/search/reranker/...
```

Test coverage includes:
- Early exit conditions (peaked, flat, insufficient)
- Distribution analysis (peaked, flat, bimodal)
- Config updates
- Timeout handling
- Score ordering

## Integration

To integrate into search service:

1. Import the reranker package
2. Create `MultiPassReranker` instance during search service initialization
3. Replace single-pass reranking call with multi-pass version
4. Update response metadata to include `RerankingMetadata`

Example search service integration:

```go
// In search service
if enableReranking && len(results) > 0 {
    mpResult, err := s.multiPassReranker.Rerank(ctx, query, results)
    if err != nil {
        s.log.Warn("Multi-pass reranking failed", "error", err)
    } else {
        results = mpResult.Results
        metadata.Reranking = &RerankingMetadata{
            Pass1Applied:    mpResult.Pass1Applied,
            Pass1LatencyMs:  mpResult.Pass1LatencyMs,
            Pass2Applied:    mpResult.Pass2Applied,
            Pass2LatencyMs:  mpResult.Pass2LatencyMs,
            EarlyExit:       mpResult.EarlyExit,
            EarlyExitReason: mpResult.EarlyExitReason,
        }
    }
}
```

## Design Decisions

1. **Two-pass approach**: Balances quality (pass 2) with latency (early exit after pass 1)
2. **Distribution analysis**: Statistical approach to determine when pass 2 is needed
3. **Timeout per pass**: Independent timeouts allow fine-grained control
4. **Fallback on timeout**: Returns pass 1 results if pass 2 times out
5. **Configurable thresholds**: Allows tuning for different use cases

## References

- NestJS implementation: `api/src/ranking/multi-pass-reranker.service.ts`
- Early exit signals: Distribution shape, score ratio, score gap
- AGENTS.md: Multi-pass reranking requirements
